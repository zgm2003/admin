package architecture_test

import (
	"context"
	"os"
	"testing"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

const mailLogImmutableMigrationPath = "../../../docs/database/2026-09-09-mail-log-immutable.sql"

func TestMailLogImmutableMigrationRemovesDeletionAndIsIdempotent(t *testing.T) {
	database, ctx := openMailLogImmutableSchema(t, false)
	script, err := os.ReadFile(mailLogImmutableMigrationPath)
	if err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 2; run++ {
		if err := database.WithContext(ctx).Exec(string(script)).Error; err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		var result struct {
			DeletedColumns, OldIndexes, NewIndexes, ActiveDeleteActions int64
			AdminMenuVersion, CanvasMenuVersion                         int64
		}
		if err := database.WithContext(ctx).Raw(`SELECT
(SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name IN ('message_mail_log','message_mail_log_verification') AND column_name='deleted_at') deleted_columns,
(SELECT count(*) FROM pg_indexes WHERE schemaname=current_schema() AND indexname IN ('ux_message_mail_log_platform_challenge_active','ux_message_mail_verification_log_active')) old_indexes,
(SELECT count(*) FROM pg_indexes WHERE schemaname=current_schema() AND indexname IN ('ux_message_mail_log_platform_challenge','ux_message_mail_verification_log','ix_message_mail_log_platform_id_desc','ix_message_mail_log_to_email_prefix','ix_message_mail_log_scene_id_desc','ix_message_mail_log_status_id_desc','ix_message_mail_log_created_id_desc','ix_permission_auth_platform_code_history_prefix')) new_indexes,
(SELECT count(*) FROM permission_menu WHERE code='message:mail:log:delete' AND deleted_at IS NULL) active_delete_actions,
(SELECT menu_version FROM permission_auth_platform WHERE id=1) admin_menu_version,
(SELECT menu_version FROM permission_auth_platform WHERE id=2) canvas_menu_version`).Scan(&result).Error; err != nil {
			t.Fatal(err)
		}
		if result.DeletedColumns != 0 || result.OldIndexes != 0 || result.NewIndexes != 8 || result.ActiveDeleteActions != 0 || result.AdminMenuVersion != 4 || result.CanvasMenuVersion != 9 {
			t.Fatalf("run %d result=%+v", run, result)
		}
	}

	if err := database.WithContext(ctx).Exec(`INSERT INTO message_mail_log(platform_id,challenge_id,to_email,scene,status,created_at,updated_at) VALUES (1,'challenge-1','second@example.com','login','pending',now(),now())`).Error; err == nil {
		t.Fatal("migration still allows a challenge ID to be reused")
	}
	if err := database.WithContext(ctx).Exec(`INSERT INTO message_mail_log_verification(platform_id,mail_log_id,created_at) VALUES (1,1,now())`).Error; err == nil {
		t.Fatal("migration still allows a second verification record per mail log")
	}
}

func TestMailLogImmutableMigrationRejectsSoftDeletedAuditRowsAndRollsBack(t *testing.T) {
	database, ctx := openMailLogImmutableSchema(t, true)
	script, err := os.ReadFile(mailLogImmutableMigrationPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.WithContext(ctx).Connection(func(connection *gorm.DB) error {
		_, migrationErr := connection.Statement.ConnPool.ExecContext(ctx, string(script))
		if _, rollbackErr := connection.Statement.ConnPool.ExecContext(ctx, "ROLLBACK"); rollbackErr != nil {
			return rollbackErr
		}
		if migrationErr == nil {
			t.Error("migration accepted a soft-deleted audit row")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var result struct {
		DeletedColumns, OldIndexes, ActiveDeleteActions, AdminMenuVersion int64
	}
	if err := database.WithContext(ctx).Raw(`SELECT
(SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name IN ('message_mail_log','message_mail_log_verification') AND column_name='deleted_at') deleted_columns,
(SELECT count(*) FROM pg_indexes WHERE schemaname=current_schema() AND indexname IN ('ux_message_mail_log_platform_challenge_active','ux_message_mail_verification_log_active')) old_indexes,
(SELECT count(*) FROM permission_menu WHERE code='message:mail:log:delete' AND deleted_at IS NULL) active_delete_actions,
(SELECT menu_version FROM permission_auth_platform WHERE id=1) admin_menu_version`).Scan(&result).Error; err != nil {
		t.Fatal(err)
	}
	if result.DeletedColumns != 2 || result.OldIndexes != 2 || result.ActiveDeleteActions != 1 || result.AdminMenuVersion != 3 {
		t.Fatalf("failed migration changed state: %+v", result)
	}
}

func openMailLogImmutableSchema(t *testing.T, withDeletedLog bool) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	_ = godotenv.Load("../../.env")
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	database, ctx := testschema.Open(t, settings.PostgresDSN, "test_mail_log_immutable")
	fixture := `
CREATE TABLE permission_auth_platform(id bigint PRIMARY KEY,code varchar(49) NOT NULL,menu_version bigint NOT NULL,updated_at timestamptz NOT NULL,deleted_at timestamptz);
CREATE TABLE permission_menu(id bigint PRIMARY KEY,platform_id bigint NOT NULL REFERENCES permission_auth_platform(id),code varchar(128) NOT NULL,is_enabled smallint NOT NULL,updated_at timestamptz NOT NULL,deleted_at timestamptz);
CREATE TABLE message_mail_log(id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,platform_id bigint NOT NULL REFERENCES permission_auth_platform(id),challenge_id varchar(128),to_email varchar(254) NOT NULL,scene varchar(32) NOT NULL,status varchar(16) NOT NULL,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,deleted_at timestamptz,UNIQUE(id,platform_id));
CREATE UNIQUE INDEX ux_message_mail_log_platform_challenge_active ON message_mail_log(platform_id,challenge_id) WHERE deleted_at IS NULL AND challenge_id IS NOT NULL;
CREATE TABLE message_mail_log_verification(id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,platform_id bigint NOT NULL,mail_log_id bigint NOT NULL,created_at timestamptz NOT NULL,deleted_at timestamptz,FOREIGN KEY(mail_log_id,platform_id) REFERENCES message_mail_log(id,platform_id));
CREATE UNIQUE INDEX ux_message_mail_verification_log_active ON message_mail_log_verification(mail_log_id) WHERE deleted_at IS NULL;
INSERT INTO permission_auth_platform VALUES (1,'admin',3,now(),NULL),(2,'canvas',9,now(),NULL);
INSERT INTO permission_menu VALUES (10,1,'message:mail:log:delete',1,now(),NULL),(20,2,'unrelated:view',1,now(),NULL);
INSERT INTO message_mail_log(platform_id,challenge_id,to_email,scene,status,created_at,updated_at,deleted_at) VALUES (1,'challenge-1','user@example.com','login','sent',now(),now(),NULL);
INSERT INTO message_mail_log_verification(platform_id,mail_log_id,created_at,deleted_at) VALUES (1,1,now(),NULL);`
	if withDeletedLog {
		fixture += `UPDATE message_mail_log SET deleted_at=now() WHERE id=1;`
	}
	if err := database.WithContext(ctx).Exec(fixture).Error; err != nil {
		t.Fatal(err)
	}
	return database, ctx
}
