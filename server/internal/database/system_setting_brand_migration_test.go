package database_test

import (
	"os"
	"path/filepath"
	"testing"

	"admin/server/internal/database/testschema"
)

func TestSystemSettingBrandMigrationIsIdempotentAndPreservesValues(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_system_setting_brand")
	createSystemSettingMigrationFixture(t, db, ctx)
	if err := db.WithContext(ctx).Exec(readSystemSettingMigration(t)).Error; err != nil {
		t.Fatal(err)
	}

	scriptPath := filepath.Join(repoRoot(t), "docs", "database", "2026-09-15-system-setting-brand.sql")
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.WithContext(ctx).Exec(string(script)).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.WithContext(ctx).Exec(`UPDATE system_setting SET value='自定义标题' WHERE setting_key='app.brand.title_zh_cn'`).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.WithContext(ctx).Exec(string(script)).Error; err != nil {
		t.Fatal(err)
	}

	var count int64
	if err = db.WithContext(ctx).Raw(`SELECT count(*) FROM system_setting WHERE deleted_at IS NULL`).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Fatalf("settings=%d want=5", count)
	}
	var invalid int64
	if err = db.WithContext(ctx).Raw(`SELECT count(*) FROM system_setting WHERE setting_key LIKE 'app.brand.%' AND deleted_at IS NULL AND (value_type<>1 OR is_enabled<>1 OR is_builtin<>1)`).Scan(&invalid).Error; err != nil {
		t.Fatal(err)
	}
	if invalid != 0 {
		t.Fatalf("invalid brand settings=%d", invalid)
	}
	var title string
	if err = db.WithContext(ctx).Raw(`SELECT value FROM system_setting WHERE setting_key='app.brand.title_zh_cn' AND deleted_at IS NULL`).Scan(&title).Error; err != nil {
		t.Fatal(err)
	}
	if title != "自定义标题" {
		t.Fatalf("title=%q", title)
	}
}
