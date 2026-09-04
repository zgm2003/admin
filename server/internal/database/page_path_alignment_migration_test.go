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

func stringPointer(value string) *string { return &value }

func int64Pointer(value int64) *int64 { return &value }

type pagePathAlignmentFixture struct {
	accountID  int64
	accessID   int64
	cloudID    int64
	roleID     int64
	userID     int64
	pageIDs    map[string]int64
	pageParent map[string]int64
	grants     []pagePathAlignmentGrantRow
}

type pagePathAlignmentMenuRow struct {
	ID            int64   `gorm:"column:id"`
	ParentID      *int64  `gorm:"column:parent_id"`
	Code          string  `gorm:"column:code"`
	Path          *string `gorm:"column:path"`
	ComponentPath *string `gorm:"column:component_path"`
}

type pagePathAlignmentGrantRow struct {
	ID     int64 `gorm:"column:id"`
	RoleID int64 `gorm:"column:role_id"`
	MenuID int64 `gorm:"column:menu_id"`
}

func TestPagePathAlignmentMigrationAlignsPagePathsAndIsIdempotent(t *testing.T) {
	db, ctx := openPagePathAlignmentSchema(t)
	fixture := createPagePathAlignmentFixture(t, db, ctx, false)
	script := readPagePathAlignmentMigration(t)

	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("execute page path alignment migration: %v", err)
	}

	assertPagePathAlignmentMenus(t, db, ctx, fixture, map[string]struct{ path, componentPath string }{
		"permission:menu:view":       {"/permission/menus", "permission/menus"},
		"permission:role:view":       {"/permission/roles", "permission/roles"},
		"auth:platform:view":         {"/permission/auth-platforms", "permission/auth-platforms"},
		"account:user:loginlog:view": {"/account/login-logs", "account/login-logs"},
		"storage:object:view":        {"/cloud/storage-object", "cloud/storage-object"},
	})
	assertPagePathAlignmentGrants(t, db, ctx, fixture.grants)
	assertPagePathAlignmentVersion(t, db, ctx, fixture.userID, 2)

	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("execute page path alignment migration twice: %v", err)
	}
	assertPagePathAlignmentMenus(t, db, ctx, fixture, map[string]struct{ path, componentPath string }{
		"permission:menu:view":       {"/permission/menus", "permission/menus"},
		"permission:role:view":       {"/permission/roles", "permission/roles"},
		"auth:platform:view":         {"/permission/auth-platforms", "permission/auth-platforms"},
		"account:user:loginlog:view": {"/account/login-logs", "account/login-logs"},
		"storage:object:view":        {"/cloud/storage-object", "cloud/storage-object"},
	})
	assertPagePathAlignmentGrants(t, db, ctx, fixture.grants)
	assertPagePathAlignmentVersion(t, db, ctx, fixture.userID, 2)
}

func TestPagePathAlignmentMigrationRejectsOccupiedTargetPathAndRollsBack(t *testing.T) {
	db, ctx := openPagePathAlignmentSchema(t)
	fixture := createPagePathAlignmentFixture(t, db, ctx, true)
	script := readPagePathAlignmentMigration(t)

	err := db.WithContext(ctx).Exec(script).Error
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "occupied") {
		t.Fatalf("expected occupied-path error, got %v", err)
	}

	assertPagePathAlignmentMenus(t, db, ctx, fixture, map[string]struct{ path, componentPath string }{
		"permission:menu:view":       {"/access/menus", "access/menus"},
		"permission:role:view":       {"/access/roles", "access/roles"},
		"auth:platform:view":         {"/access/auth-platforms", "access/auth-platforms"},
		"account:user:loginlog:view": {"/account/login-logs", "user/login-logs"},
		"storage:object:view":        {"/cloud/object-storage", "storage/object"},
	})
	assertPagePathAlignmentGrants(t, db, ctx, fixture.grants)
	assertPagePathAlignmentVersion(t, db, ctx, fixture.userID, 1)
}

func openPagePathAlignmentSchema(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_page_path_alignment")
	for _, statement := range []string{
		`CREATE TABLE permission_menu (
			id BIGINT PRIMARY KEY,
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
		`CREATE TABLE permission_role_menu (
			id BIGINT PRIMARY KEY,
			role_id BIGINT NOT NULL,
			menu_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMPTZ NULL
		)`,
		`CREATE TABLE permission_access_version (
			user_id BIGINT PRIMARY KEY,
			version BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("prepare page path alignment schema: %v", err)
		}
	}
	return db, ctx
}

func createPagePathAlignmentFixture(t *testing.T, db *gorm.DB, ctx context.Context, withConflict bool) pagePathAlignmentFixture {
	t.Helper()
	accountID := int64(1)
	accessID := int64(2)
	cloudID := int64(3)
	roleID := int64(9001)
	userID := int64(9101)
	pageIDs := map[string]int64{
		"permission:menu:view":       10,
		"permission:role:view":       11,
		"auth:platform:view":         12,
		"account:user:loginlog:view": 13,
		"storage:object:view":        14,
	}
	pageParent := map[string]int64{
		"permission:menu:view":       accessID,
		"permission:role:view":       accessID,
		"auth:platform:view":         accessID,
		"account:user:loginlog:view": accountID,
		"storage:object:view":        cloudID,
	}

	for _, statement := range []struct {
		id        int64
		parentID  *int64
		menuType  string
		name      string
		code      string
		i18nKey   *string
		path      *string
		component *string
		icon      *string
		sortOrder int
	}{
		{id: accountID, menuType: "directory", name: "Account", code: "account", i18nKey: stringPointer("navigation.account"), sortOrder: 10},
		{id: accessID, menuType: "directory", name: "Access", code: "access", i18nKey: stringPointer("navigation.access"), sortOrder: 20},
		{id: cloudID, menuType: "directory", name: "Cloud", code: "cloud", i18nKey: stringPointer("navigation.cloud"), sortOrder: 30},
		{id: pageIDs["permission:menu:view"], parentID: int64Pointer(accessID), menuType: "page", name: "Menu", code: "permission:menu:view", i18nKey: stringPointer("navigation.accessMenus"), path: stringPointer("/access/menus"), component: stringPointer("access/menus"), icon: stringPointer("lucide:panel-left"), sortOrder: 10},
		{id: pageIDs["permission:role:view"], parentID: int64Pointer(accessID), menuType: "page", name: "Role", code: "permission:role:view", i18nKey: stringPointer("navigation.accessRoles"), path: stringPointer("/access/roles"), component: stringPointer("access/roles"), icon: stringPointer("lucide:user-cog"), sortOrder: 20},
		{id: pageIDs["auth:platform:view"], parentID: int64Pointer(accessID), menuType: "page", name: "Auth Platform", code: "auth:platform:view", i18nKey: stringPointer("navigation.accessAuthPlatforms"), path: stringPointer("/access/auth-platforms"), component: stringPointer("access/auth-platforms"), icon: stringPointer("lucide:key-round"), sortOrder: 30},
		{id: pageIDs["account:user:loginlog:view"], parentID: int64Pointer(accountID), menuType: "page", name: "Login Log", code: "account:user:loginlog:view", i18nKey: stringPointer("navigation.accountLoginLogs"), path: stringPointer("/account/login-logs"), component: stringPointer("user/login-logs"), icon: stringPointer("lucide:lock-keyhole"), sortOrder: 40},
		{id: pageIDs["storage:object:view"], parentID: int64Pointer(cloudID), menuType: "page", name: "Object Storage", code: "storage:object:view", i18nKey: stringPointer("navigation.storageObject"), path: stringPointer("/cloud/object-storage"), component: stringPointer("storage/object"), icon: stringPointer("lucide:cloud-upload"), sortOrder: 10},
	} {
		if err := db.WithContext(ctx).Exec(`
			INSERT INTO permission_menu
				(id, platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, icon, sort_order, is_enabled, is_hidden)
			VALUES (?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 0)`,
			statement.id, statement.parentID, statement.menuType, statement.name, statement.code, statement.i18nKey, statement.path, statement.component, statement.icon, statement.sortOrder).Error; err != nil {
			t.Fatalf("insert page path alignment fixture %s: %v", statement.code, err)
		}
	}

	if withConflict {
		if err := db.WithContext(ctx).Exec(`
			INSERT INTO permission_menu
				(id, platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, icon, sort_order, is_enabled, is_hidden)
			VALUES (15, 1, ?, 'page', 'Role Path Occupied', 'test:occupied:role:view', 'navigation.test', '/permission/roles', 'permission/roles', 'lucide:alert-circle', 90, 1, 0)`,
			accessID).Error; err != nil {
			t.Fatalf("insert occupied page: %v", err)
		}
	}

	for _, grant := range []pagePathAlignmentGrantRow{
		{ID: 101, RoleID: roleID, MenuID: pageIDs["permission:menu:view"]},
		{ID: 102, RoleID: roleID, MenuID: pageIDs["permission:role:view"]},
		{ID: 103, RoleID: roleID, MenuID: pageIDs["auth:platform:view"]},
		{ID: 104, RoleID: roleID, MenuID: pageIDs["account:user:loginlog:view"]},
		{ID: 105, RoleID: roleID, MenuID: pageIDs["storage:object:view"]},
	} {
		if err := db.WithContext(ctx).Exec(`
			INSERT INTO permission_role_menu (id, role_id, menu_id)
			VALUES (?, ?, ?)`,
			grant.ID, grant.RoleID, grant.MenuID).Error; err != nil {
			t.Fatalf("insert page path alignment grant %d: %v", grant.ID, err)
		}
	}

	if err := db.WithContext(ctx).Exec(`INSERT INTO permission_access_version (user_id, version) VALUES (?, 1)`, userID).Error; err != nil {
		t.Fatalf("insert access version: %v", err)
	}

	return pagePathAlignmentFixture{
		accountID:  accountID,
		accessID:   accessID,
		cloudID:    cloudID,
		roleID:     roleID,
		userID:     userID,
		pageIDs:    pageIDs,
		pageParent: pageParent,
		grants:     []pagePathAlignmentGrantRow{{ID: 101, RoleID: roleID, MenuID: pageIDs["permission:menu:view"]}, {ID: 102, RoleID: roleID, MenuID: pageIDs["permission:role:view"]}, {ID: 103, RoleID: roleID, MenuID: pageIDs["auth:platform:view"]}, {ID: 104, RoleID: roleID, MenuID: pageIDs["account:user:loginlog:view"]}, {ID: 105, RoleID: roleID, MenuID: pageIDs["storage:object:view"]}},
	}
}

func assertPagePathAlignmentMenus(t *testing.T, db *gorm.DB, ctx context.Context, fixture pagePathAlignmentFixture, want map[string]struct{ path, componentPath string }) {
	t.Helper()
	for code, expected := range want {
		var row pagePathAlignmentMenuRow
		if err := db.WithContext(ctx).Raw(`
			SELECT id, parent_id, code, path, component_path
			FROM permission_menu
			WHERE code = ? AND deleted_at IS NULL`, code).Scan(&row).Error; err != nil {
			t.Fatalf("read page %s: %v", code, err)
		}
		if row.ID != fixture.pageIDs[code] {
			t.Fatalf("page %s id = %d, want %d", code, row.ID, fixture.pageIDs[code])
		}
		if row.ParentID == nil || *row.ParentID != fixture.pageParent[code] {
			t.Fatalf("page %s parent_id = %v, want %d", code, row.ParentID, fixture.pageParent[code])
		}
		if row.Path == nil || *row.Path != expected.path || row.ComponentPath == nil || *row.ComponentPath != expected.componentPath {
			t.Fatalf("page %s path/component = %v/%v, want %s/%s", code, row.Path, row.ComponentPath, expected.path, expected.componentPath)
		}
	}
}

func assertPagePathAlignmentGrants(t *testing.T, db *gorm.DB, ctx context.Context, want []pagePathAlignmentGrantRow) {
	t.Helper()
	var got []pagePathAlignmentGrantRow
	if err := db.WithContext(ctx).Raw(`SELECT id, role_id, menu_id FROM permission_role_menu ORDER BY id`).Scan(&got).Error; err != nil {
		t.Fatalf("read permission_role_menu: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("permission_role_menu row count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("permission_role_menu[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func assertPagePathAlignmentVersion(t *testing.T, db *gorm.DB, ctx context.Context, userID, want int64) {
	t.Helper()
	var got int64
	if err := db.WithContext(ctx).Raw(`SELECT version FROM permission_access_version WHERE user_id = ?`, userID).Scan(&got).Error; err != nil {
		t.Fatalf("read access version: %v", err)
	}
	if got != want {
		t.Fatalf("permission_access_version = %d, want %d", got, want)
	}
}

func readPagePathAlignmentMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(rbacRepoRoot(t), "docs", "database", "2026-09-03-page-path-alignment.sql"))
	if err != nil {
		t.Fatalf("read page path alignment migration: %v", err)
	}
	return string(data)
}
