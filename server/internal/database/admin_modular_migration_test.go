package database_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"admin/server/internal/database/testschema"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestAdminModularMigrationPreservesIDsAndIsIdempotent(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_admin_modular_migration")
	createLegacyAdminFixture(t, db, ctx)
	script := readAdminModularMigration(t)
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("execute admin modular migration: %v", err)
	}
	assertTableExists(t, db, ctx, "user_profile")
	assertTableExists(t, db, ctx, "user_session")
	assertTableExists(t, db, ctx, "user_login_log")
	assertTableMissing(t, db, ctx, "auth_session")
	assertTableExists(t, db, ctx, "system_operation_log")
	assertTableMissing(t, db, ctx, "audit_operation_log")
	assertColumnMissing(t, db, ctx, "system_operation_log", "platform")
	assertColumnExists(t, db, ctx, "system_operation_log", "platform_id")
	assertCount(t, db, ctx, "user_account", 2)
	assertCount(t, db, ctx, "user_session", 1)
	assertIDExists(t, db, ctx, "user_session", 41)
	assertIDExists(t, db, ctx, "system_operation_log", 51)
	assertForeignKey(t, db, ctx, "fk_user_session_platform")

	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("second migration execution: %v", err)
	}
	assertCount(t, db, ctx, "user_session", 1)
	assertIDExists(t, db, ctx, "user_session", 41)
}

func TestUserProfileAvatarMigrationAddsObjectKeyColumn(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_user_profile_avatar_migration")
	createLegacyAdminFixture(t, db, ctx)
	if err := db.WithContext(ctx).Exec(readAdminModularMigration(t)).Error; err != nil {
		t.Fatal(err)
	}
	script := readUserProfileAvatarMigration(t)
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatal(err)
	}
	assertColumnExists(t, db, ctx, "user_profile", "avatar")
	if err := db.WithContext(ctx).Exec(`INSERT INTO user_profile (user_id, avatar) VALUES (11, 'avatar/profile.png')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatal(err)
	}
}

func TestAdminModularMigrationRollsBackUnknownSessionPlatform(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_admin_modular_rollback")
	createLegacyAdminFixture(t, db, ctx)
	if err := db.WithContext(ctx).Exec(`UPDATE auth_session SET platform = 'missing' WHERE id = 41`).Error; err != nil {
		t.Fatal(err)
	}
	err := db.WithContext(ctx).Exec(readAdminModularMigration(t)).Error
	if err == nil {
		t.Fatal("expected unknown session platform to fail")
	}
	assertTableExists(t, db, ctx, "auth_session")
	assertTableMissing(t, db, ctx, "user_session")
	var platform string
	if err := db.WithContext(ctx).Raw(`SELECT platform FROM auth_session WHERE id = 41`).Scan(&platform).Error; err != nil {
		t.Fatal(err)
	}
	if platform != "missing" {
		t.Fatalf("platform after rollback = %q", platform)
	}
}

func TestAdminModularMigrationRejectsOldAndNewObjectsTogether(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_admin_modular_conflict")
	createLegacyAdminFixture(t, db, ctx)
	if err := db.WithContext(ctx).Exec(`CREATE TABLE user_session (id BIGINT PRIMARY KEY)`).Error; err != nil {
		t.Fatal(err)
	}
	err := db.WithContext(ctx).Exec(readAdminModularMigration(t)).Error
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "both") {
		t.Fatalf("expected old/new object conflict error, got %v", err)
	}
	assertTableExists(t, db, ctx, "auth_session")
	assertTableExists(t, db, ctx, "user_session")
}

func mustPostgresDSN(t *testing.T) string {
	t.Helper()
	_ = godotenv.Load(filepath.Join(repoRoot(t), "server", ".env"))
	if dsn := os.Getenv("POSTGRES_DSN"); dsn != "" {
		return dsn
	}
	return "host=127.0.0.1 user=postgres password=postgres dbname=admin port=5432 sslmode=disable"
}

func readAdminModularMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-08-29-admin-modular-architecture.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	return string(data)
}

func readUserProfileAvatarMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-08-30-user-profile-avatar.sql"))
	if err != nil {
		t.Fatalf("read avatar migration: %v", err)
	}
	return string(data)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
}

func createLegacyAdminFixture(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	statements := []string{
		`CREATE TABLE user_account (id BIGINT PRIMARY KEY, username VARCHAR(64) NOT NULL, email VARCHAR(254) NOT NULL, phone VARCHAR(32), password_hash VARCHAR(255) NOT NULL, is_enabled SMALLINT NOT NULL DEFAULT 1, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, deleted_at TIMESTAMPTZ NULL)`,
		`CREATE TABLE auth_platform (id BIGINT PRIMARY KEY, code VARCHAR(49) NOT NULL, name VARCHAR(64) NOT NULL, policy_version BIGINT NOT NULL DEFAULT 1, access_ttl_seconds INTEGER NOT NULL DEFAULT 900, refresh_ttl_seconds INTEGER NOT NULL DEFAULT 2592000, session_cache_ttl_seconds INTEGER NOT NULL DEFAULT 900, access_cache_ttl_seconds INTEGER NOT NULL DEFAULT 900, bind_device SMALLINT NOT NULL DEFAULT 0, bind_ip SMALLINT NOT NULL DEFAULT 0, max_sessions SMALLINT NOT NULL DEFAULT 10, allow_register SMALLINT NOT NULL DEFAULT 0, is_enabled SMALLINT NOT NULL DEFAULT 1, is_builtin SMALLINT NOT NULL DEFAULT 1, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, deleted_at TIMESTAMPTZ NULL)`,
		`CREATE TABLE permission_role (id BIGINT PRIMARY KEY, code VARCHAR(64) NOT NULL, name VARCHAR(64) NOT NULL, is_default SMALLINT NOT NULL DEFAULT 0, is_enabled SMALLINT NOT NULL DEFAULT 1, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, deleted_at TIMESTAMPTZ NULL)`,
		`CREATE TABLE permission_menu (id BIGINT PRIMARY KEY, platform_id BIGINT NOT NULL, parent_id BIGINT NULL, menu_type VARCHAR(16) NOT NULL, name VARCHAR(128) NOT NULL, code VARCHAR(128) NOT NULL, path VARCHAR(255), sort_order INTEGER NOT NULL DEFAULT 0, is_enabled SMALLINT NOT NULL DEFAULT 1, is_hidden SMALLINT NOT NULL DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, deleted_at TIMESTAMPTZ NULL)`,
		`CREATE TABLE permission_user_role (id BIGINT PRIMARY KEY, user_id BIGINT NOT NULL, role_id BIGINT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, deleted_at TIMESTAMPTZ NULL)`,
		`CREATE TABLE permission_role_menu (id BIGINT PRIMARY KEY, role_id BIGINT NOT NULL, menu_id BIGINT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, deleted_at TIMESTAMPTZ NULL)`,
		`CREATE TABLE permission_access_version (user_id BIGINT PRIMARY KEY, version BIGINT NOT NULL DEFAULT 1, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE auth_session (id BIGINT PRIMARY KEY, user_id BIGINT NOT NULL, platform VARCHAR(49) NOT NULL, device_id VARCHAR(36) NOT NULL, refresh_token_hash CHAR(64) NOT NULL, version BIGINT NOT NULL DEFAULT 1, client_ip VARCHAR(64) NOT NULL, user_agent VARCHAR(512) NOT NULL, refresh_expires_at TIMESTAMPTZ NOT NULL, revoked_at TIMESTAMPTZ NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE audit_operation_log (id BIGINT PRIMARY KEY, event_id VARCHAR(64) NOT NULL, request_id VARCHAR(128) NOT NULL, user_id BIGINT NULL, session_id BIGINT NULL, platform VARCHAR(49) NULL, method VARCHAR(10) NOT NULL, route VARCHAR(255) NOT NULL, module VARCHAR(64) NOT NULL, action VARCHAR(128) NOT NULL, client_ip VARCHAR(64) NOT NULL, user_agent VARCHAR(512) NOT NULL, status_code INTEGER NOT NULL, is_success SMALLINT NOT NULL, latency_ms BIGINT NOT NULL, request_data JSONB, response_data JSONB, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE foundation_task (id VARCHAR(32) PRIMARY KEY, message VARCHAR(200) NOT NULL, status VARCHAR(16) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`INSERT INTO auth_platform (id, code, name) VALUES (1, 'admin', 'Admin'), (2, 'canvas', 'Canvas')`,
		`INSERT INTO user_account (id, username, email, password_hash) VALUES (11, 'admin', 'admin@example.com', 'hash'), (12, 'canvas', 'canvas@example.com', 'hash')`,
		`INSERT INTO permission_role (id, code, name, is_default) VALUES (21, 'admin', 'Admin', 1)`,
		`INSERT INTO permission_menu (id, platform_id, menu_type, name, code) VALUES (31, 1, 'page', 'Users', 'users')`,
		`INSERT INTO permission_user_role (id, user_id, role_id) VALUES (61, 11, 21)`,
		`INSERT INTO permission_role_menu (id, role_id, menu_id) VALUES (71, 21, 31)`,
		`INSERT INTO auth_session (id, user_id, platform, device_id, refresh_token_hash, client_ip, user_agent, refresh_expires_at) VALUES (41, 11, 'admin', 'device', repeat('a', 64), '127.0.0.1', 'agent', CURRENT_TIMESTAMP + interval '1 day')`,
		`INSERT INTO audit_operation_log (id, event_id, request_id, user_id, session_id, platform, method, route, module, action, client_ip, user_agent, status_code, is_success, latency_ms) VALUES (51, 'event-51', 'request-51', 11, 41, 'admin', 'POST', '/admin', 'user', 'user.update', '127.0.0.1', 'agent', 200, 1, 10)`,
		`INSERT INTO foundation_task (id, message, status) VALUES ('demo', 'demo', 'pending')`,
	}
	for _, statement := range statements {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("legacy fixture: %v\n%s", err, statement)
		}
	}
}

func assertTableExists(t *testing.T, db *gorm.DB, ctx context.Context, name string) {
	assertRelation(t, db, ctx, name, true)
}
func assertTableMissing(t *testing.T, db *gorm.DB, ctx context.Context, name string) {
	assertRelation(t, db, ctx, name, false)
}
func assertRelation(t *testing.T, db *gorm.DB, ctx context.Context, name string, want bool) {
	t.Helper()
	var exists bool
	if err := db.WithContext(ctx).Raw(`SELECT to_regclass(current_schema() || '.' || ?) IS NOT NULL`, name).Scan(&exists).Error; err != nil {
		t.Fatal(err)
	}
	if exists != want {
		t.Fatalf("relation %s exists=%v want=%v", name, exists, want)
	}
}
func assertColumnExists(t *testing.T, db *gorm.DB, ctx context.Context, table, column string) {
	assertColumn(t, db, ctx, table, column, true)
}
func assertColumnMissing(t *testing.T, db *gorm.DB, ctx context.Context, table, column string) {
	assertColumn(t, db, ctx, table, column, false)
}
func assertColumn(t *testing.T, db *gorm.DB, ctx context.Context, table, column string, want bool) {
	t.Helper()
	var exists bool
	if err := db.WithContext(ctx).Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=? AND column_name=?)`, table, column).Scan(&exists).Error; err != nil {
		t.Fatal(err)
	}
	if exists != want {
		t.Fatalf("column %s.%s exists=%v want=%v", table, column, exists, want)
	}
}
func assertCount(t *testing.T, db *gorm.DB, ctx context.Context, table string, want int64) {
	var got int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM ` + table).Scan(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s count=%d want=%d", table, got, want)
	}
}
func assertIDExists(t *testing.T, db *gorm.DB, ctx context.Context, table string, id int64) {
	var got int64
	if err := db.WithContext(ctx).Raw(`SELECT id FROM `+table+` WHERE id=?`, id).Scan(&got).Error; err != nil || got != id {
		t.Fatalf("%s id %d missing: %v", table, id, err)
	}
}
func assertForeignKey(t *testing.T, db *gorm.DB, ctx context.Context, name string) {
	var exists bool
	if err := db.WithContext(ctx).Raw(`SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname=? AND contype='f' AND connamespace=current_schema()::regnamespace)`, name).Scan(&exists).Error; err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatalf("foreign key %s missing", name)
	}
}
