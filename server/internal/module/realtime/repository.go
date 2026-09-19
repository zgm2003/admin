package realtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrClaimLost = errors.New("realtime outbox claim lost")

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) AppendTx(ctx context.Context, tx *gorm.DB, input EventInput) (Event, error) {
	if tx == nil {
		return Event{}, errors.New("transaction is required")
	}
	if err := ValidateEventInput(input); err != nil {
		return Event{}, err
	}
	var event Event
	query := `
WITH inserted AS (
 INSERT INTO realtime_event(event_id,dedup_key,platform_id,event_type,target_type,target_user_id,audience_max_user_id,payload,occurred_at,created_at,updated_at)
 VALUES (?::uuid,?,?,?,?,?,?,?::jsonb,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
 ON CONFLICT (dedup_key) DO NOTHING
 RETURNING sequence,event_id::text,dedup_key,platform_id,event_type,target_type,target_user_id,audience_max_user_id,payload,occurred_at,created_at,updated_at
)
SELECT * FROM inserted
UNION ALL
SELECT sequence,event_id::text,dedup_key,platform_id,event_type,target_type,target_user_id,audience_max_user_id,payload,occurred_at,created_at,updated_at
FROM realtime_event WHERE dedup_key=? AND NOT EXISTS (SELECT 1 FROM inserted)
LIMIT 1`
	if err := tx.WithContext(ctx).Raw(query,
		input.EventID, input.DedupKey, input.PlatformID, input.EventType, input.TargetType,
		input.TargetUserID, input.AudienceMaxUserID, []byte(input.Payload), input.OccurredAt, input.DedupKey,
	).Scan(&event).Error; err != nil {
		return Event{}, err
	}
	if event.Sequence <= 0 {
		return Event{}, errors.New("append realtime event returned no row")
	}
	if err := tx.WithContext(ctx).Exec(`
INSERT INTO realtime_event_outbox(event_sequence,attempts,available_at,created_at,updated_at)
VALUES(?,0,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
ON CONFLICT (event_sequence) DO NOTHING`, event.Sequence).Error; err != nil {
		return Event{}, err
	}
	return event, nil
}

func (r *Repository) ResumeWindow(ctx context.Context, platformID, userID, after int64, limit int) (ResumeWindow, error) {
	if platformID <= 0 || userID <= 0 || after < 0 || limit < 1 || limit > 500 {
		return ResumeWindow{}, errors.New("invalid resume query")
	}
	type resumeRow struct {
		ThroughSequence        int64      `gorm:"column:through_sequence"`
		DeletedThroughSequence int64      `gorm:"column:deleted_through_sequence"`
		Sequence               *int64     `gorm:"column:sequence"`
		EventID                string     `gorm:"column:event_id"`
		DedupKey               string     `gorm:"column:dedup_key"`
		PlatformID             int64      `gorm:"column:platform_id"`
		EventType              string     `gorm:"column:event_type"`
		TargetType             TargetType `gorm:"column:target_type"`
		TargetUserID           *int64     `gorm:"column:target_user_id"`
		AudienceMaxUserID      *int64     `gorm:"column:audience_max_user_id"`
		Payload                []byte     `gorm:"column:payload"`
		OccurredAt             *time.Time `gorm:"column:occurred_at"`
		CreatedAt              *time.Time `gorm:"column:created_at"`
		UpdatedAt              *time.Time `gorm:"column:updated_at"`
	}
	var rows []resumeRow
	if err := r.db.WithContext(ctx).Raw(`
WITH bounds AS MATERIALIZED (
 SELECT
  COALESCE((SELECT max(sequence) FROM realtime_event WHERE platform_id=?),0) AS through_sequence,
  COALESCE((SELECT deleted_through_sequence FROM realtime_retention_state WHERE platform_id=?),0) AS deleted_through_sequence
), eligible AS MATERIALIZED (
 SELECT event.sequence,event.event_id::text AS event_id,event.dedup_key,event.platform_id,event.event_type,event.target_type,
        event.target_user_id,event.audience_max_user_id,event.payload,event.occurred_at,event.created_at,event.updated_at
 FROM realtime_event event CROSS JOIN bounds
 WHERE ?>=bounds.deleted_through_sequence AND ?<bounds.through_sequence
   AND event.platform_id=? AND event.sequence>? AND event.sequence<=bounds.through_sequence
   AND ((event.target_type='user' AND event.target_user_id=?) OR (event.target_type='platform' AND event.audience_max_user_id>=?))
 ORDER BY event.sequence
 LIMIT ?
)
SELECT bounds.through_sequence,bounds.deleted_through_sequence,
       eligible.sequence,eligible.event_id,eligible.dedup_key,eligible.platform_id,eligible.event_type,eligible.target_type,
       eligible.target_user_id,eligible.audience_max_user_id,eligible.payload,eligible.occurred_at,eligible.created_at,eligible.updated_at
FROM bounds LEFT JOIN eligible ON TRUE
ORDER BY eligible.sequence`, platformID, platformID, after, after, platformID, after, userID, userID, limit+1).Scan(&rows).Error; err != nil {
		return ResumeWindow{}, err
	}
	if len(rows) == 0 {
		return ResumeWindow{}, errors.New("resume query returned no bounds")
	}
	window := ResumeWindow{ThroughSequence: rows[0].ThroughSequence, DeletedThroughSequence: rows[0].DeletedThroughSequence}
	if after < window.DeletedThroughSequence {
		window.ResyncRequired = true
		return window, nil
	}
	window.Events = make([]Event, 0, len(rows))
	for _, row := range rows {
		if row.Sequence == nil {
			continue
		}
		if row.OccurredAt == nil || row.CreatedAt == nil || row.UpdatedAt == nil {
			return ResumeWindow{}, errors.New("resume event timestamps are missing")
		}
		window.Events = append(window.Events, Event{
			Sequence: *row.Sequence, EventID: row.EventID, DedupKey: row.DedupKey, PlatformID: row.PlatformID,
			EventType: row.EventType, TargetType: row.TargetType, TargetUserID: row.TargetUserID,
			AudienceMaxUserID: row.AudienceMaxUserID, Payload: row.Payload, OccurredAt: *row.OccurredAt,
			CreatedAt: *row.CreatedAt, UpdatedAt: *row.UpdatedAt,
		})
	}
	if len(window.Events) > limit {
		window.ResyncRequired = true
		window.OverLimit = true
		window.Events = nil
		return window, nil
	}
	return window, nil
}

func (r *Repository) ClaimPending(ctx context.Context, limit int, token string, now time.Time, lease time.Duration) ([]OutboxEvent, error) {
	if limit < 1 || limit > 500 || token == "" || len(token) > 64 || lease <= 0 {
		return nil, errors.New("invalid outbox claim")
	}
	lockedUntil := now.Add(lease)
	type claimedRow struct {
		OutboxID          int64      `gorm:"column:outbox_id"`
		Attempts          int        `gorm:"column:attempts"`
		AvailableAt       time.Time  `gorm:"column:available_at"`
		LockedUntil       time.Time  `gorm:"column:locked_until"`
		LockToken         string     `gorm:"column:lock_token"`
		Sequence          int64      `gorm:"column:sequence"`
		EventID           string     `gorm:"column:event_id"`
		DedupKey          string     `gorm:"column:dedup_key"`
		PlatformID        int64      `gorm:"column:platform_id"`
		EventType         string     `gorm:"column:event_type"`
		TargetType        TargetType `gorm:"column:target_type"`
		TargetUserID      *int64     `gorm:"column:target_user_id"`
		AudienceMaxUserID *int64     `gorm:"column:audience_max_user_id"`
		Payload           []byte     `gorm:"column:payload"`
		OccurredAt        time.Time  `gorm:"column:occurred_at"`
		EventCreatedAt    time.Time  `gorm:"column:event_created_at"`
		EventUpdatedAt    time.Time  `gorm:"column:event_updated_at"`
	}
	var rows []claimedRow
	if err := r.db.WithContext(ctx).Raw(`
WITH candidates AS (
 SELECT id FROM realtime_event_outbox
 WHERE published_at IS NULL AND available_at<=? AND (locked_until IS NULL OR locked_until<=?)
 ORDER BY available_at,id LIMIT ? FOR UPDATE SKIP LOCKED
), claimed AS (
 UPDATE realtime_event_outbox outbox SET locked_until=?,lock_token=?,updated_at=?
 FROM candidates WHERE outbox.id=candidates.id
 RETURNING outbox.*
)
SELECT claimed.id AS outbox_id,claimed.attempts,claimed.available_at,claimed.locked_until,claimed.lock_token,
 event.sequence,event.event_id::text,event.dedup_key,event.platform_id,event.event_type,event.target_type,event.target_user_id,event.audience_max_user_id,event.payload,event.occurred_at,
 event.created_at AS event_created_at,event.updated_at AS event_updated_at
FROM claimed JOIN realtime_event event ON event.sequence=claimed.event_sequence
ORDER BY claimed.available_at,claimed.id`, now, now, limit, lockedUntil, token, now).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]OutboxEvent, 0, len(rows))
	for _, row := range rows {
		result = append(result, OutboxEvent{
			ID: row.OutboxID, Attempts: row.Attempts, AvailableAt: row.AvailableAt, LockedUntil: row.LockedUntil, LockToken: row.LockToken,
			Event: Event{Sequence: row.Sequence, EventID: row.EventID, DedupKey: row.DedupKey, PlatformID: row.PlatformID, EventType: row.EventType, TargetType: row.TargetType, TargetUserID: row.TargetUserID, AudienceMaxUserID: row.AudienceMaxUserID, Payload: row.Payload, OccurredAt: row.OccurredAt, CreatedAt: row.EventCreatedAt, UpdatedAt: row.EventUpdatedAt},
		})
	}
	return result, nil
}

func (r *Repository) MarkPublished(ctx context.Context, id int64, token string, now time.Time) error {
	if id <= 0 || token == "" {
		return errors.New("invalid publish claim")
	}
	result := r.db.WithContext(ctx).Exec(`UPDATE realtime_event_outbox SET published_at=?,locked_until=NULL,lock_token=NULL,last_error=NULL,updated_at=? WHERE id=? AND published_at IS NULL AND lock_token=?`, now, now, id, token)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrClaimLost
	}
	return nil
}

func (r *Repository) Reschedule(ctx context.Context, id int64, token, safeError string, availableAt, now time.Time) error {
	safeError = normalizedError(safeError)
	if id <= 0 || token == "" || safeError == "" || availableAt.Before(now) {
		return errors.New("invalid reschedule claim")
	}
	result := r.db.WithContext(ctx).Exec(`UPDATE realtime_event_outbox SET attempts=attempts+1,available_at=?,locked_until=NULL,lock_token=NULL,last_error=?,updated_at=? WHERE id=? AND published_at IS NULL AND lock_token=?`, availableAt, safeError, now, id, token)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrClaimLost
	}
	return nil
}

func (r *Repository) CleanupExpired(ctx context.Context, cutoff time.Time, limit int, now time.Time) (int, error) {
	if cutoff.IsZero() || limit < 1 || limit > 1000 || now.IsZero() {
		return 0, errors.New("invalid retention cleanup")
	}
	var deleted int
	query := `
WITH candidates AS MATERIALIZED (
 SELECT event.sequence,event.platform_id
 FROM realtime_event event JOIN realtime_event_outbox outbox ON outbox.event_sequence=event.sequence
 WHERE event.occurred_at<? AND outbox.published_at IS NOT NULL
 ORDER BY event.occurred_at,event.sequence LIMIT ? FOR UPDATE OF event SKIP LOCKED
), watermarks AS (
 INSERT INTO realtime_retention_state(platform_id,deleted_through_sequence,created_at,updated_at)
 SELECT platform_id,max(sequence),?,? FROM candidates GROUP BY platform_id
 ON CONFLICT(platform_id) DO UPDATE SET deleted_through_sequence=GREATEST(realtime_retention_state.deleted_through_sequence,EXCLUDED.deleted_through_sequence),updated_at=EXCLUDED.updated_at
 RETURNING platform_id
), deleted AS (
 DELETE FROM realtime_event event USING candidates WHERE event.sequence=candidates.sequence RETURNING event.sequence
)
SELECT count(*) FROM deleted`
	if err := r.db.WithContext(ctx).Raw(query, cutoff, limit, now, now).Scan(&deleted).Error; err != nil {
		return 0, fmt.Errorf("cleanup realtime events: %w", err)
	}
	return deleted, nil
}

func safeErrorClass(err error) string {
	if err == nil {
		return ""
	}
	return normalizedError(strings.TrimSpace(err.Error()))
}
