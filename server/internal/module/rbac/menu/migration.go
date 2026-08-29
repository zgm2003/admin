package menu

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"admin/server/internal/module/auth/platform"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type legacyIconTarget struct {
	Old string
	New string
}

type migratedPageTarget struct {
	ParentCode    string
	Path          string
	ComponentPath string
	I18nKey       string
	Icon          string
	SortOrder     int
}

type legacyMigrationRow struct {
	ID        int64
	MenuType  Type
	Code      string
	ViewKey   *string
	Icon      *string
	SortOrder int
}

var legacyPermissionCodes = map[string]string{
	"system:menu:list": "rbac:menu:list", "system:menu:create": "rbac:menu:create",
	"system:menu:update": "rbac:menu:update", "system:menu:delete": "rbac:menu:delete",
	"system:role:list": "rbac:role:list", "system:role:create": "rbac:role:create",
	"system:role:update": "rbac:role:update", "system:role:status": "rbac:role:status",
	"system:role:default": "rbac:role:default", "system:role:delete": "rbac:role:delete",
	"system:role:authorize": "rbac:role:authorize",
	"system:user:list":      "account:user:list", "system:user:update": "account:user:update",
	"system:user:status": "account:user:status", "system:user:delete": "account:user:delete",
	"system:user:roles":   "account:user:roles",
	"system:session:list": "auth:session:list", "system:session:revoke": "auth:session:revoke",
	"system:auth-platform:list": "auth:platform:list", "system:auth-platform:create": "auth:platform:create",
	"system:auth-platform:update": "auth:platform:update", "system:auth-platform:status": "auth:platform:status",
	"system:auth-platform:delete": "auth:platform:delete",
	"system:operation-log:list":   "audit:operation-log:list",
}

var legacyMenuNames = map[string]string{
	"system": "权限与认证", "system:menu:list": "菜单管理", "system:menu:create": "新增菜单",
	"system:menu:update": "修改菜单", "system:menu:delete": "删除菜单", "system:role:list": "角色管理",
	"system:role:create": "新增角色", "system:role:update": "修改角色", "system:role:status": "修改角色状态",
	"system:role:default": "设置默认角色", "system:role:delete": "删除角色", "system:role:authorize": "配置角色权限",
	"system:user:list": "用户管理", "system:user:update": "修改用户", "system:user:status": "修改用户状态",
	"system:user:delete": "删除用户", "system:user:roles": "分配用户角色", "system:session:list": "会话管理",
	"system:session:revoke": "踢出会话", "system:auth-platform:list": "认证平台",
	"system:auth-platform:create": "新增认证平台", "system:auth-platform:update": "编辑认证平台",
	"system:auth-platform:status": "变更认证平台状态", "system:auth-platform:delete": "删除认证平台",
	"system:operation-log:list": "操作日志",
}

var legacyMenuIcons = map[string]legacyIconTarget{
	"system":                    {Old: "Setting", New: "lucide:shield-check"},
	"system:menu:list":          {Old: "Menu", New: "lucide:panel-left"},
	"system:role:list":          {Old: "UserFilled", New: "lucide:user-cog"},
	"system:user:list":          {Old: "User", New: "lucide:user-round-cog"},
	"system:auth-platform:list": {Old: "Key", New: "lucide:key-round"},
	"system:session:list":       {Old: "List", New: "lucide:monitor-smartphone"},
	"system:operation-log:list": {Old: "List", New: "lucide:scroll-text"},
}

var migratedPages = map[string]migratedPageTarget{
	"account:user:list":        {ParentCode: "account", Path: "/account/users", ComponentPath: "account/users", I18nKey: "navigation.accountUsers", Icon: "lucide:user-round-cog", SortOrder: 10},
	"auth:session:list":        {ParentCode: "account", Path: "/account/sessions", ComponentPath: "account/sessions", I18nKey: "navigation.accountSessions", Icon: "lucide:monitor-smartphone", SortOrder: 20},
	"rbac:menu:list":           {ParentCode: "access", Path: "/access/menus", ComponentPath: "access/menus", I18nKey: "navigation.accessMenus", Icon: "lucide:panel-left", SortOrder: 10},
	"rbac:role:list":           {ParentCode: "access", Path: "/access/roles", ComponentPath: "access/roles", I18nKey: "navigation.accessRoles", Icon: "lucide:user-cog", SortOrder: 20},
	"auth:platform:list":       {ParentCode: "access", Path: "/access/auth-platforms", ComponentPath: "access/auth-platforms", I18nKey: "navigation.accessAuthPlatforms", Icon: "lucide:key-round", SortOrder: 30},
	"audit:operation-log:list": {ParentCode: "system", Path: "/system/operation-logs", ComponentPath: "system/operation-logs", I18nKey: "navigation.systemOperationLogs", Icon: "lucide:scroll-text", SortOrder: 10},
}

func PrepareSchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("prepare menu schema requires a database")
	}
	db = db.WithContext(ctx)
	var exists bool
	if err := db.Raw(`SELECT to_regclass(current_schema() || '.rbac_menu') IS NOT NULL`).Scan(&exists).Error; err != nil {
		return fmt.Errorf("inspect menu table: %w", err)
	}
	if !exists {
		return nil
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, statement := range []string{
			`ALTER TABLE rbac_menu ADD COLUMN IF NOT EXISTS name VARCHAR(128)`,
			`ALTER TABLE rbac_menu ALTER COLUMN i18n_key DROP NOT NULL`,
		} {
			if err := tx.Exec(statement).Error; err != nil {
				return err
			}
		}
		if err := prepareLegacyMenuCatalog(tx); err != nil {
			return err
		}
		var invalidNames int64
		if err := tx.Raw(`SELECT count(*) FROM rbac_menu WHERE name IS NULL OR btrim(name) = ''`).Scan(&invalidNames).Error; err != nil {
			return fmt.Errorf("inspect menu names: %w", err)
		}
		if invalidNames != 0 {
			return fmt.Errorf("menu catalog contains %d rows without a name", invalidNames)
		}
		if err := tx.Exec(`ALTER TABLE rbac_menu ALTER COLUMN name SET NOT NULL`).Error; err != nil {
			return fmt.Errorf("require menu name: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("prepare menu schema: %w", err)
	}
	return nil
}

func PreparePlatformSchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("prepare menu platform schema requires a database")
	}
	db = db.WithContext(ctx)
	var exists bool
	if err := db.Raw(`SELECT to_regclass(current_schema() || '.rbac_menu') IS NOT NULL`).Scan(&exists).Error; err != nil {
		return fmt.Errorf("inspect menu table for platform migration: %w", err)
	}
	if !exists {
		return nil
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`LOCK TABLE auth_platform IN SHARE ROW EXCLUSIVE MODE`).Error; err != nil {
			return fmt.Errorf("lock authentication platforms for menu migration: %w", err)
		}
		if err := tx.Exec(`LOCK TABLE rbac_menu IN SHARE ROW EXCLUSIVE MODE`).Error; err != nil {
			return fmt.Errorf("lock menus for platform migration: %w", err)
		}
		adminIDs := make([]int64, 0, 2)
		if err := tx.Raw(`
			SELECT id
			FROM auth_platform
			WHERE code = ? AND is_builtin = 1 AND deleted_at IS NULL
			ORDER BY id
			LIMIT 2`, authplatform.BuiltinAdminCode).Scan(&adminIDs).Error; err != nil {
			return fmt.Errorf("find builtin Admin platform for menu migration: %w", err)
		}
		if len(adminIDs) != 1 || adminIDs[0] < 1 {
			return fmt.Errorf("menu platform migration requires exactly one builtin Admin platform")
		}
		if err := tx.Exec(`ALTER TABLE rbac_menu ADD COLUMN IF NOT EXISTS platform_id BIGINT`).Error; err != nil {
			return fmt.Errorf("add menu platform column: %w", err)
		}
		if err := tx.Exec(`UPDATE rbac_menu SET platform_id = ? WHERE platform_id IS NULL`, adminIDs[0]).Error; err != nil {
			return fmt.Errorf("backfill menu platform column: %w", err)
		}
		if err := tx.Exec(`ALTER TABLE rbac_menu ALTER COLUMN platform_id SET NOT NULL`).Error; err != nil {
			return fmt.Errorf("require menu platform column: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("prepare menu platform schema: %w", err)
	}
	return nil
}

func prepareLegacyMenuCatalog(db *gorm.DB) error {
	var accessExists bool
	if err := db.Raw(`SELECT EXISTS (SELECT 1 FROM rbac_menu WHERE code = 'access')`).Scan(&accessExists).Error; err != nil {
		return fmt.Errorf("inspect current access menu root: %w", err)
	}
	if accessExists {
		var oldCodes []string
		if err := db.Raw(`SELECT code FROM rbac_menu WHERE code LIKE 'system:%' ORDER BY code`).Scan(&oldCodes).Error; err != nil {
			return fmt.Errorf("inspect legacy menu codes: %w", err)
		}
		if len(oldCodes) != 0 {
			return fmt.Errorf("menu catalog mixes legacy and current codes: %s", strings.Join(oldCodes, ", "))
		}
		result := db.Exec(`
			UPDATE rbac_menu
			SET i18n_key = 'navigation.access', updated_at = CURRENT_TIMESTAMP
			WHERE code = 'access'
			  AND menu_type = 'directory'
			  AND i18n_key = 'lucide:shield-check'
			  AND icon = 'lucide:shield-check'
			  AND deleted_at IS NULL`)
		if result.Error != nil {
			return fmt.Errorf("repair corrupted access root i18n key: %w", result.Error)
		}
		if result.RowsAffected > 1 {
			return fmt.Errorf("repair corrupted access root i18n key affected %d rows", result.RowsAffected)
		}
		return nil
	}

	var rows []legacyMigrationRow
	viewExpression, err := legacyMenuViewExpression(db)
	if err != nil {
		return err
	}
	legacyRowsQuery := `
		SELECT id, menu_type, code, ` + viewExpression + ` AS view_key, icon, sort_order
		FROM rbac_menu
		WHERE code = 'system' OR code LIKE 'system:%'
		ORDER BY id`
	if err := db.Raw(legacyRowsQuery).Scan(&rows).Error; err != nil {
		return fmt.Errorf("read legacy menu catalog: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}

	rowByCode := make(map[string]legacyMigrationRow, len(rows))
	unknown := make([]string, 0)
	for _, row := range rows {
		if _, duplicate := rowByCode[row.Code]; duplicate {
			return fmt.Errorf("legacy menu code %s is duplicated", row.Code)
		}
		rowByCode[row.Code] = row
		if _, known := legacyMenuNames[row.Code]; !known {
			unknown = append(unknown, row.Code)
		}
	}
	missing := make([]string, 0)
	for code := range legacyMenuNames {
		if _, exists := rowByCode[code]; !exists {
			missing = append(missing, code)
		}
	}
	sort.Strings(unknown)
	sort.Strings(missing)
	if len(unknown) != 0 || len(missing) != 0 {
		return fmt.Errorf("legacy menu catalog is incomplete: unknown=[%s] missing=[%s]", strings.Join(unknown, ", "), strings.Join(missing, ", "))
	}

	for _, row := range rows {
		iconTarget, hasIcon := legacyMenuIcons[row.Code]
		if hasIcon {
			if row.Icon == nil || *row.Icon != iconTarget.Old {
				return fmt.Errorf("legacy menu %s icon is %q, want %q", row.Code, pointerString(row.Icon), iconTarget.Old)
			}
		} else if row.Icon != nil {
			return fmt.Errorf("legacy menu %s has unsupported icon %q", row.Code, *row.Icon)
		}
		if row.MenuType == TypePage {
			if !isLegacyComponentPath(pointerString(row.ViewKey)) {
				return fmt.Errorf("legacy menu %s view_key %q has no component path mapping", row.Code, pointerString(row.ViewKey))
			}
		}
	}

	for _, targetCode := range append([]string{"account", "access"}, currentPermissionCodes()...) {
		var occupied bool
		if err := db.Raw(`SELECT EXISTS (SELECT 1 FROM rbac_menu WHERE code = ?)`, targetCode).Scan(&occupied).Error; err != nil {
			return fmt.Errorf("inspect target menu code %s: %w", targetCode, err)
		}
		if occupied {
			return fmt.Errorf("target menu code %s is already occupied", targetCode)
		}
	}

	if err := db.Exec(`
		UPDATE rbac_role_menu AS role_menu
		SET deleted_at = NULL, updated_at = CURRENT_TIMESTAMP
		FROM rbac_menu AS menu
		WHERE role_menu.menu_id = menu.id
		  AND menu.code IN ('system:menu:list', 'system:menu:create', 'system:menu:update', 'system:menu:delete')
		  AND menu.deleted_at IS NOT NULL
		  AND role_menu.deleted_at = menu.deleted_at`).Error; err != nil {
		return fmt.Errorf("restore protected menu grants: %w", err)
	}

	for _, row := range rows {
		temporaryCode := fmt.Sprintf("migration:menu-%d", row.ID)
		if err := db.Exec(`UPDATE rbac_menu SET code = ? WHERE id = ?`, temporaryCode, row.ID).Error; err != nil {
			return fmt.Errorf("reserve legacy menu code %s: %w", row.Code, err)
		}
	}

	nowExpression := gorm.Expr("CURRENT_TIMESTAMP")
	for _, root := range []struct {
		Name      string
		Code      string
		I18nKey   string
		Icon      string
		SortOrder int
	}{
		{Name: "用户与账号", Code: "account", I18nKey: "navigation.account", Icon: "lucide:users-round", SortOrder: 100},
		{Name: "系统管理", Code: "system", I18nKey: "navigation.system", Icon: "lucide:settings-2", SortOrder: 300},
	} {
		result := db.Exec(`
			INSERT INTO rbac_menu
				(parent_id, menu_type, name, code, i18n_key, path, component_path, icon,
				 sort_order, is_enabled, is_hidden, created_at, updated_at)
			VALUES (NULL, 'directory', ?, ?, ?, NULL, NULL, ?, ?, 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			root.Name, root.Code, root.I18nKey, root.Icon, root.SortOrder)
		if result.Error != nil {
			return fmt.Errorf("create %s menu root: %w", root.Code, result.Error)
		}
	}

	parentIDs := map[string]int64{"access": rowByCode["system"].ID}
	for _, code := range []string{"account", "system"} {
		var id int64
		if err := db.Raw(`SELECT id FROM rbac_menu WHERE code = ?`, code).Scan(&id).Error; err != nil || id < 1 {
			return fmt.Errorf("resolve %s menu root: %w", code, err)
		}
		parentIDs[code] = id
	}
	for oldCode, newCode := range legacyPermissionCodes {
		if target, page := migratedPages[newCode]; page {
			parentIDs[newCode] = rowByCode[oldCode].ID
			_ = target
		}
	}

	for _, row := range rows {
		name := legacyMenuNames[row.Code]
		newCode := "access"
		var parentID *int64
		var i18nKey *string
		var path *string
		var componentPath *string
		var icon *string
		sortOrder := row.SortOrder
		isHidden := yesno.Yes
		if row.Code == "system" {
			i18nValue := "navigation.access"
			iconValue := legacyMenuIcons[row.Code].New
			i18nKey = &i18nValue
			icon = &iconValue
			sortOrder = 200
			isHidden = yesno.No
		} else {
			newCode = legacyPermissionCodes[row.Code]
			if page, exists := migratedPages[newCode]; exists {
				value := parentIDs[page.ParentCode]
				parentID = &value
				i18nValue, pathValue, componentValue, iconValue := page.I18nKey, page.Path, page.ComponentPath, page.Icon
				i18nKey, path, componentPath, icon = &i18nValue, &pathValue, &componentValue, &iconValue
				sortOrder = page.SortOrder
				isHidden = yesno.No
			} else {
				legacyParentCode := legacyActionParent(row.Code)
				value := parentIDs[legacyPermissionCodes[legacyParentCode]]
				parentID = &value
			}
		}
		result := db.Exec(`
			UPDATE rbac_menu
			SET parent_id = ?, name = ?, code = ?, i18n_key = ?, path = ?, component_path = ?, icon = ?,
			    sort_order = ?, is_hidden = ?,
			    deleted_at = CASE WHEN id IN (?, ?, ?, ?) THEN NULL ELSE deleted_at END,
			    updated_at = ?
			WHERE id = ?`, parentID, name, newCode, i18nKey, path, componentPath, icon,
			sortOrder, isHidden, rowByCode["system:menu:list"].ID, rowByCode["system:menu:create"].ID,
			rowByCode["system:menu:update"].ID, rowByCode["system:menu:delete"].ID, nowExpression, row.ID)
		if result.Error != nil {
			return fmt.Errorf("rekey legacy menu %s: %w", row.Code, result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("rekey legacy menu %s affected %d rows", row.Code, result.RowsAffected)
		}
	}

	if err := db.Exec(`ALTER TABLE rbac_menu DROP COLUMN IF EXISTS view_key`).Error; err != nil {
		return fmt.Errorf("drop legacy menu view_key: %w", err)
	}
	if err := db.Exec(`UPDATE rbac_access_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP`).Error; err != nil {
		return fmt.Errorf("advance access versions after menu rekey: %w", err)
	}
	return nil
}

func isLegacyComponentPath(value string) bool {
	if _, exists := legacyComponentPaths[value]; exists {
		return true
	}
	for _, path := range legacyComponentPaths {
		if path == value {
			return true
		}
	}
	return false
}

func legacyMenuViewExpression(db *gorm.DB) (string, error) {
	var hasViewKey, hasComponentPath bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = 'rbac_menu' AND column_name = 'view_key'
		)`).Scan(&hasViewKey).Error; err != nil {
		return "", fmt.Errorf("inspect legacy menu view_key column: %w", err)
	}
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = 'rbac_menu' AND column_name = 'component_path'
		)`).Scan(&hasComponentPath).Error; err != nil {
		return "", fmt.Errorf("inspect legacy menu component_path column: %w", err)
	}
	switch {
	case hasViewKey && hasComponentPath:
		return "COALESCE(view_key, component_path)", nil
	case hasViewKey:
		return "view_key", nil
	case hasComponentPath:
		return "component_path", nil
	default:
		return "", fmt.Errorf("legacy menu catalog requires view_key or component_path")
	}
}

func currentPermissionCodes() []string {
	codes := make([]string, 0, len(legacyPermissionCodes))
	for _, code := range legacyPermissionCodes {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func legacyActionParent(code string) string {
	lastSeparator := strings.LastIndex(code, ":")
	if lastSeparator < 0 {
		return ""
	}
	return code[:lastSeparator] + ":list"
}

func pointerString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
