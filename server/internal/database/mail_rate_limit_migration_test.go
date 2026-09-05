package database_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"admin/server/internal/database/testschema"
	"gorm.io/gorm"
)

func TestMailRateLimitMigrationCreatesFixedPoliciesAndHiddenAction(t *testing.T) {
	db, ctx := openMailRateLimitSchema(t)
	seedMailRateLimitFixture(t, db, ctx)
	script := readMailRateLimitMigration(t)

	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("execute mail rate limit migration: %v", err)
	}

	assertMailRateLimitPolicies(t, db, ctx)
	assertMailRateLimitConstraints(t, db, ctx)
	assertMailRateLimitTimestamps(t, db, ctx)
	assertMailRateLimitActionShape(t, db, ctx)
}

func TestMailRateLimitMigrationIsIdempotentAndPreservesEditedValues(t *testing.T) {
	db, ctx := openMailRateLimitSchema(t)
	seedMailRateLimitFixture(t, db, ctx)
	script := readMailRateLimitMigration(t)

	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("execute mail rate limit migration: %v", err)
	}
	if err := db.WithContext(ctx).Exec(
		`UPDATE message_mail_rate_limit_policy SET limit_count = 2, window_seconds = 120, revision = revision + 1 WHERE policy_key = 'business_email_minute'`,
	).Error; err != nil {
		t.Fatalf("edit policy: %v", err)
	}
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("re-execute mail rate limit migration: %v", err)
	}

	var limit int
	if err := db.WithContext(ctx).Raw(
		`SELECT limit_count FROM message_mail_rate_limit_policy WHERE policy_key = 'business_email_minute'`,
	).Scan(&limit).Error; err != nil {
		t.Fatalf("read edited policy: %v", err)
	}
	if limit != 2 {
		t.Fatalf("edited limit_count = %d, want 2 (migration must not overwrite admin edits)", limit)
	}
}

func TestMailRateLimitMigrationScopesHiddenActionByPlatform(t *testing.T) {
	db, ctx := openMailRateLimitSchema(t)
	seedMailRateLimitFixture(t, db, ctx)
	if err := db.WithContext(ctx).Exec(`
		INSERT INTO auth_platform (id, code, name, is_enabled) VALUES (2, 'canvas', 'Canvas', 1);
		INSERT INTO permission_menu
			(id, platform_id, parent_id, menu_type, name, code, is_enabled, is_hidden)
		VALUES (200, 2, NULL, 'action', 'Canvas action', 'message:mail:rate-limit:update', 1, 1);
	`).Error; err != nil {
		t.Fatalf("seed cross-platform action: %v", err)
	}

	if err := db.WithContext(ctx).Exec(readMailRateLimitMigration(t)).Error; err != nil {
		t.Fatalf("execute migration with same code on another platform: %v", err)
	}

	var adminCount, canvasCount int64
	if err := db.WithContext(ctx).Table("permission_menu").
		Where("platform_id = ? AND code = ? AND deleted_at IS NULL", 1, "message:mail:rate-limit:update").Count(&adminCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Table("permission_menu").
		Where("platform_id = ? AND code = ? AND deleted_at IS NULL", 2, "message:mail:rate-limit:update").Count(&canvasCount).Error; err != nil {
		t.Fatal(err)
	}
	if adminCount != 1 || canvasCount != 1 {
		t.Fatalf("platform-scoped action counts = admin:%d canvas:%d, want 1/1", adminCount, canvasCount)
	}
}

func openMailRateLimitSchema(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_mail_rate_limit")
	for _, statement := range []string{
		`CREATE TABLE auth_platform (
			id BIGINT PRIMARY KEY,
			code VARCHAR(49) NOT NULL,
			name VARCHAR(64) NOT NULL,
			is_enabled SMALLINT NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMPTZ NULL
		)`,
		`CREATE TABLE permission_menu (
			id BIGSERIAL PRIMARY KEY,
			platform_id BIGINT NOT NULL,
			parent_id BIGINT NULL,
			menu_type VARCHAR(16) NOT NULL,
			name VARCHAR(128) NOT NULL,
			code VARCHAR(128) NOT NULL,
			i18n_key VARCHAR(128) NULL,
			path VARCHAR(255) NULL,
			component_path VARCHAR(255) NULL,
			icon VARCHAR(128) NULL,
			sort_order INTEGER NOT NULL DEFAULT 0,
			is_enabled SMALLINT NOT NULL DEFAULT 1,
			is_hidden SMALLINT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMPTZ NULL
		)`,
		`CREATE TABLE permission_access_version (
			user_id BIGINT PRIMARY KEY,
			version BIGINT NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("prepare mail rate limit schema: %v", err)
		}
	}
	return db, ctx
}

func seedMailRateLimitFixture(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	statements := []string{
		`INSERT INTO auth_platform (id, code, name, is_enabled) VALUES (1, 'admin', 'Admin', 1)`,
		`INSERT INTO permission_menu (id, platform_id, parent_id, menu_type, name, code, i18n_key, sort_order, is_enabled, is_hidden)
		 VALUES (100, 1, NULL, 'directory', '消息服务', 'message', 'navigation.message', 80, 1, 0)`,
		`INSERT INTO permission_menu (id, platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, icon, sort_order, is_enabled, is_hidden)
		 VALUES (101, 1, 100, 'page', '邮件服务', 'message:mail:view', 'navigation.mail', '/message/mail', 'message/mail', 'lucide:mail', 10, 1, 0)`,
		`INSERT INTO permission_access_version (user_id, version) VALUES (1, 1)`,
	}
	for _, statement := range statements {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("seed mail rate limit fixture: %v\n%s", err, statement)
		}
	}
}

func assertMailRateLimitPolicies(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	var count int
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM message_mail_rate_limit_policy`).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 7 {
		t.Fatalf("policy row count = %d, want 7", count)
	}

	expected := map[string]struct {
		limit  int
		window int
	}{
		"business_email_minute": {1, 60},
		"business_email_10m":    {5, 600},
		"business_ip_minute":    {10, 60},
		"business_scene_minute": {30, 60},
		"admin_test_user_10m":   {5, 600},
		"admin_test_ip_minute":  {10, 60},
		"admin_test_email_10m":  {3, 600},
	}
	var rows []struct {
		Key     string `gorm:"column:policy_key"`
		Limit   int    `gorm:"column:limit_count"`
		Window  int    `gorm:"column:window_seconds"`
		Version int64  `gorm:"column:revision"`
	}
	if err := db.WithContext(ctx).Table("message_mail_rate_limit_policy").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]int)
	for _, row := range rows {
		seen[row.Key]++
		want, ok := expected[row.Key]
		if !ok {
			t.Fatalf("unexpected policy key %q", row.Key)
		}
		if row.Limit != want.limit || row.Window != want.window {
			t.Fatalf("policy %q = limit %d window %d, want %d/%d", row.Key, row.Limit, row.Window, want.limit, want.window)
		}
		if row.Version < 1 {
			t.Fatalf("policy %q revision = %d, want >= 1", row.Key, row.Version)
		}
	}
	if len(seen) != 7 {
		t.Fatalf("distinct policy keys = %d, want 7", len(seen))
	}
	for key, n := range seen {
		if n != 1 {
			t.Fatalf("policy key %q appears %d times, want once", key, n)
		}
	}
}

func assertMailRateLimitConstraints(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	reject := []string{
		`UPDATE message_mail_rate_limit_policy SET limit_count = 0 WHERE policy_key = 'business_email_minute'`,
		`UPDATE message_mail_rate_limit_policy SET window_seconds = 0 WHERE policy_key = 'business_email_minute'`,
		`UPDATE message_mail_rate_limit_policy SET limit_count = 100001 WHERE policy_key = 'business_email_minute'`,
		`UPDATE message_mail_rate_limit_policy SET mode = 'business', dimension = 'ip' WHERE policy_key = 'business_email_minute'`,
		`INSERT INTO message_mail_rate_limit_policy (policy_key, mode, dimension, limit_count, window_seconds, revision)
		 VALUES ('unknown_key', 'business', 'platform_scene_email', 1, 60, 1)`,
	}
	for _, statement := range reject {
		if err := db.WithContext(ctx).Exec(statement).Error; err == nil {
			t.Fatalf("constraint accepted invalid statement: %s", statement)
		}
	}
}

func assertMailRateLimitTimestamps(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	var row struct {
		DataType   string `gorm:"column:data_type"`
		IsNullable string `gorm:"column:is_nullable"`
	}
	err := db.WithContext(ctx).Raw(`
		SELECT data_type, is_nullable
		FROM information_schema.columns
		WHERE table_name = 'message_mail_rate_limit_policy' AND column_name = 'created_at'`,
	).Scan(&row).Error
	if err != nil {
		t.Fatal(err)
	}
	if row.DataType != "timestamp with time zone" || row.IsNullable != "NO" {
		t.Fatalf("created_at type/nullable = %s/%s, want timestamptz NOT NULL", row.DataType, row.IsNullable)
	}
}

func assertMailRateLimitActionShape(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	var row struct {
		MenuType      string  `gorm:"column:menu_type"`
		IsHidden      int     `gorm:"column:is_hidden"`
		Path          *string `gorm:"column:path"`
		ComponentPath *string `gorm:"column:component_path"`
		I18nKey       *string `gorm:"column:i18n_key"`
		Icon          *string `gorm:"column:icon"`
	}
	if err := db.WithContext(ctx).Raw(`
		SELECT menu_type, is_hidden, path, component_path, i18n_key, icon
		FROM permission_menu WHERE code = 'message:mail:rate-limit:update' AND deleted_at IS NULL`,
	).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.MenuType != "action" || row.IsHidden != 1 {
		t.Fatalf("action shape = type %q hidden %d, want action/1", row.MenuType, row.IsHidden)
	}
	if row.Path != nil || row.ComponentPath != nil || row.I18nKey != nil || row.Icon != nil {
		t.Fatalf("action must not carry render fields: path=%v component=%v i18n=%v icon=%v",
			row.Path, row.ComponentPath, row.I18nKey, row.Icon)
	}
}

func readMailRateLimitMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-09-04-mail-rate-limit-policies.sql"))
	if err != nil {
		t.Fatalf("read mail rate limit migration: %v", err)
	}
	if strings.TrimSpace(string(data)) == "" {
		t.Fatal("mail rate limit migration is empty")
	}
	return string(data)
}
