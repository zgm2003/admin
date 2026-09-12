package database_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"admin/server/internal/database/testschema"
	"gorm.io/gorm"
)

func TestSystemSettingCaptchaMigrationIsIdempotentAndSeedsMenu(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_system_setting_captcha")
	createSystemSettingMigrationFixture(t, db, ctx)

	script := readSystemSettingMigration(t)
	for run := 0; run < 2; run++ {
		if err := db.WithContext(ctx).Exec(script).Error; err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
	}

	var settings int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM system_setting WHERE deleted_at IS NULL`).Scan(&settings).Error; err != nil {
		t.Fatal(err)
	}
	if settings != 2 {
		t.Fatalf("system settings = %d, want 2", settings)
	}

	var menuCount int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_menu WHERE platform_id=1 AND code LIKE 'system:setting:%' AND deleted_at IS NULL`).Scan(&menuCount).Error; err != nil {
		t.Fatal(err)
	}
	if menuCount != 7 {
		t.Fatalf("system setting menu rows = %d, want 7", menuCount)
	}

	var versions []struct {
		ID      int64
		Version int64
	}
	if err := db.WithContext(ctx).Raw(`SELECT id, menu_version AS version FROM permission_auth_platform ORDER BY id`).Scan(&versions).Error; err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 || versions[0].ID != 1 || versions[0].Version != 2 || versions[1].ID != 2 || versions[1].Version != 1 {
		t.Fatalf("menu versions = %+v", versions)
	}
}

func createSystemSettingMigrationFixture(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE permission_auth_platform (
  id BIGSERIAL PRIMARY KEY,
  menu_version BIGINT NOT NULL DEFAULT 1,
  is_enabled SMALLINT NOT NULL DEFAULT 1,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMPTZ
);
CREATE TABLE permission_menu (
  id BIGSERIAL PRIMARY KEY,
  parent_id BIGINT,
  platform_id BIGINT NOT NULL,
  menu_type VARCHAR(16) NOT NULL,
  code VARCHAR(128) NOT NULL,
  i18n_key VARCHAR(128),
  path VARCHAR(255),
  component_path VARCHAR(255),
  icon VARCHAR(64),
  sort_order INTEGER NOT NULL DEFAULT 0,
  is_enabled SMALLINT NOT NULL DEFAULT 1,
  is_hidden SMALLINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  name VARCHAR(128) NOT NULL,
  remark VARCHAR(255),
  deleted_at TIMESTAMPTZ
);`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`
INSERT INTO permission_auth_platform(id, menu_version, is_enabled) VALUES (1, 1, 1), (2, 1, 1);
INSERT INTO permission_menu(platform_id, menu_type, code, i18n_key, sort_order, name)
VALUES (1, 'directory', 'system', 'navigation.system', 10, '系统');`).Error; err != nil {
		t.Fatal(err)
	}
}

func readSystemSettingMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-09-12-system-setting-captcha.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
