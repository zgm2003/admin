package notification

import (
	"context"
	"strings"
	"testing"
	"time"

	"admin/server/internal/database/testquery"
	"admin/server/internal/module/realtime"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

func TestMailboxListAndSummaryCombineTargetedAndPlatformWithIsolation(t *testing.T) {
	db, ctx := openNotificationRepositoryDB(t)
	repository := NewRepository(db, realtime.NewRepository(db))
	now := time.Now().UTC()
	service := NewService(repository, mailboxSettingStub{record: sharedsetting.Record{Key: sharedsetting.MessageNotificationRetentionDaysKey, Value: "180", ValueType: sharedsetting.ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes}})
	targeted, err := service.CreateForUsers(ctx, validCreateForUsersInput(1, "mailbox-targeted", []int64{1}))
	if err != nil {
		t.Fatal(err)
	}
	broadcast := insertBroadcastNotification(t, db, ctx, 1, 2, "mailbox-broadcast", now)
	insertBroadcastNotification(t, db, ctx, 1, 0, "mailbox-new-user", now)
	insertBroadcastNotification(t, db, ctx, 2, 2, "mailbox-other-platform", now)

	page, err := service.List(ctx, MailboxQuery{PlatformID: 1, UserID: 1, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.Items[0].ID != broadcast.ID || page.Items[1].ID != targeted.ID || page.NextBeforeID != nil {
		t.Fatalf("page=%+v", page)
	}
	summary, err := service.Summary(ctx, 1, 1)
	if err != nil || summary.UnreadCount != 2 || len(summary.Recent) != 2 {
		t.Fatalf("summary=%+v err=%v", summary, err)
	}
}

func TestMailboxListAndSummaryUseOneKeysetQueryEach(t *testing.T) {
	db, ctx := openNotificationRepositoryDB(t)
	baseRepository := NewRepository(db, realtime.NewRepository(db))
	service := NewService(baseRepository, mailboxSettingStub{record: sharedsetting.Record{Key: sharedsetting.MessageNotificationRetentionDaysKey, Value: "180", ValueType: sharedsetting.ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes}})
	if _, err := service.CreateForUsers(ctx, validCreateForUsersInput(1, "query-budget", []int64{1})); err != nil {
		t.Fatal(err)
	}
	insertBroadcastNotification(t, db, ctx, 1, 2, "query-budget-broadcast", time.Now().UTC())

	probe := testquery.New(db.Logger, "message_notification", "message_notification_recipient", "message_notification_broadcast_state", "message_notification_mailbox_state")
	measured := db.Session(&gorm.Session{Logger: probe})
	measuredService := NewService(NewRepository(measured, realtime.NewRepository(measured)), service.settings)
	if _, err := measuredService.List(ctx, MailboxQuery{PlatformID: 1, UserID: 1, Limit: 20}); err != nil {
		t.Fatal(err)
	}
	assertSingleKeysetSelect(t, probe, "list")
	probe.Reset()
	if _, err := measuredService.Summary(ctx, 1, 1); err != nil {
		t.Fatal(err)
	}
	assertSingleKeysetSelect(t, probe, "summary")
}

func assertSingleKeysetSelect(t *testing.T, probe *testquery.Counter, operation string) {
	t.Helper()
	if probe.Total() != 1 {
		t.Fatalf("%s SELECT count=%d want=1 statements=%v", operation, probe.Total(), probe.Statements())
	}
	if strings.Contains(strings.ToUpper(probe.Statements()[0]), " OFFSET ") {
		t.Fatalf("%s uses OFFSET: %s", operation, probe.Statements()[0])
	}
}

func TestMailboxMutationsAreAtomicAndIdempotent(t *testing.T) {
	db, ctx := openNotificationRepositoryDB(t)
	repository := NewRepository(db, realtime.NewRepository(db))
	service := NewService(repository, mailboxSettingStub{record: sharedsetting.Record{Key: sharedsetting.MessageNotificationRetentionDaysKey, Value: "180", ValueType: sharedsetting.ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes}})
	first, err := service.CreateForUsers(ctx, validCreateForUsersInput(1, "mutation-one", []int64{1}))
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateForUsers(ctx, validCreateForUsersInput(1, "mutation-two", []int64{1}))
	if err != nil {
		t.Fatal(err)
	}
	before := realtimeEventCount(t, db, ctx)
	if err := service.Read(ctx, 1, 1, first.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.Read(ctx, 1, 1, first.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.ReadAll(ctx, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := service.ReadAll(ctx, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, 1, 1, second.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, 1, 1, second.ID); err != nil {
		t.Fatal(err)
	}
	after := realtimeEventCount(t, db, ctx)
	if after-before != 3 {
		t.Fatalf("state events=%d want=3", after-before)
	}
	page, err := service.List(ctx, MailboxQuery{PlatformID: 1, UserID: 1, Limit: 50})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != first.ID || !page.Items[0].IsRead {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}

func insertBroadcastNotification(t *testing.T, db *gorm.DB, ctx context.Context, platformID, maxUserID int64, source string, now time.Time) Notification {
	t.Helper()
	row := Notification{PlatformID: platformID, SourceType: "system.test", SourceKey: source, AudienceType: AudiencePlatform, AudienceMaxUserID: maxUserID, Title: source, ContentHTML: "<p>broadcast</p>", Summary: "broadcast", Variant: VariantInfo, Priority: PriorityNormal, LinkType: LinkNone, PublishedAt: now, CreatedAt: now, UpdatedAt: now}
	if err := db.WithContext(ctx).Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

func realtimeEventCount(t *testing.T, db *gorm.DB, ctx context.Context) int64 {
	t.Helper()
	var count int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM realtime_event`).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

type mailboxSettingStub struct{ record sharedsetting.Record }

func (s mailboxSettingStub) FindByKey(context.Context, string) (sharedsetting.Record, error) {
	return s.record, nil
}
