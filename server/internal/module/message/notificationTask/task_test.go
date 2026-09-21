package notificationtask

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"admin/server/internal/database/testquery"
	"admin/server/internal/module/message/notification"
	"admin/server/internal/module/realtime"
	"gorm.io/gorm"
)

func TestProcessorPlatformAudienceIsConstantWrite(t *testing.T) {
	db, ctx := openTaskDB(t)
	service := NewService(NewRepository(db, discardBatchJobWriter{}))
	task, err := service.Create(ctx, 1, DraftInput{PlatformID: 1, Title: "broadcast", ContentHTML: "<p>content</p>", Variant: notification.VariantInfo, Priority: notification.PriorityNormal, LinkType: notification.LinkNone, AudienceType: AudiencePlatform})
	if err != nil {
		t.Fatal(err)
	}
	task, err = service.Submit(ctx, task.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	probe := testquery.New(db.Logger, "user_account", "permission_user_role", "message_notification_task_target")
	measured := db.Session(&gorm.Session{Logger: probe})
	processor := NewProcessor(measured, realtime.NewRepository(measured), discardBatchJobWriter{})
	payload := BatchPayload{SchemaVersion: 1, TaskID: task.ID, BatchNo: 0}
	if err = processor.Process(ctx, payload); err != nil {
		t.Fatal(err)
	}
	if err = processor.Process(ctx, payload); err != nil {
		t.Fatal(err)
	}
	assertProcessCounts(t, db, ctx, 1, 0, 1)
	if probe.Count("user_account") != 0 || probe.Count("permission_user_role") != 0 || probe.Count("message_notification_task_target") != 0 {
		t.Fatalf("platform audience read user tables: %v", probe.Statements())
	}
}
func TestProcessorUserAudienceCreatesRecipientsAndEventsOnce(t *testing.T) {
	db, ctx := openTaskDB(t)
	service := NewService(NewRepository(db, discardBatchJobWriter{}))
	task, err := service.Create(ctx, 1, DraftInput{PlatformID: 1, Title: "users", ContentHTML: "<p>content</p>", Variant: notification.VariantInfo, Priority: notification.PriorityNormal, LinkType: notification.LinkNone, AudienceType: AudienceUser, TargetIDs: []int64{1, 2}})
	if err != nil {
		t.Fatal(err)
	}
	task, err = service.Submit(ctx, task.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	processor := NewProcessor(db, realtime.NewRepository(db), discardBatchJobWriter{})
	payload := BatchPayload{SchemaVersion: 1, TaskID: task.ID, BatchNo: 0}
	if err = processor.Process(ctx, payload); err != nil {
		t.Fatal(err)
	}
	if err = processor.Process(ctx, payload); err != nil {
		t.Fatal(err)
	}
	assertProcessCounts(t, db, ctx, 1, 2, 2)
}

func TestProcessorKeepsPublishedAtStableAcrossBatches(t *testing.T) {
	db, ctx := openTaskDB(t)
	if err := db.WithContext(ctx).Exec(`INSERT INTO user_account(id,username,is_enabled,deleted_at) SELECT value,'user-' || value,1,NULL FROM generate_series(3,501) AS value`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO permission_user_role(user_id,role_id,created_at,updated_at) SELECT value,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP FROM generate_series(3,501) AS value`).Error; err != nil {
		t.Fatal(err)
	}
	targets := make([]int64, 501)
	for index := range targets {
		targets[index] = int64(index + 1)
	}
	service := NewService(NewRepository(db, discardBatchJobWriter{}))
	task, err := service.Create(ctx, 1, DraftInput{PlatformID: 1, Title: "two batches", ContentHTML: "<p>content</p>", Variant: notification.VariantInfo, Priority: notification.PriorityNormal, LinkType: notification.LinkNone, AudienceType: AudienceUser, TargetIDs: targets})
	if err != nil {
		t.Fatal(err)
	}
	task, err = service.Submit(ctx, task.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	processor := NewProcessor(db, realtime.NewRepository(db), discardBatchJobWriter{})
	if err = processor.Process(ctx, BatchPayload{SchemaVersion: 1, TaskID: task.ID, BatchNo: 0}); err != nil {
		t.Fatal(err)
	}
	if err = processor.Process(ctx, BatchPayload{SchemaVersion: 1, TaskID: task.ID, BatchNo: 1}); err != nil {
		t.Fatal(err)
	}

	var notificationPublishedAt, taskPublishedAt time.Time
	if err = db.WithContext(ctx).Raw(`SELECT published_at FROM message_notification WHERE source_task_id=?`, task.ID).Scan(&notificationPublishedAt).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.WithContext(ctx).Raw(`SELECT published_at FROM message_notification_task WHERE id=?`, task.ID).Scan(&taskPublishedAt).Error; err != nil {
		t.Fatal(err)
	}
	if !taskPublishedAt.Equal(notificationPublishedAt) {
		t.Fatalf("task publishedAt=%s notification publishedAt=%s", taskPublishedAt, notificationPublishedAt)
	}
	var payloads [][]byte
	if err = db.WithContext(ctx).Raw(`SELECT payload FROM realtime_event ORDER BY sequence`).Scan(&payloads).Error; err != nil {
		t.Fatal(err)
	}
	if len(payloads) != 501 {
		t.Fatalf("event payload count=%d want=501", len(payloads))
	}
	for index, payload := range payloads {
		var event struct {
			PublishedAt time.Time `json:"publishedAt"`
		}
		if err = json.Unmarshal(payload, &event); err != nil {
			t.Fatalf("decode payload %d: %v", index, err)
		}
		if !event.PublishedAt.Equal(notificationPublishedAt) {
			t.Fatalf("payload %d publishedAt=%s want=%s", index, event.PublishedAt, notificationPublishedAt)
		}
	}
}

func TestProcessorUserAndRoleAudienceUseOneKeysetSelectionQuery(t *testing.T) {
	for _, audience := range []AudienceType{AudienceUser, AudienceRole} {
		t.Run(string(audience), func(t *testing.T) {
			db, ctx := openTaskDB(t)
			if audience == AudienceRole {
				if err := db.WithContext(ctx).Exec(`INSERT INTO permission_user_role(user_id,role_id,created_at,updated_at) VALUES(1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),(2,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error; err != nil {
					t.Fatal(err)
				}
			}
			service := NewService(NewRepository(db, discardBatchJobWriter{}))
			task, err := service.Create(ctx, 1, DraftInput{PlatformID: 1, Title: "keyset", ContentHTML: "<p>content</p>", Variant: notification.VariantInfo, Priority: notification.PriorityNormal, LinkType: notification.LinkNone, AudienceType: audience, TargetIDs: []int64{1}})
			if err != nil {
				t.Fatal(err)
			}
			task, err = service.Submit(ctx, task.ID, time.Now().UTC())
			if err != nil {
				t.Fatal(err)
			}
			probe := testquery.New(db.Logger, "message_notification_task_target", "permission_user_role", "user_account")
			measured := db.Session(&gorm.Session{Logger: probe})
			if err = NewProcessor(measured, realtime.NewRepository(measured), discardBatchJobWriter{}).Process(ctx, BatchPayload{SchemaVersion: 1, TaskID: task.ID, BatchNo: 0}); err != nil {
				t.Fatal(err)
			}
			selectionTable := "message_notification_task_target"
			if audience == AudienceRole {
				selectionTable = "permission_user_role"
			}
			if probe.Count(selectionTable) != 1 {
				t.Fatalf("%s selection queries=%d statements=%v", audience, probe.Count(selectionTable), probe.Statements())
			}
			for _, statement := range probe.Statements() {
				if strings.Contains(strings.ToUpper(statement), " OFFSET ") {
					t.Fatalf("%s uses OFFSET: %s", audience, statement)
				}
			}
		})
	}
}

func TestProcessorRollsBackRecipientsAndEventsWhenTaskAdvanceFails(t *testing.T) {
	db, ctx := openTaskDB(t)
	service := NewService(NewRepository(db, discardBatchJobWriter{}))
	task, err := service.Create(ctx, 1, DraftInput{PlatformID: 1, Title: "rollback", ContentHTML: "<p>content</p>", Variant: notification.VariantInfo, Priority: notification.PriorityNormal, LinkType: notification.LinkNone, AudienceType: AudienceUser, TargetIDs: []int64{1, 2}})
	if err != nil {
		t.Fatal(err)
	}
	task, err = service.Submit(ctx, task.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err = db.WithContext(ctx).Exec(`CREATE FUNCTION reject_task_advance() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.next_batch_no > OLD.next_batch_no THEN RAISE EXCEPTION 'forced task advance failure'; END IF; RETURN NEW; END $$`).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.WithContext(ctx).Exec(`CREATE TRIGGER reject_task_advance BEFORE UPDATE ON message_notification_task FOR EACH ROW EXECUTE FUNCTION reject_task_advance()`).Error; err != nil {
		t.Fatal(err)
	}
	if err = NewProcessor(db, realtime.NewRepository(db), discardBatchJobWriter{}).Process(ctx, BatchPayload{SchemaVersion: 1, TaskID: task.ID, BatchNo: 0}); err == nil {
		t.Fatal("process succeeded despite forced task advance failure")
	}
	assertProcessCounts(t, db, ctx, 0, 0, 0)
}

func TestCancelWaitsForBatchLockAndCannotOverwriteCompletion(t *testing.T) {
	db, ctx := openTaskDB(t)
	service := NewService(NewRepository(db, discardBatchJobWriter{}))
	task, err := service.Create(ctx, 1, DraftInput{PlatformID: 1, Title: "race", ContentHTML: "<p>content</p>", Variant: notification.VariantInfo, Priority: notification.PriorityNormal, LinkType: notification.LinkNone, AudienceType: AudienceUser, TargetIDs: []int64{1}})
	if err != nil {
		t.Fatal(err)
	}
	task, err = service.Submit(ctx, task.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var barrier sync.Once
	if err = db.Callback().Update().Before("gorm:update").Register("test:block_task_advance", func(tx *gorm.DB) {
		if tx.Statement.Table == "message_notification_task" {
			barrier.Do(func() {
				close(entered)
				<-release
			})
		}
	}); err != nil {
		t.Fatal(err)
	}
	processResult := make(chan error, 1)
	go func() {
		processResult <- NewProcessor(db, realtime.NewRepository(db), discardBatchJobWriter{}).Process(ctx, BatchPayload{SchemaVersion: 1, TaskID: task.ID, BatchNo: 0})
	}()
	<-entered
	cancelStarted, cancelResult := make(chan struct{}), make(chan error, 1)
	go func() {
		close(cancelStarted)
		_, cancelErr := service.Cancel(ctx, task.ID, time.Now().UTC())
		cancelResult <- cancelErr
	}()
	<-cancelStarted
	select {
	case cancelErr := <-cancelResult:
		t.Fatalf("cancel bypassed batch row lock: %v", cancelErr)
	default:
	}
	close(release)
	if err = <-processResult; err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if err = <-cancelResult; !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("cancel error=%v want ErrInvalidTransition", err)
	}
	var status Status
	if err = db.WithContext(ctx).Raw(`SELECT status FROM message_notification_task WHERE id=?`, task.ID).Scan(&status).Error; err != nil || status != StatusCompleted {
		t.Fatalf("status=%d err=%v", status, err)
	}
}
func assertProcessCounts(t *testing.T, db *gorm.DB, ctx context.Context, notifications, recipients, events int64) {
	t.Helper()
	for table, want := range map[string]int64{"message_notification": notifications, "message_notification_recipient": recipients, "realtime_event": events, "realtime_event_outbox": events} {
		var got int64
		if err := db.WithContext(ctx).Raw("SELECT count(*) FROM " + table).Scan(&got).Error; err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s=%d want=%d", table, got, want)
		}
	}
}
