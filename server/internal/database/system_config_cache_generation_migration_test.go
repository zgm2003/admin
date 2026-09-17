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

func TestSystemConfigCacheGenerationMigrationIsIdempotentAndScoped(t *testing.T) {
	db, ctx := openSystemConfigCacheGenerationSchema(t)
	seedSystemConfigCacheGenerationFixture(t, db, ctx)
	script := readSystemConfigCacheGenerationMigration(t)

	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("execute migration: %v", err)
	}

	assertTableExists(t, db, ctx, "system_config_cache_generation")
	assertTableExists(t, db, ctx, "system_config_cache_outbox")
	assertForeignKey(t, db, ctx, "fk_system_config_cache_outbox_generation")
	assertCacheGenerationConstraint(t, db, ctx, "ck_system_config_cache_generation_namespace")
	assertCacheGenerationConstraint(t, db, ctx, "ck_system_config_cache_generation_value")
	assertCacheGenerationConstraint(t, db, ctx, "ck_system_config_cache_outbox_namespace")
	assertCacheGenerationConstraint(t, db, ctx, "ck_system_config_cache_outbox_lock")
	assertCacheGenerationConstraint(t, db, ctx, "ck_system_config_cache_outbox_published_lock")
	assertCacheGenerationIndex(t, db, ctx, "ix_system_config_cache_outbox_pending")

	var generation int64
	if err := db.WithContext(ctx).Raw(`SELECT generation FROM system_config_cache_generation WHERE namespace='system.setting' AND scope_key='global'`).Scan(&generation).Error; err != nil {
		t.Fatal(err)
	}
	if generation != 1 {
		t.Fatalf("generation=%d want=1", generation)
	}
	var pending int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM system_config_cache_outbox WHERE namespace='system.setting' AND scope_key='global' AND published_at IS NULL`).Scan(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if pending != 1 {
		t.Fatalf("pending outbox=%d want=1", pending)
	}

	assertCacheGenerationMenuShape(t, db, ctx)
	assertCount(t, db, ctx, "permission_menu", 3)

	adminVersion, canvasVersion := readCacheGenerationMenuVersions(t, db, ctx)
	if adminVersion != 2 || canvasVersion != 1 {
		t.Fatalf("menu versions admin=%d canvas=%d want 2/1", adminVersion, canvasVersion)
	}

	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("re-execute migration: %v", err)
	}
	assertCount(t, db, ctx, "system_config_cache_generation", 1)
	assertCount(t, db, ctx, "system_config_cache_outbox", 1)
	assertCount(t, db, ctx, "permission_menu", 3)
	assertCacheGenerationMenuShape(t, db, ctx)
	adminVersion, canvasVersion = readCacheGenerationMenuVersions(t, db, ctx)
	if adminVersion != 2 || canvasVersion != 1 {
		t.Fatalf("menu versions after re-run admin=%d canvas=%d want 2/1", adminVersion, canvasVersion)
	}
}

func TestSystemConfigCacheGenerationMigrationRollsBackOnMenuConflicts(t *testing.T) {
	cases := []struct {
		name        string
		conflict    string
		wantMessage string
	}{
		{
			"code conflict",
			`INSERT INTO permission_menu (id, platform_id, parent_id, menu_type, name, code, is_enabled, is_hidden) VALUES (11, 1, 10, 'action', '占位动作', 'system:cacheGeneration:view', 1, 1)`,
			"system:cacheGeneration:view",
		},
		{
			"path conflict",
			`INSERT INTO permission_menu (id, platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, is_enabled, is_hidden) VALUES (11, 1, 10, 'page', '占用页面', 'system:other:view', 'navigation.systemOther', '/system/cacheGeneration', 'system/other', 1, 0)`,
			"path /system/cacheGeneration is occupied",
		},
		{
			"component path conflict",
			`INSERT INTO permission_menu (id, platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, is_enabled, is_hidden) VALUES (11, 1, 10, 'page', '占用页面', 'system:other:view', 'navigation.systemOther', '/system/other', 'system/cacheGeneration', 1, 0)`,
			"component path system/cacheGeneration is occupied",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			db, ctx := openSystemConfigCacheGenerationSchema(t)
			seedSystemConfigCacheGenerationFixture(t, db, ctx)
			if err := db.WithContext(ctx).Exec(test.conflict).Error; err != nil {
				t.Fatalf("seed conflict: %v", err)
			}
			err := db.WithContext(ctx).Exec(readSystemConfigCacheGenerationMigration(t)).Error
			if err == nil {
				t.Fatal("migration accepted an incompatible menu node")
			}
			if !strings.Contains(err.Error(), test.wantMessage) {
				t.Fatalf("error = %v, want contains %q", err, test.wantMessage)
			}
			assertTableMissing(t, db, ctx, "system_config_cache_generation")
			assertTableMissing(t, db, ctx, "system_config_cache_outbox")
			assertCount(t, db, ctx, "permission_menu", 2)
			adminVersion, canvasVersion := readCacheGenerationMenuVersions(t, db, ctx)
			if adminVersion != 1 || canvasVersion != 1 {
				t.Fatalf("menu versions after rollback admin=%d canvas=%d want 1/1", adminVersion, canvasVersion)
			}
		})
	}
}

func assertCacheGenerationMenuShape(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	var page struct {
		ID            int64
		ParentID      int64
		Name          string
		MenuType      string
		I18nKey       *string
		Path          *string
		ComponentPath *string
		Icon          *string
		SortOrder     int
		IsEnabled     int
		IsHidden      int
	}
	if err := db.WithContext(ctx).Raw(`SELECT id, parent_id, name, menu_type, i18n_key, path, component_path, icon, sort_order, is_enabled, is_hidden FROM permission_menu WHERE platform_id=1 AND code='system:cacheGeneration:view' AND deleted_at IS NULL`).Scan(&page).Error; err != nil {
		t.Fatal(err)
	}
	if page.ID == 0 || page.ParentID != 10 || page.Name != "配置缓存代际" || page.MenuType != "page" ||
		page.I18nKey == nil || *page.I18nKey != "navigation.systemCacheGeneration" ||
		page.Path == nil || *page.Path != "/system/cacheGeneration" ||
		page.ComponentPath == nil || *page.ComponentPath != "system/cacheGeneration" ||
		page.Icon == nil || *page.Icon != "lucide:database-zap" ||
		page.SortOrder != 91 || page.IsEnabled != 1 || page.IsHidden != 0 {
		t.Fatalf("unexpected page shape: %+v", page)
	}

	var action struct {
		ParentID      int64
		Name          string
		MenuType      string
		I18nKey       *string
		Path          *string
		ComponentPath *string
		Icon          *string
		IsEnabled     int
		IsHidden      int
	}
	if err := db.WithContext(ctx).Raw(`SELECT parent_id, name, menu_type, i18n_key, path, component_path, icon, is_enabled, is_hidden FROM permission_menu WHERE platform_id=1 AND code='system:cacheGeneration:list' AND deleted_at IS NULL`).Scan(&action).Error; err != nil {
		t.Fatal(err)
	}
	if action.ParentID != page.ID || action.Name != "查看配置缓存代际" || action.MenuType != "action" ||
		action.I18nKey != nil || action.Path != nil || action.ComponentPath != nil || action.Icon != nil ||
		action.IsEnabled != 1 || action.IsHidden != 1 {
		t.Fatalf("unexpected action shape: %+v", action)
	}
}

func readCacheGenerationMenuVersions(t *testing.T, db *gorm.DB, ctx context.Context) (int64, int64) {
	t.Helper()
	var adminVersion, canvasVersion int64
	if err := db.WithContext(ctx).Raw(`SELECT menu_version FROM permission_auth_platform WHERE id=1`).Scan(&adminVersion).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT menu_version FROM permission_auth_platform WHERE id=2`).Scan(&canvasVersion).Error; err != nil {
		t.Fatal(err)
	}
	return adminVersion, canvasVersion
}

func assertCacheGenerationConstraint(t *testing.T, db *gorm.DB, ctx context.Context, name string) {
	t.Helper()
	var exists bool
	if err := db.WithContext(ctx).Raw(`SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname=? AND connamespace=current_schema()::regnamespace)`, name).Scan(&exists).Error; err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatalf("constraint %s is missing", name)
	}
}

func assertCacheGenerationIndex(t *testing.T, db *gorm.DB, ctx context.Context, name string) {
	t.Helper()
	var exists bool
	if err := db.WithContext(ctx).Raw(`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname=current_schema() AND indexname=?)`, name).Scan(&exists).Error; err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatalf("index %s is missing", name)
	}
}

func openSystemConfigCacheGenerationSchema(t *testing.T) (*gorm.DB, context.Context) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_system_config_cache_generation")
	for _, statement := range []string{
		`CREATE TABLE permission_auth_platform (
			id BIGINT PRIMARY KEY,
			code VARCHAR(49) NOT NULL,
			is_enabled SMALLINT NOT NULL DEFAULT 1,
			menu_version BIGINT NOT NULL DEFAULT 1,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMPTZ NULL)`,
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
			deleted_at TIMESTAMPTZ NULL,
			CONSTRAINT ck_rbac_menu_shape CHECK (
				btrim(name) <> '' AND (
					(menu_type = 'directory' AND i18n_key IS NOT NULL AND path IS NULL AND component_path IS NULL) OR
					(menu_type = 'page' AND i18n_key IS NOT NULL AND path IS NOT NULL AND btrim(path) <> '' AND component_path IS NOT NULL AND btrim(component_path) <> '') OR
					(menu_type = 'action' AND i18n_key IS NULL AND path IS NULL AND component_path IS NULL AND icon IS NULL AND is_hidden = 1)
				)))`,
		`CREATE UNIQUE INDEX ux_rbac_menu_code_active ON permission_menu (platform_id, code) WHERE deleted_at IS NULL`,
		`CREATE UNIQUE INDEX ux_rbac_menu_page_path_active ON permission_menu (platform_id, path) WHERE deleted_at IS NULL AND menu_type = 'page'`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("prepare cache generation schema: %v", err)
		}
	}
	return db, ctx
}

func seedSystemConfigCacheGenerationFixture(t *testing.T, db *gorm.DB, ctx context.Context) {
	for _, statement := range []string{
		`INSERT INTO permission_auth_platform (id, code, is_enabled, menu_version) VALUES (1, 'admin', 1, 1), (2, 'canvas', 1, 1)`,
		`INSERT INTO permission_menu (id, platform_id, menu_type, name, code, i18n_key, sort_order, is_enabled, is_hidden) VALUES (10, 1, 'directory', '系统管理', 'system', 'navigation.system', 300, 1, 0)`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("seed cache generation fixture: %v", err)
		}
	}
}

func readSystemConfigCacheGenerationMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-09-16-system-config-cache-generation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) == "" {
		t.Fatal("cache generation migration is empty")
	}
	return string(data)
}
