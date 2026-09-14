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

func TestSystemQueueMonitorMigrationIsIdempotentAndPlatformScoped(t *testing.T) {
	db, ctx := openSystemQueueMonitorSchema(t)
	seedSystemQueueMonitorFixture(t, db, ctx)
	script := readSystemQueueMonitorMigration(t)
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("execute queue monitor migration: %v", err)
	}
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("re-execute queue monitor migration: %v", err)
	}

	var pages, actions, version int64
	db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_menu WHERE platform_id = 1 AND code = 'system:queueMonitor:view' AND deleted_at IS NULL`).Scan(&pages)
	db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_menu WHERE platform_id = 1 AND code = 'system:queueMonitor:list' AND deleted_at IS NULL`).Scan(&actions)
	db.WithContext(ctx).Raw(`SELECT menu_version FROM permission_auth_platform WHERE id = 1`).Scan(&version)
	if pages != 1 || actions != 1 || version != 2 {
		t.Fatalf("queue menu counts/version = %d/%d/%d, want 1/1/2", pages, actions, version)
	}

	var row struct {
		ParentID                     int64
		Path, ComponentPath, I18nKey *string
		MenuType                     string
		IsHidden                     int
	}
	if err := db.WithContext(ctx).Raw(`SELECT parent_id, path, component_path, i18n_key, menu_type, is_hidden FROM permission_menu WHERE code = 'system:queueMonitor:view' AND deleted_at IS NULL`).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ParentID != 10 || row.Path == nil || *row.Path != "/system/queueMonitor" || row.ComponentPath == nil || *row.ComponentPath != "system/queueMonitor" || row.I18nKey == nil || *row.I18nKey != "navigation.systemQueueMonitor" || row.MenuType != "page" || row.IsHidden != 0 {
		t.Fatalf("unexpected page shape: %+v", row)
	}
}

func openSystemQueueMonitorSchema(t *testing.T) (*gorm.DB, context.Context) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_system_queue_monitor")
	for _, statement := range []string{
		`CREATE TABLE permission_auth_platform (id BIGINT PRIMARY KEY, code VARCHAR(49) NOT NULL, is_enabled SMALLINT NOT NULL, menu_version BIGINT NOT NULL DEFAULT 1, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, deleted_at TIMESTAMPTZ NULL)`,
		`CREATE TABLE permission_menu (id BIGSERIAL PRIMARY KEY, platform_id BIGINT NOT NULL, parent_id BIGINT NULL, menu_type VARCHAR(16) NOT NULL, name VARCHAR(128) NOT NULL, code VARCHAR(128) NOT NULL, i18n_key VARCHAR(128) NULL, path VARCHAR(255) NULL, component_path VARCHAR(255) NULL, icon VARCHAR(128) NULL, sort_order INTEGER NOT NULL DEFAULT 0, is_enabled SMALLINT NOT NULL DEFAULT 1, is_hidden SMALLINT NOT NULL DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, deleted_at TIMESTAMPTZ NULL)`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("prepare queue monitor schema: %v", err)
		}
	}
	return db, ctx
}

func seedSystemQueueMonitorFixture(t *testing.T, db *gorm.DB, ctx context.Context) {
	for _, statement := range []string{
		`INSERT INTO permission_auth_platform (id, code, is_enabled) VALUES (1, 'admin', 1), (2, 'canvas', 1)`,
		`INSERT INTO permission_menu (id, platform_id, menu_type, name, code, i18n_key, is_enabled, is_hidden) VALUES (10, 1, 'directory', '系统设置', 'system', 'navigation.system', 1, 0)`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("seed queue monitor fixture: %v", err)
		}
	}
}

func readSystemQueueMonitorMigration(t *testing.T) string {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-09-14-system-queue-monitor.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) == "" {
		t.Fatal("queue monitor migration is empty")
	}
	return string(data)
}
