package menu_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/menu"
	"gorm.io/gorm"
)

func TestMenuProtocolMigrationConvertsKnownPagesAndRetiresStaticMenuNode(t *testing.T) {
	connection, ctx := openMenuSchema(t)
	tx := openLegacyMenuSchema(t, connection.GORM, "known")

	if err := tx.AutoMigrate(&menu.Menu{}, &menu.RoleMenu{}); err != nil {
		t.Fatalf("add new menu model columns: %v", err)
	}
	if err := menu.EnsureSchema(ctx, tx); err != nil {
		t.Fatalf("migrate legacy menu schema: %v", err)
	}

	var pages []struct {
		Code          string  `gorm:"column:code"`
		ComponentPath *string `gorm:"column:component_path"`
		IsHidden      int16   `gorm:"column:is_hidden"`
		DeletedAt     *time.Time
	}
	if err := tx.Table("sys_menu").Order("id").Find(&pages).Error; err != nil {
		t.Fatalf("read migrated menus: %v", err)
	}
	wantPaths := map[string]string{
		"system:role:list":          "system/roles",
		"system:user:list":          "system/users",
		"system:auth-platform:list": "system/auth-platforms",
		"system:session:list":       "system/sessions",
		"system:operation-log:list": "system/operation-logs",
	}
	for _, page := range pages {
		if want, exists := wantPaths[page.Code]; exists {
			if page.ComponentPath == nil || *page.ComponentPath != want || page.IsHidden != 0 || page.DeletedAt != nil {
				t.Errorf("migrated page %s = %+v, want componentPath=%s visible and active", page.Code, page, want)
			}
		}
		if strings.HasPrefix(page.Code, "system:menu:") && page.DeletedAt == nil {
			t.Errorf("static menu management node %s remains active", page.Code)
		}
		if strings.HasSuffix(page.Code, ":update") && strings.Contains(page.Code, "role") && page.IsHidden != 1 {
			t.Errorf("action %s isHidden = %d, want 1", page.Code, page.IsHidden)
		}
	}

	var activeStaticGrants int64
	if err := tx.Table("sys_role_menu").Where("menu_id IN ? AND deleted_at IS NULL", []int64{2, 3, 4, 5}).Count(&activeStaticGrants).Error; err != nil {
		t.Fatalf("count static menu grants: %v", err)
	}
	if activeStaticGrants != 0 {
		t.Fatalf("active static menu grants = %d, want 0", activeStaticGrants)
	}
	assertMigrationColumnMissing(t, tx, "view_key")

	if err := tx.Exec(`UPDATE sys_menu SET is_hidden = 1 WHERE code = 'system:role:list'`).Error; err != nil {
		t.Fatalf("hide migrated page: %v", err)
	}
	if err := menu.EnsureSchema(ctx, tx); err != nil {
		t.Fatalf("repeat menu schema migration: %v", err)
	}
	var hidden int16
	if err := tx.Raw(`SELECT is_hidden FROM sys_menu WHERE code = 'system:role:list'`).Scan(&hidden).Error; err != nil {
		t.Fatalf("read hidden page after repeated migration: %v", err)
	}
	if hidden != 1 {
		t.Fatalf("hidden page after repeated migration = %d, want 1", hidden)
	}
}

func TestMenuProtocolMigrationRejectsUnknownPageMappingAndRollsBack(t *testing.T) {
	connection, ctx := openMenuSchema(t)
	tx := openLegacyMenuSchema(t, connection.GORM, "unknown")
	if err := tx.Exec(`UPDATE sys_menu SET view_key = 'custom-view' WHERE code = 'system:user:list'`).Error; err != nil {
		t.Fatalf("set unknown view key: %v", err)
	}
	if err := tx.AutoMigrate(&menu.Menu{}, &menu.RoleMenu{}); err != nil {
		t.Fatalf("add new menu model columns: %v", err)
	}

	err := menu.EnsureSchema(ctx, tx)
	if err == nil || !strings.Contains(err.Error(), "system:user:list") || !strings.Contains(err.Error(), "custom-view") {
		t.Fatalf("unknown view key error = %v", err)
	}
	assertMigrationColumnExists(t, tx, "view_key")
}

func openLegacyMenuSchema(t *testing.T, db *gorm.DB, suffix string) *gorm.DB {
	t.Helper()
	schemaName := fmt.Sprintf("menu_protocol_%s_%d", suffix, time.Now().UnixNano())
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin legacy schema transaction: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	if err := tx.Exec(`CREATE SCHEMA ` + schemaName).Error; err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	if err := tx.Exec(`SET LOCAL search_path TO ` + schemaName).Error; err != nil {
		t.Fatalf("set legacy schema search path: %v", err)
	}
	statements := []string{
		`CREATE TABLE sys_role (id BIGINT PRIMARY KEY)`,
		`CREATE TABLE sys_menu (
			id BIGINT PRIMARY KEY,
			parent_id BIGINT NULL,
			menu_type VARCHAR(16) NOT NULL,
			code VARCHAR(128) NOT NULL,
			i18n_key VARCHAR(128) NOT NULL,
			path VARCHAR(255) NULL,
			view_key VARCHAR(128) NULL,
			icon VARCHAR(64) NULL,
			sort_order INTEGER NOT NULL DEFAULT 0,
			is_enabled SMALLINT NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMPTZ NULL
		)`,
		`CREATE TABLE sys_role_menu (
			id BIGINT PRIMARY KEY,
			role_id BIGINT NOT NULL,
			menu_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMPTZ NULL
		)`,
		`INSERT INTO sys_role (id) VALUES (1)`,
		`INSERT INTO sys_menu (id, parent_id, menu_type, code, i18n_key, path, view_key, sort_order) VALUES
			(1, NULL, 'directory', 'system', 'navigation.system', NULL, NULL, 10),
			(2, 1, 'page', 'system:menu:list', 'navigation.systemMenus', '/system/menus', 'system-menus', 10),
			(3, 2, 'action', 'system:menu:create', 'permission.menuCreate', NULL, NULL, 10),
			(4, 2, 'action', 'system:menu:update', 'permission.menuUpdate', NULL, NULL, 20),
			(5, 2, 'action', 'system:menu:delete', 'permission.menuDelete', NULL, NULL, 30),
			(6, 1, 'page', 'system:role:list', 'navigation.systemRoles', '/system/roles', 'system-roles', 20),
			(7, 6, 'action', 'system:role:update', 'permission.roleUpdate', NULL, NULL, 10),
			(8, 1, 'page', 'system:user:list', 'navigation.systemUsers', '/system/users', 'system-users', 30),
			(9, 1, 'page', 'system:auth-platform:list', 'navigation.systemAuthPlatforms', '/system/auth-platforms', 'system-auth-platforms', 40),
			(10, 1, 'page', 'system:session:list', 'navigation.systemSessions', '/system/sessions', 'system-sessions', 50),
			(11, 1, 'page', 'system:operation-log:list', 'navigation.systemOperationLogs', '/system/operation-logs', 'system-operation-logs', 60)`,
		`INSERT INTO sys_role_menu (id, role_id, menu_id) VALUES (1, 1, 2), (2, 1, 3), (3, 1, 6)`,
	}
	for _, statement := range statements {
		if err := tx.Exec(statement).Error; err != nil {
			t.Fatalf("prepare legacy schema: %v", err)
		}
	}
	return tx
}

func assertMigrationColumnMissing(t *testing.T, db *gorm.DB, columnName string) {
	t.Helper()
	if migrationColumnExists(t, db, columnName) {
		t.Fatalf("column sys_menu.%s still exists", columnName)
	}
}

func assertMigrationColumnExists(t *testing.T, db *gorm.DB, columnName string) {
	t.Helper()
	if !migrationColumnExists(t, db, columnName) {
		t.Fatalf("column sys_menu.%s does not exist", columnName)
	}
}

func migrationColumnExists(t *testing.T, db *gorm.DB, columnName string) bool {
	t.Helper()
	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = 'sys_menu'
			  AND column_name = ?
		)`, columnName).Scan(&exists).Error; err != nil {
		t.Fatalf("inspect migration column %s: %v", columnName, err)
	}
	return exists
}
