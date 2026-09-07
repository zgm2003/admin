package database_test

import (
	"os"
	"path/filepath"
	"testing"

	"admin/server/internal/database/testschema"
)

func TestAuthPlatformLoginTypesMigrationIsIdempotent(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_auth_platform_login_types")
	for _, statement := range []string{
		`CREATE TABLE auth_platform (id BIGINT PRIMARY KEY, code VARCHAR(49) NOT NULL, name VARCHAR(64) NOT NULL)`,
		`CREATE TABLE user_account (id BIGINT PRIMARY KEY, email VARCHAR(254) NOT NULL, deleted_at TIMESTAMPTZ)`,
		`INSERT INTO auth_platform (id, code, name) VALUES (1, 'admin', 'Admin')`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("prepare login types schema: %v", err)
		}
	}

	script := readAuthPlatformLoginTypesMigration(t)
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("execute login types migration: %v", err)
	}

	var loginTypes string
	if err := db.WithContext(ctx).Raw(`SELECT login_types::text FROM auth_platform WHERE id = 1`).Scan(&loginTypes).Error; err != nil {
		t.Fatalf("read login types: %v", err)
	}
	if loginTypes != `["email", "password"]` {
		t.Fatalf("admin login types = %q, want [\"email\", \"password\"]", loginTypes)
	}

	// The CHECK rejects invalid values, empty sets and over-long sets.
	for _, invalid := range []string{`'["sms"]'`, `'[]'`, `'["email","phone","password","sms"]'`} {
		err := db.WithContext(ctx).Exec(`UPDATE auth_platform SET login_types = ` + invalid + `::jsonb WHERE id = 1`).Error
		if err == nil {
			t.Fatalf("invalid login types %s was accepted", invalid)
		}
	}

	// The email partial unique index only constrains non-empty email addresses.
	if err := db.WithContext(ctx).Exec(`INSERT INTO user_account (id, email) VALUES (1, '')`).Error; err != nil {
		t.Fatalf("insert empty-email user: %v", err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO user_account (id, email) VALUES (2, '')`).Error; err != nil {
		t.Fatalf("insert second empty-email user: %v", err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO user_account (id, email) VALUES (3, 'dup@example.com')`).Error; err != nil {
		t.Fatalf("insert first dup user: %v", err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO user_account (id, email) VALUES (4, 'dup@example.com')`).Error; err == nil {
		t.Fatalf("duplicate email was accepted")
	}

	// Re-running the migration must be idempotent and preserve an edited value.
	if err := db.WithContext(ctx).Exec(`UPDATE auth_platform SET login_types = '["phone","password"]'::jsonb WHERE id = 1`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("second login types migration: %v", err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT login_types::text FROM auth_platform WHERE id = 1`).Scan(&loginTypes).Error; err != nil {
		t.Fatal(err)
	}
	if loginTypes != `["phone", "password"]` {
		t.Fatalf("edited login types were overwritten: %q", loginTypes)
	}

	// A partially applied legacy schema can contain an invalid value after its
	// CHECK was removed. Re-running the forward migration must repair it.
	if err := db.WithContext(ctx).Exec(`ALTER TABLE auth_platform DROP CONSTRAINT ck_auth_platform_login_types`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`UPDATE auth_platform SET login_types = '["email","email"]'::jsonb WHERE id = 1`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("migration did not repair invalid legacy login types: %v", err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT login_types::text FROM auth_platform WHERE id = 1`).Scan(&loginTypes).Error; err != nil {
		t.Fatal(err)
	}
	if loginTypes != `["email", "password"]` {
		t.Fatalf("repaired login types = %q", loginTypes)
	}
}

func readAuthPlatformLoginTypesMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-09-05-auth-platform-login-types.sql"))
	if err != nil {
		t.Fatalf("read login types migration: %v", err)
	}
	return string(data)
}
