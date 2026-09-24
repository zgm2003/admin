package database_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"admin/server/internal/database/testschema"
)

func TestUserLoginLogNumericMigrationConvertsBackfillsAndIsIdempotent(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_user_login_log_numeric")
	for _, statement := range []string{
		`CREATE TABLE user_account (id BIGINT PRIMARY KEY, username VARCHAR(64) NOT NULL, deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE user_login_log (
 id BIGINT PRIMARY KEY, user_id BIGINT, session_id BIGINT, platform_id BIGINT NOT NULL,
 login_account VARCHAR(254) NOT NULL, event_type VARCHAR(16) NOT NULL, login_type VARCHAR(32),
 is_success SMALLINT NOT NULL, reason_code VARCHAR(64) NOT NULL, client_ip VARCHAR(64) NOT NULL,
 user_agent VARCHAR(512) NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL)`,
		`INSERT INTO user_account (id, username) VALUES (1, 'user_abc')`,
		`INSERT INTO user_login_log VALUES (1, 1, 9, 1, '', 'logout', NULL, 1, 'success', '127.0.0.1', 'test', now(), now()), (2, 1, 10, 1, 'a@example.com', 'login', 'email', 1, 'success', '127.0.0.1', 'test', now(), now())`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("prepare fixture: %v", err)
		}
	}
	script := readUserLoginLogNumericMigration(t)
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("execute migration: %v", err)
	}
	var account, event, login string
	var session int
	if err := db.WithContext(ctx).Raw(`SELECT data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='user_login_log' AND column_name='account'`).Scan(&account).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='user_login_log' AND column_name='event_type'`).Scan(&event).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='user_login_log' AND column_name='login_type'`).Scan(&login).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='user_login_log' AND column_name='session_id'`).Scan(&session).Error; err != nil {
		t.Fatal(err)
	}
	if account != "character varying" || event != "smallint" || login != "smallint" || session != 0 {
		t.Fatalf("schema account=%q event=%q login=%q session=%d", account, event, login, session)
	}
	var logoutAccount string
	var eventType, loginType int
	if err := db.WithContext(ctx).Raw(`SELECT account, event_type, COALESCE(login_type, 0) FROM user_login_log WHERE id=1`).Row().Scan(&logoutAccount, &eventType, &loginType); err != nil {
		t.Fatal(err)
	}
	if logoutAccount != "user_abc" || eventType != 3 || loginType != 0 {
		t.Fatalf("logout row account=%q event=%d login=%d", logoutAccount, eventType, loginType)
	}
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("rerun migration: %v", err)
	}
}

func TestUserLoginLogNumericMigrationRejectsUnknownAndRollsBack(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_user_login_log_numeric_unknown")
	for _, statement := range []string{
		`CREATE TABLE user_account (id BIGINT PRIMARY KEY, username VARCHAR(64) NOT NULL, deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE user_login_log (id BIGINT PRIMARY KEY, user_id BIGINT, session_id BIGINT, platform_id BIGINT NOT NULL, login_account VARCHAR(254) NOT NULL, event_type VARCHAR(16) NOT NULL, login_type VARCHAR(32), is_success SMALLINT NOT NULL, reason_code VARCHAR(64) NOT NULL, client_ip VARCHAR(64) NOT NULL, user_agent VARCHAR(512) NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL)`,
		`INSERT INTO user_login_log VALUES (1, NULL, NULL, 1, 'x@example.com', 'unknown', 'email', 1, 'bad', '127.0.0.1', 'test', now(), now())`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("prepare unknown fixture: %v", err)
		}
	}
	err := db.WithContext(ctx).Exec(readUserLoginLogNumericMigration(t)).Error
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unknown event_type") {
		t.Fatalf("migration error=%v", err)
	}
	var dataType, value string
	if err := db.WithContext(ctx).Raw(`SELECT data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='user_login_log' AND column_name='event_type'`).Scan(&dataType).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT event_type FROM user_login_log WHERE id=1`).Scan(&value).Error; err != nil {
		t.Fatal(err)
	}
	if dataType != "character varying" || value != "unknown" {
		t.Fatalf("failed migration mutated schema/data: type=%q value=%q", dataType, value)
	}
}

func readUserLoginLogNumericMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-09-23-user-login-log-numeric.sql"))
	if err != nil {
		t.Fatalf("read user login log migration: %v", err)
	}
	return string(data)
}
