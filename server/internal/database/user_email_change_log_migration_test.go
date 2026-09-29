package database_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"admin/server/internal/database/testschema"
)

func TestUserEmailChangeLogMigrationBackfillsPlaintextAndSeedsDetailAction(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_user_email_change_log_numeric")
	for _, statement := range []string{
		`CREATE TABLE permission_auth_platform (id BIGINT PRIMARY KEY, code VARCHAR(49) NOT NULL, deleted_at TIMESTAMPTZ, menu_version BIGINT NOT NULL DEFAULT 1, updated_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		`CREATE TABLE permission_menu (id BIGSERIAL PRIMARY KEY, platform_id BIGINT NOT NULL, parent_id BIGINT, menu_type VARCHAR(16) NOT NULL, code VARCHAR(128) NOT NULL, i18n_key VARCHAR(128), path VARCHAR(255), component_path VARCHAR(255), icon VARCHAR(128), sort_order INT NOT NULL DEFAULT 0, is_enabled SMALLINT NOT NULL DEFAULT 1, is_hidden SMALLINT NOT NULL DEFAULT 0, name VARCHAR(128) NOT NULL, remark VARCHAR(512), deleted_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		`CREATE TABLE user_account (id BIGINT PRIMARY KEY, username VARCHAR(64) NOT NULL)`,
		`CREATE TABLE message_mail_log (id BIGINT PRIMARY KEY, user_id BIGINT, platform_id BIGINT NOT NULL, scene VARCHAR(32) NOT NULL, to_email VARCHAR(254) NOT NULL, status SMALLINT NOT NULL, created_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE user_email_change_log (id BIGINT PRIMARY KEY, user_id BIGINT NOT NULL, platform_id BIGINT NOT NULL, action VARCHAR(16) NOT NULL CHECK (action IN ('bind','change')), old_email_hint VARCHAR(128) NOT NULL DEFAULT '', old_email_hmac VARCHAR(128) NOT NULL DEFAULT '', new_email_hint VARCHAR(128) NOT NULL DEFAULT '', new_email_hmac VARCHAR(128) NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL)`,
		`INSERT INTO permission_auth_platform(id, code, menu_version) VALUES (1, 'admin', 1)`,
		`INSERT INTO permission_menu(id, platform_id, menu_type, code, name) VALUES (10, 1, 'page', 'user:account:view', 'Users')`,
		`INSERT INTO user_account(id, username) VALUES (718, 'alice')`,
		`INSERT INTO message_mail_log(id, user_id, platform_id, scene, to_email, status, created_at) VALUES (17, 718, 1, 'bind_email', 'zgm_2003@qq.com', 2, '2026-09-29 13:48:50+08'), (18, 718, 1, 'bind_email', 'test@qq.com', 3, '2026-09-29 13:49:08+08'), (19, 718, 1, 'bind_email', '123456@qq.com', 2, '2026-09-29 13:50:13+08')`,
		`INSERT INTO user_email_change_log(id, user_id, platform_id, action, old_email_hint, new_email_hint, created_at, updated_at) VALUES (1, 718, 1, 'change', 'z***@qq.com', '1***@qq.com', '2026-09-29 13:50:23+08', '2026-09-29 13:50:23+08')`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("prepare fixture: %v", err)
		}
	}
	script := readUserEmailChangeLogMigration(t)
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("execute migration: %v", err)
	}
	var actionType, oldEmail, newEmail string
	if err := db.WithContext(ctx).Raw(`SELECT c.data_type, l.old_email, l.new_email FROM information_schema.columns c CROSS JOIN user_email_change_log l WHERE c.table_name='user_email_change_log' AND c.column_name='action' AND l.id=1`).Row().Scan(&actionType, &oldEmail, &newEmail); err != nil {
		t.Fatal(err)
	}
	if actionType != "smallint" || oldEmail != "zgm_2003@qq.com" || newEmail != "123456@qq.com" {
		t.Fatalf("action type=%q old=%q new=%q", actionType, oldEmail, newEmail)
	}
	var code, menuType, parentID string
	var menuVersion int64
	if err := db.WithContext(ctx).Raw(`SELECT menu.code, menu.menu_type, menu.parent_id::text, platform.menu_version FROM permission_menu menu JOIN permission_auth_platform platform ON platform.id=menu.platform_id WHERE menu.code='user:account:detail'`).Row().Scan(&code, &menuType, &parentID, &menuVersion); err != nil {
		t.Fatal(err)
	}
	if code != "user:account:detail" || menuType != "action" || parentID != "10" || menuVersion != 2 {
		t.Fatalf("detail action code=%q type=%q parent=%q menuVersion=%d", code, menuType, parentID, menuVersion)
	}
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("rerun migration: %v", err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT menu_version FROM permission_auth_platform WHERE id=1`).Scan(&menuVersion).Error; err != nil {
		t.Fatal(err)
	}
	if menuVersion != 2 {
		t.Fatalf("idempotent rerun changed menu version to %d", menuVersion)
	}
}

func TestUserEmailChangeLogMigrationRejectsUnrecoverableHistory(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_user_email_change_log_unknown")
	for _, statement := range []string{
		`CREATE TABLE permission_auth_platform (id BIGINT PRIMARY KEY, code VARCHAR(49) NOT NULL, deleted_at TIMESTAMPTZ, menu_version BIGINT NOT NULL DEFAULT 1, updated_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		`CREATE TABLE permission_menu (id BIGSERIAL PRIMARY KEY, platform_id BIGINT NOT NULL, parent_id BIGINT, menu_type VARCHAR(16) NOT NULL, code VARCHAR(128) NOT NULL, name VARCHAR(128) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		`CREATE TABLE user_account (id BIGINT PRIMARY KEY, username VARCHAR(64) NOT NULL)`,
		`CREATE TABLE message_mail_log (id BIGINT PRIMARY KEY, user_id BIGINT, platform_id BIGINT NOT NULL, scene VARCHAR(32) NOT NULL, to_email VARCHAR(254) NOT NULL, status SMALLINT NOT NULL, created_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE user_email_change_log (id BIGINT PRIMARY KEY, user_id BIGINT NOT NULL, platform_id BIGINT NOT NULL, action VARCHAR(16) NOT NULL CHECK (action IN ('bind','change')), old_email_hint VARCHAR(128) NOT NULL DEFAULT '', old_email_hmac VARCHAR(128) NOT NULL DEFAULT '', new_email_hint VARCHAR(128) NOT NULL DEFAULT '', new_email_hmac VARCHAR(128) NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL)`,
		`INSERT INTO permission_auth_platform(id, code) VALUES (1, 'admin')`,
		`INSERT INTO user_email_change_log(id, user_id, platform_id, action, created_at, updated_at) VALUES (1, 99, 1, 'change', now(), now())`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("prepare unknown fixture: %v", err)
		}
	}
	err := db.WithContext(ctx).Exec(readUserEmailChangeLogMigration(t)).Error
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unrecoverable") {
		t.Fatalf("migration error=%v", err)
	}
	var dataType string
	if err := db.WithContext(ctx).Raw(`SELECT data_type FROM information_schema.columns WHERE table_name='user_email_change_log' AND column_name='action'`).Scan(&dataType).Error; err != nil {
		t.Fatal(err)
	}
	if dataType != "character varying" {
		t.Fatalf("failed migration mutated action type to %q", dataType)
	}
}

func readUserEmailChangeLogMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-09-29-user-email-change-log.sql"))
	if err != nil {
		t.Fatalf("read email change log migration: %v", err)
	}
	return string(data)
}
