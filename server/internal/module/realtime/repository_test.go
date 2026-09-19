package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database/testquery"
	"admin/server/internal/database/testschema"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestRepositoryAppendTxCommitsRollsBackAndDeduplicates(t *testing.T) {
	db, ctx := openRealtimeRepositoryDB(t)
	repository := NewRepository(db)
	input := realtimeUserEventInput("dedup-1", 1, 10, time.Now().UTC())
	var first Event
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { var err error; first, err = repository.AppendTx(ctx, tx, input); return err }); err != nil {
		t.Fatal(err)
	}
	if first.Sequence <= 0 || first.EventID != input.EventID {
		t.Fatalf("event=%+v", first)
	}
	assertRealtimeRows(t, db, ctx, 1, 1)

	rollbackInput := realtimeUserEventInput("dedup-rollback", 1, 10, time.Now().UTC())
	wantRollback := errors.New("rollback")
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := repository.AppendTx(ctx, tx, rollbackInput); err != nil {
			return err
		}
		return wantRollback
	})
	if !errors.Is(err, wantRollback) {
		t.Fatalf("rollback err=%v", err)
	}
	assertRealtimeRows(t, db, ctx, 1, 1)

	retry := input
	retry.EventID = uuid.NewString()
	var duplicate Event
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		duplicate, err = repository.AppendTx(ctx, tx, retry)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if duplicate.Sequence != first.Sequence || duplicate.EventID != first.EventID {
		t.Fatalf("duplicate=%+v first=%+v", duplicate, first)
	}
	assertRealtimeRows(t, db, ctx, 1, 1)
}

func TestRepositoryResumeWindowIsolatesTargetsAndUsesTwoQueries(t *testing.T) {
	db, ctx := openRealtimeRepositoryDB(t)
	repository := NewRepository(db)
	now := time.Now().UTC()
	appendRealtimeEvent(t, repository, db, ctx, realtimeUserEventInput("p1-u10", 1, 10, now))
	appendRealtimeEvent(t, repository, db, ctx, realtimeUserEventInput("p2-u10", 2, 10, now))
	appendRealtimeEvent(t, repository, db, ctx, realtimeUserEventInput("p1-u20", 1, 20, now))
	maxUser := int64(15)
	appendRealtimeEvent(t, repository, db, ctx, EventInput{EventID: uuid.NewString(), DedupKey: "p1-platform", PlatformID: 1, EventType: EventNotificationCreated, TargetType: TargetPlatform, AudienceMaxUserID: &maxUser, Payload: json.RawMessage(`{}`), OccurredAt: now})

	probe := testquery.New(db.Logger, "realtime_event", "realtime_retention_state")
	measured := db.Session(&gorm.Session{Logger: probe})
	window, err := NewRepository(measured).ResumeWindow(ctx, 1, 10, 0, 500)
	if err != nil {
		t.Fatal(err)
	}
	if probe.Total() != 1 {
		t.Fatalf("resume queries=%d want=1 window=%+v", probe.Total(), window)
	}
	if window.ResyncRequired || window.ThroughSequence != 4 || len(window.Events) != 2 {
		t.Fatalf("window=%+v", window)
	}
	if window.Events[0].DedupKey != "p1-u10" || window.Events[1].DedupKey != "p1-platform" {
		t.Fatalf("events=%+v", window.Events)
	}

	other, err := repository.ResumeWindow(ctx, 2, 10, 0, 500)
	if err != nil || len(other.Events) != 1 || other.Events[0].DedupKey != "p2-u10" {
		t.Fatalf("other=%+v err=%v", other, err)
	}
}

func TestRepositoryResumeWindowRequiresResyncForWatermarkAndOverLimit(t *testing.T) {
	db, ctx := openRealtimeRepositoryDB(t)
	repository := NewRepository(db)
	now := time.Now().UTC()
	if err := db.WithContext(ctx).Exec(`INSERT INTO realtime_event(event_id,dedup_key,platform_id,event_type,target_type,target_user_id,payload,occurred_at,created_at,updated_at)
		SELECT gen_random_uuid(),'bulk-'||value,1,'notification.created.v1','user',10,'{}'::jsonb,?,?,? FROM generate_series(1,501) AS value`, now, now, now).Error; err != nil {
		t.Fatal(err)
	}
	window, err := repository.ResumeWindow(ctx, 1, 10, 0, 500)
	if err != nil {
		t.Fatal(err)
	}
	if !window.ResyncRequired || !window.OverLimit || len(window.Events) != 0 {
		t.Fatalf("over-limit window=%+v", window)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO realtime_retention_state(platform_id,deleted_through_sequence,created_at,updated_at) VALUES(1,200,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error; err != nil {
		t.Fatal(err)
	}
	watermark, err := repository.ResumeWindow(ctx, 1, 10, 199, 500)
	if err != nil {
		t.Fatal(err)
	}
	if !watermark.ResyncRequired || watermark.DeletedThroughSequence != 200 || len(watermark.Events) != 0 {
		t.Fatalf("watermark=%+v", watermark)
	}
}

func TestRepositoryOutboxClaimsDoNotOverlapAndExpiredLeaseIsTakenOver(t *testing.T) {
	db, ctx := openRealtimeRepositoryDB(t)
	r1, r2 := NewRepository(db), NewRepository(db.Session(&gorm.Session{}))
	now := time.Now().UTC()
	appendRealtimeEvent(t, r1, db, ctx, realtimeUserEventInput("claim-1", 1, 10, now))
	appendRealtimeEvent(t, r1, db, ctx, realtimeUserEventInput("claim-2", 1, 10, now))
	now = time.Now().UTC()
	first, err := r1.ClaimPending(ctx, 1, "token-a", now, time.Minute)
	if err != nil || len(first) != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := r2.ClaimPending(ctx, 1, "token-b", now, time.Minute)
	if err != nil || len(second) != 1 || second[0].ID == first[0].ID {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	if err := r1.MarkPublished(ctx, first[0].ID, "wrong", now); !errors.Is(err, ErrClaimLost) {
		t.Fatalf("old mark err=%v", err)
	}
	if err := r1.MarkPublished(ctx, first[0].ID, "token-a", now); err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`UPDATE realtime_event_outbox SET locked_until=? WHERE id=?`, now.Add(-time.Second), second[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	taken, err := r1.ClaimPending(ctx, 1, "token-c", now, time.Minute)
	if err != nil || len(taken) != 1 || taken[0].ID != second[0].ID {
		t.Fatalf("taken=%+v err=%v", taken, err)
	}
	if err := r2.Reschedule(ctx, second[0].ID, "token-b", "old", now.Add(time.Minute), now); !errors.Is(err, ErrClaimLost) {
		t.Fatalf("old reschedule err=%v", err)
	}
	if err := r1.Reschedule(ctx, second[0].ID, "token-c", " temporary failure ", now.Add(time.Minute), now); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryCleanupExpiredKeepsPendingAndAdvancesWatermark(t *testing.T) {
	db, ctx := openRealtimeRepositoryDB(t)
	repository := NewRepository(db)
	now := time.Now().UTC()
	oldPublished := appendRealtimeEvent(t, repository, db, ctx, realtimeUserEventInput("old-published", 1, 10, now.Add(-48*time.Hour)))
	oldPending := appendRealtimeEvent(t, repository, db, ctx, realtimeUserEventInput("old-pending", 1, 10, now.Add(-48*time.Hour)))
	appendRealtimeEvent(t, repository, db, ctx, realtimeUserEventInput("recent", 1, 10, now))
	now = time.Now().UTC()
	claimed, err := repository.ClaimPending(ctx, 1, "publish", now, time.Minute)
	if err != nil || len(claimed) != 1 || claimed[0].Event.Sequence != oldPublished.Sequence {
		t.Fatalf("claimed=%+v err=%v", claimed, err)
	}
	if err := repository.MarkPublished(ctx, claimed[0].ID, "publish", now); err != nil {
		t.Fatal(err)
	}
	deleted, err := repository.CleanupExpired(ctx, now.Add(-24*time.Hour), 100, now)
	if err != nil || deleted != 1 {
		t.Fatalf("deleted=%d err=%v", deleted, err)
	}
	var sequences []int64
	if err := db.WithContext(ctx).Raw(`SELECT sequence FROM realtime_event ORDER BY sequence`).Scan(&sequences).Error; err != nil {
		t.Fatal(err)
	}
	if len(sequences) != 2 || sequences[0] != oldPending.Sequence {
		t.Fatalf("remaining=%v", sequences)
	}
	var watermark int64
	if err := db.WithContext(ctx).Raw(`SELECT deleted_through_sequence FROM realtime_retention_state WHERE platform_id=1`).Scan(&watermark).Error; err != nil {
		t.Fatal(err)
	}
	if watermark != oldPublished.Sequence {
		t.Fatalf("watermark=%d", watermark)
	}
}

func openRealtimeRepositoryDB(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	db, ctx := testschema.Open(t, realtimeRepositoryDSN(t), "test_realtime_repository")
	for _, statement := range []string{
		`CREATE EXTENSION IF NOT EXISTS pgcrypto`,
		`CREATE TABLE permission_auth_platform(id BIGINT PRIMARY KEY)`,
		`CREATE TABLE user_account(id BIGINT PRIMARY KEY)`,
		`INSERT INTO permission_auth_platform(id) VALUES(1),(2)`,
		`INSERT INTO user_account(id) VALUES(10),(20)`,
		`CREATE TABLE realtime_event(sequence BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,event_id UUID NOT NULL UNIQUE,dedup_key VARCHAR(256) NOT NULL UNIQUE,platform_id BIGINT NOT NULL REFERENCES permission_auth_platform(id),event_type VARCHAR(64) NOT NULL,target_type VARCHAR(16) NOT NULL,target_user_id BIGINT NULL REFERENCES user_account(id),audience_max_user_id BIGINT NULL,payload JSONB NOT NULL,occurred_at TIMESTAMPTZ NOT NULL,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE realtime_event_outbox(id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,event_sequence BIGINT NOT NULL UNIQUE REFERENCES realtime_event(sequence) ON DELETE CASCADE,attempts INTEGER NOT NULL DEFAULT 0,available_at TIMESTAMPTZ NOT NULL,locked_until TIMESTAMPTZ NULL,lock_token VARCHAR(64) NULL,published_at TIMESTAMPTZ NULL,last_error VARCHAR(512) NULL,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE realtime_retention_state(platform_id BIGINT PRIMARY KEY REFERENCES permission_auth_platform(id),deleted_through_sequence BIGINT NOT NULL DEFAULT 0,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL)`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db, ctx
}

func realtimeRepositoryDSN(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", ".."))
	if err := godotenv.Load(filepath.Join(repoRoot, "server", ".env")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	return settings.PostgresDSN
}

func realtimeUserEventInput(dedup string, platformID, userID int64, occurredAt time.Time) EventInput {
	return EventInput{EventID: uuid.NewString(), DedupKey: dedup, PlatformID: platformID, EventType: EventNotificationCreated, TargetType: TargetUser, TargetUserID: &userID, Payload: json.RawMessage(`{}`), OccurredAt: occurredAt}
}

func appendRealtimeEvent(t *testing.T, repository *Repository, db *gorm.DB, ctx context.Context, input EventInput) Event {
	t.Helper()
	var event Event
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { var err error; event, err = repository.AppendTx(ctx, tx, input); return err }); err != nil {
		t.Fatal(err)
	}
	return event
}

func assertRealtimeRows(t *testing.T, db *gorm.DB, ctx context.Context, wantEvents, wantOutbox int64) {
	t.Helper()
	var events, outbox int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM realtime_event`).Scan(&events).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM realtime_event_outbox`).Scan(&outbox).Error; err != nil {
		t.Fatal(err)
	}
	if events != wantEvents || outbox != wantOutbox {
		t.Fatalf("events=%d outbox=%d want=%d/%d", events, outbox, wantEvents, wantOutbox)
	}
}
