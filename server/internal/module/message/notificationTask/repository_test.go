package notificationtask

import (
	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"admin/server/internal/module/message/notification"
	"context"
	"errors"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

type discardBatchJobWriter struct{}

func (discardBatchJobWriter) CreateBatchJobTx(context.Context, *gorm.DB, int64, int, time.Time, time.Time) error {
	return nil
}
func (discardBatchJobWriter) CancelBatchJobsTx(context.Context, *gorm.DB, int64, time.Time) error {
	return nil
}

func TestRepositoryRejectsPastScheduledTimeAtSubmit(t *testing.T) {
	db, ctx := openTaskDB(t)
	service := NewService(NewRepository(db, discardBatchJobWriter{}))
	past := time.Now().UTC().Add(-time.Minute)
	created, err := service.Create(ctx, 1, DraftInput{PlatformID: 1, Title: "past", ContentHTML: "<p>content</p>", Variant: notification.VariantInfo, Priority: notification.PriorityNormal, LinkType: notification.LinkNone, AudienceType: AudiencePlatform, ScheduledAt: &past})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Submit(ctx, created.ID, time.Now().UTC()); !errors.Is(err, ErrInvalidFacts) {
		t.Fatalf("submit error=%v want ErrInvalidFacts", err)
	}
	var task Task
	if err = db.WithContext(ctx).Where("id=?", created.ID).Take(&task).Error; err != nil {
		t.Fatal(err)
	}
	if task.Status != StatusDraft || task.SubmittedAt != nil {
		t.Fatalf("task changed after rejected submit: %+v", task)
	}
}

func TestRepositoryListFiltersStatusAcrossTheWholePage(t *testing.T) {
	db, ctx := openTaskDB(t)
	service := NewService(NewRepository(db, discardBatchJobWriter{}))
	input := DraftInput{PlatformID: 1, Title: "draft", ContentHTML: "<p>content</p>", Variant: notification.VariantInfo, Priority: notification.PriorityNormal, LinkType: notification.LinkNone, AudienceType: AudiencePlatform}
	draft, err := service.Create(ctx, 1, input)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.Create(ctx, 1, DraftInput{PlatformID: 1, Title: "completed", ContentHTML: "<p>content</p>", Variant: notification.VariantInfo, Priority: notification.PriorityNormal, LinkType: notification.LinkNone, AudienceType: AudiencePlatform})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.WithContext(ctx).Model(&Task{}).Where("id=?", completed.ID).Updates(map[string]any{"status": StatusCompleted, "completed_at": time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	rows, total, err := NewRepository(db, discardBatchJobWriter{}).List(ctx, ListQuery{Status: StatusDraft, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 || rows[0].ID != draft.ID {
		t.Fatalf("total=%d rows=%+v", total, rows)
	}
	if rows[0].PlatformName != "Admin" {
		t.Fatalf("platform name=%q want Admin", rows[0].PlatformName)
	}
}

func TestRepositoryListAppliesEveryApprovedFilter(t *testing.T) {
	db, ctx := openTaskDB(t)
	service := NewService(NewRepository(db, discardBatchJobWriter{}))
	matching, err := service.Create(ctx, 1, DraftInput{PlatformID: 1, Title: "Planned Maintenance", ContentHTML: "<p>content</p>", Variant: notification.VariantWarning, Priority: notification.PriorityNormal, LinkType: notification.LinkNone, AudienceType: AudienceRole, TargetIDs: []int64{1}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Create(ctx, 1, DraftInput{PlatformID: 1, Title: "Other", ContentHTML: "<p>content</p>", Variant: notification.VariantInfo, Priority: notification.PriorityNormal, LinkType: notification.LinkNone, AudienceType: AudiencePlatform}); err != nil {
		t.Fatal(err)
	}
	from := matching.CreatedAt.Add(-time.Second)
	to := matching.CreatedAt.Add(time.Second)
	rows, total, err := NewRepository(db, discardBatchJobWriter{}).List(ctx, ListQuery{PlatformID: &matching.PlatformID, Status: StatusDraft, AudienceType: AudienceRole, Keyword: "maintenance", From: &from, To: &to, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 || rows[0].ID != matching.ID {
		t.Fatalf("total=%d rows=%+v", total, rows)
	}
}

func TestRepositoryListTreatsKeywordWildcardsLiterally(t *testing.T) {
	db, ctx := openTaskDB(t)
	service := NewService(NewRepository(db, discardBatchJobWriter{}))
	matching, err := service.Create(ctx, 1, DraftInput{PlatformID: 1, Title: `Release 100%_ready`, ContentHTML: "<p>content</p>", Variant: notification.VariantInfo, Priority: notification.PriorityNormal, LinkType: notification.LinkNone, AudienceType: AudiencePlatform})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Create(ctx, 1, DraftInput{PlatformID: 1, Title: `Release 100X-ready`, ContentHTML: "<p>content</p>", Variant: notification.VariantInfo, Priority: notification.PriorityNormal, LinkType: notification.LinkNone, AudienceType: AudiencePlatform}); err != nil {
		t.Fatal(err)
	}
	rows, total, err := NewRepository(db, discardBatchJobWriter{}).List(ctx, ListQuery{Keyword: `%_`, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 || rows[0].ID != matching.ID {
		t.Fatalf("total=%d rows=%+v", total, rows)
	}
}

func TestRepositoryDraftSubmitCancelCopyTransactions(t *testing.T) {
	db, ctx := openTaskDB(t)
	service := NewService(NewRepository(db, discardBatchJobWriter{}))
	future := time.Now().UTC().Add(time.Hour)
	input := DraftInput{PlatformID: 1, Title: "notice", ContentHTML: "<p>content</p>", Variant: notification.VariantInfo, Priority: notification.PriorityNormal, LinkType: notification.LinkNone, AudienceType: AudienceUser, TargetIDs: []int64{2, 1}, ScheduledAt: &future}
	created, err := service.Create(ctx, 1, input)
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != StatusDraft || len(created.TargetIDs) != 2 {
		t.Fatalf("created=%+v", created)
	}
	if created.PlatformName != "Admin" {
		t.Fatalf("created platform name=%q want Admin", created.PlatformName)
	}
	submitted, err := service.Submit(ctx, created.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if submitted.Status != StatusScheduled || submitted.AudienceMaxUserID == nil || *submitted.AudienceMaxUserID != 2 || submitted.SubmittedAt == nil {
		t.Fatalf("submitted=%+v", submitted)
	}
	if _, err = service.Submit(ctx, created.ID, time.Now().UTC()); err == nil {
		t.Fatal("repeat submit accepted")
	}
	canceled, err := service.Cancel(ctx, created.ID, time.Now().UTC())
	if err != nil || canceled.Status != StatusCanceled {
		t.Fatalf("canceled=%+v err=%v", canceled, err)
	}
	copied, err := service.Copy(ctx, created.ID, 2, time.Now().UTC())
	if err != nil || copied.Status != StatusDraft || copied.SubmittedAt != nil || copied.AudienceMaxUserID != nil || len(copied.TargetIDs) != 2 {
		t.Fatalf("copied=%+v err=%v", copied, err)
	}
}

func TestRepositoryRejectsTargetsWithoutPlatformAccess(t *testing.T) {
	db, ctx := openTaskDB(t)
	statements := []string{
		`INSERT INTO permission_role(id,code,name,is_enabled,deleted_at) VALUES(2,'canvas_viewer','Canvas viewer',1,NULL)`,
		`INSERT INTO permission_menu(id,platform_id,parent_id,menu_type,code,is_hidden,is_enabled,deleted_at) VALUES(20,2,NULL,'page','canvas:home:view',0,1,NULL)`,
		`INSERT INTO permission_role_menu(role_id,menu_id,created_at,updated_at) VALUES(2,20,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
		`INSERT INTO user_account(id,username,is_enabled,deleted_at) VALUES(3,'canvas-only',1,NULL)`,
		`INSERT INTO permission_user_role(user_id,role_id,created_at,updated_at) VALUES(3,2,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
	}
	for _, statement := range statements {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	service := NewService(NewRepository(db, discardBatchJobWriter{}))
	base := DraftInput{PlatformID: 1, Title: "platform check", ContentHTML: "<p>content</p>", Variant: notification.VariantInfo, Priority: notification.PriorityNormal, LinkType: notification.LinkNone}
	for _, audience := range []AudienceType{AudienceUser, AudienceRole} {
		base.AudienceType = audience
		base.TargetIDs = []int64{3}
		if _, err := service.Create(ctx, 1, base); err == nil {
			t.Fatalf("created %s task for target without platform access", audience)
		}
	}
}
func TestRepositoryRejectsPlatformWithoutCompleteNotificationCapability(t *testing.T) {
	db, ctx := openTaskDB(t)
	service := NewService(NewRepository(db, discardBatchJobWriter{}))
	input := DraftInput{PlatformID: 2, Title: "notice", ContentHTML: "<p>content</p>", Variant: notification.VariantInfo, Priority: notification.PriorityNormal, LinkType: notification.LinkNone, AudienceType: AudiencePlatform}
	if _, err := service.Create(ctx, 1, input); err == nil {
		t.Fatal("platform without capability accepted")
	}
}

func TestRepositoryPlatformOptionsSkipIneligibleRowsWithoutBreakingCursor(t *testing.T) {
	db, ctx := openTaskDB(t)
	for _, statement := range []string{
		`INSERT INTO permission_auth_platform VALUES(3,'Operations',1,NULL)`,
		`INSERT INTO permission_menu VALUES(30,3,NULL,'page','message:notification:view',1,1,NULL),(31,3,30,'action','message:notification:list',1,1,NULL),(32,3,30,'action','message:notification:read',1,1,NULL),(33,3,30,'action','message:notification:delete',1,1,NULL)`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	rows, err := NewRepository(db, discardBatchJobWriter{}).Options(ctx, "platform", 0, "", 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != 1 || rows[1].ID != 3 {
		t.Fatalf("platform options=%+v", rows)
	}
}

func TestRepositoryPlatformOptionsRejectIncompleteCapabilityFacts(t *testing.T) {
	db, ctx := openTaskDB(t)
	if err := db.WithContext(ctx).Exec(`INSERT INTO permission_menu VALUES(20,2,NULL,'page','message:notification:view',1,1,NULL)`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := NewRepository(db, discardBatchJobWriter{}).Options(ctx, "platform", 0, "", 0, 50); err == nil {
		t.Fatal("incomplete platform notification capability was silently skipped")
	}
}

func TestRepositoryOptionsTreatKeywordWildcardsAndBackslashesLiterally(t *testing.T) {
	db, ctx := openTaskDB(t)
	if err := db.WithContext(ctx).Exec(`UPDATE permission_auth_platform SET name=? WHERE id=1`, `Admin 100%_C:\west`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`UPDATE user_account SET username=? WHERE id=1`, `user%_C:\west`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`UPDATE permission_role SET name=? WHERE id=1`, `role%_C:\west`).Error; err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		kind string
		want int64
	}{
		{kind: "platform", want: 1},
		{kind: "user", want: 1},
		{kind: "role", want: 1},
	} {
		rows, err := NewRepository(db, discardBatchJobWriter{}).Options(ctx, test.kind, 1, `%_C:\`, 0, 50)
		if err != nil {
			t.Fatalf("kind=%s: %v", test.kind, err)
		}
		if len(rows) != 1 || rows[0].ID != test.want {
			t.Fatalf("kind=%s rows=%+v", test.kind, rows)
		}
	}
}
func openTaskDB(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	db, ctx := testschema.Open(t, taskDSN(t), "test_notification_task")
	statements := []string{
		`CREATE TABLE permission_auth_platform(id BIGINT PRIMARY KEY,name TEXT NOT NULL,is_enabled SMALLINT NOT NULL,deleted_at TIMESTAMPTZ NULL)`, `INSERT INTO permission_auth_platform VALUES(1,'Admin',1,NULL),(2,'Canvas',1,NULL)`,
		`CREATE TABLE user_account(id BIGINT PRIMARY KEY,username TEXT NOT NULL,is_enabled SMALLINT NOT NULL,deleted_at TIMESTAMPTZ NULL)`, `INSERT INTO user_account VALUES(1,'one',1,NULL),(2,'two',1,NULL)`,
		`CREATE TABLE permission_role(id BIGINT PRIMARY KEY,code TEXT NOT NULL,name TEXT NOT NULL,is_enabled SMALLINT NOT NULL,deleted_at TIMESTAMPTZ NULL)`, `INSERT INTO permission_role VALUES(1,'role','role',1,NULL)`,
		`CREATE TABLE permission_user_role(id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,user_id BIGINT NOT NULL,role_id BIGINT NOT NULL,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL,deleted_at TIMESTAMPTZ NULL)`, `INSERT INTO permission_user_role(user_id,role_id,created_at,updated_at) VALUES(1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),(2,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
		`CREATE TABLE permission_menu(id BIGINT PRIMARY KEY,platform_id BIGINT NOT NULL,parent_id BIGINT NULL,menu_type TEXT NOT NULL,code TEXT NOT NULL,is_hidden SMALLINT NOT NULL,is_enabled SMALLINT NOT NULL,deleted_at TIMESTAMPTZ NULL)`, `INSERT INTO permission_menu VALUES(10,1,NULL,'page','message:notification:view',1,1,NULL),(11,1,10,'action','message:notification:list',1,1,NULL),(12,1,10,'action','message:notification:read',1,1,NULL),(13,1,10,'action','message:notification:delete',1,1,NULL)`,
		`CREATE TABLE permission_role_menu(id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,role_id BIGINT NOT NULL,menu_id BIGINT NOT NULL,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL,deleted_at TIMESTAMPTZ NULL)`, `INSERT INTO permission_role_menu(role_id,menu_id,created_at,updated_at) SELECT 1,id,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP FROM permission_menu WHERE platform_id=1`,
		`CREATE TABLE message_notification_task(id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,platform_id BIGINT NOT NULL,title TEXT NOT NULL,content_html TEXT NOT NULL,summary TEXT NOT NULL,variant TEXT NOT NULL,priority TEXT NOT NULL,link_type TEXT NOT NULL,link TEXT NOT NULL,audience_type TEXT NOT NULL,scheduled_at TIMESTAMPTZ NULL,audience_max_user_id BIGINT NULL,submitted_at TIMESTAMPTZ NULL,status SMALLINT NOT NULL,next_user_id BIGINT NOT NULL DEFAULT 0,next_batch_no INTEGER NOT NULL DEFAULT 0,generated_count BIGINT NOT NULL DEFAULT 0,failure_message TEXT NULL,published_at TIMESTAMPTZ NULL,completed_at TIMESTAMPTZ NULL,canceled_at TIMESTAMPTZ NULL,failed_at TIMESTAMPTZ NULL,created_by BIGINT NOT NULL,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL,deleted_at TIMESTAMPTZ NULL)`,
		`CREATE TABLE message_notification_task_target(id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,task_id BIGINT NOT NULL REFERENCES message_notification_task(id) ON DELETE CASCADE,target_type TEXT NOT NULL,target_id BIGINT NOT NULL,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL,deleted_at TIMESTAMPTZ NULL)`, `CREATE UNIQUE INDEX task_target_active ON message_notification_task_target(task_id,target_type,target_id) WHERE deleted_at IS NULL`,
		`CREATE TABLE message_notification_dispatch_outbox(id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,task_id BIGINT NOT NULL REFERENCES message_notification_task(id) ON DELETE CASCADE,batch_no INTEGER NOT NULL,attempts INTEGER NOT NULL DEFAULT 0,available_at TIMESTAMPTZ NOT NULL,locked_until TIMESTAMPTZ NULL,lock_token TEXT NULL,published_at TIMESTAMPTZ NULL,last_error TEXT NULL,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL)`, `CREATE UNIQUE INDEX dispatch_batch ON message_notification_dispatch_outbox(task_id,batch_no)`,
		`CREATE TABLE message_notification(id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,platform_id BIGINT NOT NULL,source_task_id BIGINT NULL,source_type TEXT NOT NULL,source_key TEXT NOT NULL,audience_type TEXT NOT NULL,audience_max_user_id BIGINT NOT NULL,title TEXT NOT NULL,content_html TEXT NOT NULL,summary TEXT NOT NULL,variant TEXT NOT NULL,priority TEXT NOT NULL,link_type TEXT NOT NULL,link TEXT NOT NULL,published_at TIMESTAMPTZ NOT NULL,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL)`, `CREATE UNIQUE INDEX notification_source_task ON message_notification(source_task_id) WHERE source_task_id IS NOT NULL`,
		`CREATE TABLE message_notification_recipient(id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,notification_id BIGINT NOT NULL,platform_id BIGINT NOT NULL,user_id BIGINT NOT NULL,read_at TIMESTAMPTZ NULL,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL,deleted_at TIMESTAMPTZ NULL)`, `CREATE UNIQUE INDEX notification_recipient ON message_notification_recipient(notification_id,user_id)`,
		`CREATE TABLE realtime_event(sequence BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,event_id UUID NOT NULL UNIQUE,dedup_key TEXT NOT NULL UNIQUE,platform_id BIGINT NOT NULL,event_type TEXT NOT NULL,target_type TEXT NOT NULL,target_user_id BIGINT NULL,audience_max_user_id BIGINT NULL,payload JSONB NOT NULL,occurred_at TIMESTAMPTZ NOT NULL,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE realtime_event_outbox(id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,event_sequence BIGINT NOT NULL UNIQUE REFERENCES realtime_event(sequence) ON DELETE CASCADE,attempts INTEGER NOT NULL,available_at TIMESTAMPTZ NOT NULL,locked_until TIMESTAMPTZ NULL,lock_token TEXT NULL,published_at TIMESTAMPTZ NULL,last_error TEXT NULL,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL)`,
	}
	for _, statement := range statements {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db, ctx
}
func taskDSN(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
	if err := godotenv.Load(filepath.Join(root, "server", ".env")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	return settings.PostgresDSN
}
