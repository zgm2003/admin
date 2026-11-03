package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"admin/server/internal/module/realtime"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Model struct {
	ID          int64
	RunID       int64
	Sequence    int64
	EventType   string
	Payload     json.RawMessage `gorm:"type:jsonb"`
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Model) TableName() string { return "ai_run_event" }

type Repository struct {
	db       *gorm.DB
	realtime *realtime.Repository
}

func NewRepository(db *gorm.DB, events *realtime.Repository) *Repository {
	return &Repository{db: db, realtime: events}
}
func (r *Repository) AppendTx(ctx context.Context, tx *gorm.DB, runID int64, kind string, payload json.RawMessage, now time.Time) (Model, error) {
	var row Model
	if r == nil || r.realtime == nil || tx == nil || runID < 1 || now.IsZero() || len(payload) > 60*1024 {
		return row, errors.New("AI event dependencies or payload invalid")
	}
	switch kind {
	case "queued", "started", "textDelta", "toolStarted", "toolFinished", "mediaUpdated", "finished", "billingUpdated":
	default:
		return row, errors.New("AI event kind invalid")
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(payload, &object) != nil || object == nil {
		return row, errors.New("AI event payload must be an object")
	}
	var owner struct{ UserID, PlatformID, NextEventSequence int64 }
	res := tx.WithContext(ctx).Raw(`UPDATE ai_run SET next_event_sequence=next_event_sequence+1,updated_at=? WHERE id=? RETURNING user_id,platform_id,next_event_sequence`, now, runID).Scan(&owner)
	if res.Error != nil {
		return row, res.Error
	}
	if res.RowsAffected != 1 {
		return row, gorm.ErrRecordNotFound
	}
	row = Model{RunID: runID, Sequence: owner.NextEventSequence, EventType: kind, Payload: payload, CreatedAt: now, UpdatedAt: now}
	if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
		return Model{}, err
	}
	data, err := json.Marshal(struct {
		RunID     string          `json:"runId"`
		Sequence  int64           `json:"sequence"`
		EventType string          `json:"eventType"`
		Payload   json.RawMessage `json:"payload"`
	}{fmt.Sprint(runID), row.Sequence, kind, payload})
	if err != nil {
		return Model{}, err
	}
	key := fmt.Sprintf("ai.run:%d:%d", runID, row.Sequence)
	_, err = r.realtime.AppendTx(ctx, tx, realtime.EventInput{EventID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(key)).String(), DedupKey: key, PlatformID: owner.PlatformID, EventType: realtime.EventAIRunChanged, TargetType: realtime.TargetUser, TargetUserID: &owner.UserID, Payload: data, OccurredAt: now})
	if err != nil {
		return Model{}, err
	}
	row.PublishedAt = &now
	if err = tx.WithContext(ctx).Model(&Model{}).Where("id=?", row.ID).Updates(map[string]any{"published_at": now, "updated_at": now}).Error; err != nil {
		return Model{}, err
	}
	return row, nil
}
func (r *Repository) List(ctx context.Context, runID, after int64, limit int) ([]Model, error) {
	rows := make([]Model, 0)
	if r == nil || r.db == nil || runID < 1 || after < 0 || limit < 1 || limit > 100 {
		return rows, errors.New("AI event query invalid")
	}
	err := r.db.WithContext(ctx).Where("run_id=? AND sequence>?", runID, after).Order("sequence").Limit(limit + 1).Find(&rows).Error
	return rows, err
}

// MediaUpdated records a durable run event in its own short transaction. It
// is used by the media worker after a transfer state change; callers never
// publish directly to realtime without the PostgreSQL fact.
func (r *Repository) MediaUpdated(ctx context.Context, runID, mediaFileID, storageFileID int64, now time.Time) error {
	if r == nil || r.db == nil || runID < 1 || mediaFileID < 1 || storageFileID < 0 || now.IsZero() {
		return errors.New("AI media event input invalid")
	}
	payload, err := json.Marshal(map[string]any{"mediaFileId": mediaFileID, "storageFileId": storageFileID})
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, err := r.AppendTx(ctx, tx, runID, "mediaUpdated", payload, now)
		return err
	})
}

func (r *Repository) Cleanup(ctx context.Context, before time.Time, limit int) (int64, error) {
	if r == nil || r.db == nil || before.IsZero() || limit < 1 || limit > 10000 {
		return 0, errors.New("AI event cleanup input invalid")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	res := r.db.WithContext(ctx).Exec(`WITH doomed AS (
		SELECT e.id FROM ai_run_event e JOIN ai_run r ON r.id=e.run_id
		WHERE e.created_at<? AND e.published_at IS NOT NULL
		AND r.execution_status IN (3,4,5) AND r.billing_status=2
		ORDER BY e.id LIMIT ?
	) DELETE FROM ai_run_event e USING doomed d WHERE e.id=d.id`, before, limit)
	return res.RowsAffected, res.Error
}
