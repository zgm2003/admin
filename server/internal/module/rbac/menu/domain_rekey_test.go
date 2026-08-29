package menu_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/auth/platform"
	"admin/server/internal/module/rbac/menu"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type legacyMenuRow struct {
	id        int64
	parentID  *int64
	menuType  string
	code      string
	i18nKey   string
	path      *string
	viewKey   *string
	icon      *string
	sortOrder int
	deletedAt *time.Time
}

func TestPrepareSchemaRekeysLegacyMenuCatalogInPlace(t *testing.T) {
	db, ctx, retiredAt, historicalAt := openDomainRekeySchema(t, "complete")

	if err := menu.PrepareSchema(ctx, db); err != nil {
		t.Fatal(err)
	}

	rows := readRekeyedMenus(t, db)
	byCode := make(map[string]rekeyedMenuRow, len(rows))
	for _, row := range rows {
		byCode[row.Code] = row
		if strings.TrimSpace(row.Name) == "" {
			t.Errorf("menu %s has an empty name", row.Code)
		}
		if row.MenuType == "action" && (row.I18nKey != nil || row.Icon != nil || row.IsHidden != 1) {
			t.Errorf("action %s shape = %+v", row.Code, row)
		}
	}

	for id, code := range map[int64]string{
		1: "access", 2: "rbac:menu:list", 3: "rbac:menu:create", 4: "rbac:menu:update", 5: "rbac:menu:delete",
		6: "rbac:role:list", 7: "rbac:role:create", 8: "rbac:role:update", 9: "rbac:role:status",
		10: "rbac:role:default", 11: "rbac:role:delete", 12: "rbac:role:authorize",
		13: "account:user:list", 14: "account:user:update", 15: "account:user:status", 16: "account:user:delete", 17: "account:user:roles",
		18: "auth:session:list", 19: "auth:session:revoke", 20: "auth:platform:list", 21: "auth:platform:create",
		22: "auth:platform:update", 23: "auth:platform:status", 24: "auth:platform:delete", 25: "audit:operation-log:list",
	} {
		row, exists := byCode[code]
		if !exists || row.ID != id {
			t.Errorf("menu %s = %+v, want id %d", code, row, id)
		}
	}

	account := byCode["account"]
	access := byCode["access"]
	system := byCode["system"]
	if account.ID < 26 || access.ID != 1 || system.ID < 26 || account.ID == system.ID {
		t.Fatalf("root ids account=%d access=%d system=%d", account.ID, access.ID, system.ID)
	}
	if access.I18nKey == nil || *access.I18nKey != "navigation.access" ||
		access.Icon == nil || *access.Icon != "lucide:shield-check" {
		t.Fatalf("access root render fields = %+v", access)
	}
	assertRekeyedPage(t, byCode["account:user:list"], account.ID, "/account/users", "account/users", "navigation.accountUsers", "lucide:user-round-cog", 10)
	assertRekeyedPage(t, byCode["auth:session:list"], account.ID, "/account/sessions", "account/sessions", "navigation.accountSessions", "lucide:monitor-smartphone", 20)
	assertRekeyedPage(t, byCode["rbac:menu:list"], access.ID, "/access/menus", "access/menus", "navigation.accessMenus", "lucide:panel-left", 10)
	assertRekeyedPage(t, byCode["rbac:role:list"], access.ID, "/access/roles", "access/roles", "navigation.accessRoles", "lucide:user-cog", 20)
	assertRekeyedPage(t, byCode["auth:platform:list"], access.ID, "/access/auth-platforms", "access/auth-platforms", "navigation.accessAuthPlatforms", "lucide:key-round", 30)
	assertRekeyedPage(t, byCode["audit:operation-log:list"], system.ID, "/system/operation-logs", "system/operation-logs", "navigation.systemOperationLogs", "lucide:scroll-text", 10)

	for _, code := range []string{"rbac:menu:list", "rbac:menu:create", "rbac:menu:update", "rbac:menu:delete"} {
		if byCode[code].DeletedAt != nil {
			t.Errorf("protected menu %s remains deleted at %v", code, byCode[code].DeletedAt)
		}
	}
	assertRoleMenuDeletion(t, db, 101, nil)
	assertRoleMenuDeletion(t, db, 102, nil)
	assertRoleMenuDeletion(t, db, 103, &historicalAt)
	assertRoleMenuDeletion(t, db, 104, nil)
	if retiredAt.Equal(historicalAt) {
		t.Fatal("fixture deletion times must differ")
	}
	assertAccessVersions(t, db, map[int64]int64{7001: 5, 7002: 8})

	customName := "自定义权限中心"
	if err := db.Exec(`UPDATE rbac_menu SET name = ?, sort_order = 260 WHERE code = 'access'`, customName).Error; err != nil {
		t.Fatal(err)
	}
	if err := menu.PrepareSchema(ctx, db); err != nil {
		t.Fatalf("second PrepareSchema() error = %v", err)
	}
	var repeated struct {
		Name      string
		SortOrder int `gorm:"column:sort_order"`
	}
	if err := db.Raw(`SELECT name, sort_order FROM rbac_menu WHERE code = 'access'`).Scan(&repeated).Error; err != nil {
		t.Fatal(err)
	}
	if repeated.Name != customName || repeated.SortOrder != 260 {
		t.Fatalf("second migration overwrote access root: %+v", repeated)
	}
	assertAccessVersions(t, db, map[int64]int64{7001: 5, 7002: 8})
}

func TestPrepareSchemaRejectsUnknownLegacyIconAndRollsBack(t *testing.T) {
	db, ctx, _, _ := openDomainRekeySchema(t, "unknown_icon")
	if err := db.Exec(`UPDATE rbac_menu SET icon = 'UnknownIcon' WHERE code = 'system:session:list'`).Error; err != nil {
		t.Fatal(err)
	}
	err := menu.PrepareSchema(ctx, db)
	if err == nil || !strings.Contains(err.Error(), "system:session:list") || !strings.Contains(err.Error(), "UnknownIcon") {
		t.Fatalf("PrepareSchema() error = %v", err)
	}
	var code string
	if err := db.Raw(`SELECT code FROM rbac_menu WHERE id = 1`).Scan(&code).Error; err != nil {
		t.Fatal(err)
	}
	if code != "system" {
		t.Fatalf("rollback root code = %q", code)
	}
}

func TestPrepareSchemaRepairsCorruptedAccessRootI18nKey(t *testing.T) {
	connection, ctx := openMenuSchema(t)
	db := connection.GORM.WithContext(ctx)
	var adminPlatform authplatform.Platform
	if err := db.Where("code = ?", authplatform.BuiltinAdminCode).Take(&adminPlatform).Error; err != nil {
		t.Fatal(err)
	}
	corrupted := "lucide:shield-check"
	access := menu.Menu{
		PlatformID: adminPlatform.ID, MenuType: menu.TypeDirectory, Name: "权限与认证", Code: "access",
		I18nKey: &corrupted, Icon: &corrupted, SortOrder: 200,
		IsEnabled: yesno.Yes, IsHidden: yesno.No,
	}
	if err := db.Create(&access).Error; err != nil {
		t.Fatalf("create corrupted access root: %v", err)
	}

	if err := menu.PrepareSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("PrepareSchema() error = %v", err)
	}

	var stored menu.Menu
	if err := db.Where("id = ?", access.ID).Take(&stored).Error; err != nil {
		t.Fatalf("read repaired access root: %v", err)
	}
	if stored.I18nKey == nil || *stored.I18nKey != "navigation.access" {
		t.Fatalf("repaired access i18n key = %v", stored.I18nKey)
	}
}

type rekeyedMenuRow struct {
	ID            int64
	ParentID      *int64
	MenuType      string
	Code          string
	Name          string
	I18nKey       *string
	Path          *string
	ComponentPath *string
	Icon          *string
	SortOrder     int
	IsHidden      int16
	DeletedAt     *time.Time
}

func readRekeyedMenus(t *testing.T, db *gorm.DB) []rekeyedMenuRow {
	t.Helper()
	var rows []rekeyedMenuRow
	if err := db.Raw(`
		SELECT id, parent_id, menu_type, code, name, i18n_key, path, component_path, icon,
		       sort_order, is_hidden, deleted_at
		FROM rbac_menu ORDER BY id`).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	return rows
}

func assertRekeyedPage(t *testing.T, row rekeyedMenuRow, parentID int64, path, componentPath, i18nKey, icon string, sortOrder int) {
	t.Helper()
	if row.ParentID == nil || *row.ParentID != parentID || row.Path == nil || *row.Path != path ||
		row.ComponentPath == nil || *row.ComponentPath != componentPath || row.I18nKey == nil || *row.I18nKey != i18nKey ||
		row.Icon == nil || *row.Icon != icon || row.SortOrder != sortOrder || row.DeletedAt != nil {
		t.Errorf("page %s = %+v", row.Code, row)
	}
}

func assertRoleMenuDeletion(t *testing.T, db *gorm.DB, id int64, want *time.Time) {
	t.Helper()
	var got sql.NullTime
	if err := db.Raw(`SELECT deleted_at FROM rbac_role_menu WHERE id = ?`, id).Scan(&got).Error; err != nil {
		t.Fatal(err)
	}
	if want == nil && got.Valid {
		t.Errorf("role menu %d deletedAt = %v, want nil", id, got)
	}
	if want != nil && (!got.Valid || !got.Time.Equal(*want)) {
		t.Errorf("role menu %d deletedAt = %v, want %v", id, got, want)
	}
}

func assertAccessVersions(t *testing.T, db *gorm.DB, want map[int64]int64) {
	t.Helper()
	var rows []struct {
		UserID  int64
		Version int64
	}
	if err := db.Raw(`SELECT user_id, version FROM rbac_access_version ORDER BY user_id`).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(want) {
		t.Fatalf("access versions = %+v", rows)
	}
	for _, row := range rows {
		if row.Version != want[row.UserID] {
			t.Errorf("access version user %d = %d, want %d", row.UserID, row.Version, want[row.UserID])
		}
	}
}

func openDomainRekeySchema(t *testing.T, suffix string) (*gorm.DB, context.Context, time.Time, time.Time) {
	t.Helper()
	connection, ctx := openMenuSchema(t)
	tx := connection.GORM.WithContext(ctx).Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	schema := fmt.Sprintf("menu_rekey_%s_%d", suffix, time.Now().UnixNano())
	for _, statement := range []string{
		`CREATE SCHEMA ` + schema,
		`SET LOCAL search_path TO ` + schema,
		`CREATE TABLE rbac_menu (
			id BIGSERIAL PRIMARY KEY,
			parent_id BIGINT,
			menu_type VARCHAR(16) NOT NULL,
			code VARCHAR(128) NOT NULL,
			i18n_key VARCHAR(128) NOT NULL,
			path VARCHAR(255),
			component_path VARCHAR(255),
			view_key VARCHAR(128),
			icon VARCHAR(128),
			sort_order INTEGER NOT NULL DEFAULT 0,
			is_enabled SMALLINT NOT NULL DEFAULT 1,
			is_hidden SMALLINT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMPTZ
		)`,
		`CREATE UNIQUE INDEX ux_rbac_menu_code_active ON rbac_menu (code) WHERE deleted_at IS NULL`,
		`CREATE TABLE rbac_role_menu (
			id BIGINT PRIMARY KEY,
			role_id BIGINT NOT NULL,
			menu_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMPTZ
		)`,
		`CREATE TABLE rbac_access_version (
			user_id BIGINT PRIMARY KEY,
			version BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	} {
		if err := tx.Exec(statement).Error; err != nil {
			t.Fatalf("prepare rekey schema: %v", err)
		}
	}

	retiredAt := time.Date(2026, 8, 20, 8, 0, 0, 123000, time.UTC)
	historicalAt := retiredAt.Add(-24 * time.Hour)
	for _, row := range legacyMenuRows(retiredAt) {
		if err := tx.Exec(`
			INSERT INTO rbac_menu
				(id, parent_id, menu_type, code, i18n_key, path, component_path, view_key, icon,
				 sort_order, is_enabled, is_hidden, deleted_at)
			VALUES (?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, 1, ?, ?)`,
			row.id, row.parentID, row.menuType, row.code, row.i18nKey, row.path, row.viewKey, row.icon,
			row.sortOrder, boolToSmallint(row.menuType == "action"), row.deletedAt).Error; err != nil {
			t.Fatalf("insert legacy menu %s: %v", row.code, err)
		}
	}
	for _, statement := range []struct {
		id        int64
		menuID    int64
		deletedAt *time.Time
	}{
		{id: 101, menuID: 2, deletedAt: &retiredAt},
		{id: 102, menuID: 3, deletedAt: &retiredAt},
		{id: 103, menuID: 4, deletedAt: &historicalAt},
		{id: 104, menuID: 6},
	} {
		if err := tx.Exec(`INSERT INTO rbac_role_menu (id, role_id, menu_id, deleted_at) VALUES (?, 9001, ?, ?)`, statement.id, statement.menuID, statement.deletedAt).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Exec(`INSERT INTO rbac_access_version (user_id, version) VALUES (7001, 4), (7002, 7)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(`SELECT setval(pg_get_serial_sequence('rbac_menu', 'id'), 25, true)`).Error; err != nil {
		t.Fatal(err)
	}
	return tx, ctx, retiredAt, historicalAt
}

func legacyMenuRows(retiredAt time.Time) []legacyMenuRow {
	rootID, menuPageID, rolePageID, userPageID, sessionPageID, platformPageID := int64(1), int64(2), int64(6), int64(13), int64(18), int64(20)
	deleted := func(id int64) *time.Time {
		if id >= 2 && id <= 5 {
			value := retiredAt
			return &value
		}
		return nil
	}
	page := func(id int64, code, i18nKey, path, viewKey, icon string, sort int) legacyMenuRow {
		return legacyMenuRow{id: id, parentID: &rootID, menuType: "page", code: code, i18nKey: i18nKey, path: rekeyStringPointer(path), viewKey: rekeyStringPointer(viewKey), icon: rekeyStringPointer(icon), sortOrder: sort, deletedAt: deleted(id)}
	}
	action := func(id, parentID int64, code, i18nKey string, sort int) legacyMenuRow {
		return legacyMenuRow{id: id, parentID: &parentID, menuType: "action", code: code, i18nKey: i18nKey, sortOrder: sort, deletedAt: deleted(id)}
	}
	return []legacyMenuRow{
		{id: 1, menuType: "directory", code: "system", i18nKey: "navigation.system", icon: rekeyStringPointer("Setting"), sortOrder: 10},
		page(2, "system:menu:list", "navigation.systemMenus", "/system/menus", "system-menus", "Menu", 10),
		action(3, menuPageID, "system:menu:create", "permission.menuCreate", 10),
		action(4, menuPageID, "system:menu:update", "permission.menuUpdate", 20),
		action(5, menuPageID, "system:menu:delete", "permission.menuDelete", 30),
		page(6, "system:role:list", "navigation.systemRoles", "/system/roles", "system-roles", "UserFilled", 20),
		action(7, rolePageID, "system:role:create", "permission.roleCreate", 10),
		action(8, rolePageID, "system:role:update", "permission.roleUpdate", 20),
		action(9, rolePageID, "system:role:status", "permission.roleStatus", 30),
		action(10, rolePageID, "system:role:default", "permission.roleDefault", 40),
		action(11, rolePageID, "system:role:delete", "permission.roleDelete", 50),
		action(12, rolePageID, "system:role:authorize", "permission.roleAuthorize", 60),
		page(13, "system:user:list", "navigation.systemUsers", "/system/users", "system-users", "User", 30),
		action(14, userPageID, "system:user:update", "permission.userUpdate", 10),
		action(15, userPageID, "system:user:status", "permission.userStatus", 20),
		action(16, userPageID, "system:user:delete", "permission.userDelete", 30),
		action(17, userPageID, "system:user:roles", "permission.userRoles", 40),
		page(18, "system:session:list", "navigation.systemSessions", "/system/sessions", "system-sessions", "List", 40),
		action(19, sessionPageID, "system:session:revoke", "permission.sessionRevoke", 10),
		page(20, "system:auth-platform:list", "navigation.systemAuthPlatforms", "/system/auth-platforms", "system-auth-platforms", "Key", 50),
		action(21, platformPageID, "system:auth-platform:create", "permission.authPlatformCreate", 10),
		action(22, platformPageID, "system:auth-platform:update", "permission.authPlatformUpdate", 20),
		action(23, platformPageID, "system:auth-platform:status", "permission.authPlatformStatus", 30),
		action(24, platformPageID, "system:auth-platform:delete", "permission.authPlatformDelete", 40),
		page(25, "system:operation-log:list", "navigation.systemOperationLogs", "/system/operation-logs", "system-operation-logs", "List", 60),
	}
}

func boolToSmallint(value bool) int16 {
	if value {
		return 1
	}
	return 0
}

func rekeyStringPointer(value string) *string {
	return &value
}
