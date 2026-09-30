package database_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"admin/server/internal/database/testschema"
)

func TestUserPhoneChangeLogMigrationConvertsEmptyHistoryAndIsIdempotent(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_user_phone_change_log_numeric")
	for _, statement := range []string{
		`CREATE TABLE permission_auth_platform (id BIGINT PRIMARY KEY, code VARCHAR(49) NOT NULL, deleted_at TIMESTAMPTZ, menu_version BIGINT NOT NULL DEFAULT 1, updated_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		`CREATE TABLE permission_menu (id BIGSERIAL PRIMARY KEY, platform_id BIGINT NOT NULL, parent_id BIGINT, menu_type VARCHAR(16) NOT NULL, code VARCHAR(128) NOT NULL, i18n_key VARCHAR(128), path VARCHAR(255), component_path VARCHAR(255), icon VARCHAR(128), sort_order INT NOT NULL DEFAULT 0, is_enabled SMALLINT NOT NULL DEFAULT 1, is_hidden SMALLINT NOT NULL DEFAULT 0, name VARCHAR(128) NOT NULL, remark VARCHAR(512), deleted_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		`CREATE TABLE user_phone_change_log (id BIGINT PRIMARY KEY, user_id BIGINT NOT NULL, platform_id BIGINT NOT NULL, action VARCHAR(16) NOT NULL CHECK (action IN ('bind','change')), old_phone_hint VARCHAR(32) NOT NULL DEFAULT '', old_phone_hmac VARCHAR(128) NOT NULL DEFAULT '', new_phone_hint VARCHAR(32) NOT NULL DEFAULT '', new_phone_hmac VARCHAR(128) NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL)`,
		`INSERT INTO permission_auth_platform(id, code) VALUES (1, 'admin')`,
		`INSERT INTO permission_menu(id, platform_id, menu_type, code, name) VALUES (10, 1, 'page', 'user:account:view', 'Users')`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("prepare fixture: %v", err)
		}
	}
	script := readUserPhoneChangeLogMigration(t)
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("execute migration: %v", err)
	}
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("rerun migration: %v", err)
	}
	var actionType string
	if err := db.WithContext(ctx).Raw(`SELECT data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='user_phone_change_log' AND column_name='action'`).Scan(&actionType).Error; err != nil {
		t.Fatal(err)
	}
	if actionType != "smallint" {
		t.Fatalf("action type=%q", actionType)
	}
	var legacyColumns int
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='user_phone_change_log' AND column_name IN ('old_phone_hint', 'old_phone_hmac', 'new_phone_hint', 'new_phone_hmac')`).Scan(&legacyColumns).Error; err != nil {
		t.Fatal(err)
	}
	if legacyColumns != 0 {
		t.Fatalf("legacy phone columns remain: %d", legacyColumns)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO user_phone_change_log(id, user_id, platform_id, action, old_phone, new_phone, created_at, updated_at) VALUES (1, 1, 1, 2, NULL, '+8613800000000', now(), now())`).Error; err != nil {
		t.Fatalf("valid phone history rejected: %v", err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO user_phone_change_log(id, user_id, platform_id, action, old_phone, new_phone, created_at, updated_at) VALUES (2, 1, 1, 2, NULL, '13800000000', now(), now())`).Error; err == nil {
		t.Fatal("invalid phone history accepted")
	}
}

func TestUserPhoneChangeLogMigrationRejectsExistingPlaintextGap(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_user_phone_change_log_unknown")
	for _, statement := range []string{
		`CREATE TABLE permission_auth_platform (id BIGINT PRIMARY KEY, code VARCHAR(49) NOT NULL, deleted_at TIMESTAMPTZ, menu_version BIGINT NOT NULL DEFAULT 1, updated_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		`CREATE TABLE permission_menu (id BIGSERIAL PRIMARY KEY, platform_id BIGINT NOT NULL, parent_id BIGINT, menu_type VARCHAR(16) NOT NULL, code VARCHAR(128) NOT NULL, i18n_key VARCHAR(128), path VARCHAR(255), component_path VARCHAR(255), icon VARCHAR(128), sort_order INT NOT NULL DEFAULT 0, is_enabled SMALLINT NOT NULL DEFAULT 1, is_hidden SMALLINT NOT NULL DEFAULT 0, name VARCHAR(128) NOT NULL, remark VARCHAR(512), deleted_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		`CREATE TABLE user_phone_change_log (id BIGINT PRIMARY KEY, user_id BIGINT NOT NULL, platform_id BIGINT NOT NULL, action VARCHAR(16) NOT NULL CHECK (action IN ('bind','change')), old_phone_hint VARCHAR(32) NOT NULL DEFAULT '', old_phone_hmac VARCHAR(128) NOT NULL DEFAULT '', new_phone_hint VARCHAR(32) NOT NULL DEFAULT '', new_phone_hmac VARCHAR(128) NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL)`,
		`INSERT INTO permission_auth_platform(id, code) VALUES (1, 'admin')`,
		`INSERT INTO user_phone_change_log(id, user_id, platform_id, action, created_at, updated_at) VALUES (1, 1, 1, 'bind', now(), now())`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("prepare fixture: %v", err)
		}
	}
	err := db.WithContext(ctx).Exec(readUserPhoneChangeLogMigration(t)).Error
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unrecoverable") {
		t.Fatalf("migration error=%v", err)
	}
}

func readUserPhoneChangeLogMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-09-29-user-phone-change-log.sql"))
	if err != nil {
		t.Fatalf("read phone change log migration: %v", err)
	}
	return string(data)
}
