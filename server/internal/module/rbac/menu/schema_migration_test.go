package menu_test

import (
	"errors"
	"testing"
	"time"

	"admin/server/internal/module/authplatform"
	"admin/server/internal/module/rbac/menu"
	"admin/server/internal/module/rbac/role"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

func TestMenuEnsureSchemaPreservesActiveCatalogAndGrants(t *testing.T) {
	connection, ctx := openMenuSchema(t)
	db := connection.GORM.WithContext(ctx)
	adminPlatformID := menuAdminPlatformID(t, db)

	createdRole := role.Role{Code: "schema_keeper", Name: "Schema Keeper", IsEnabled: yesno.Yes}
	if err := db.Create(&createdRole).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	accessI18nKey := "navigation.access"
	root := menu.Menu{
		PlatformID: adminPlatformID, MenuType: menu.TypeDirectory, Name: "自定义权限中心", Code: "access",
		I18nKey: &accessI18nKey, SortOrder: 210, IsEnabled: yesno.Yes, IsHidden: yesno.No,
	}
	if err := db.Create(&root).Error; err != nil {
		t.Fatalf("create access root: %v", err)
	}
	pageI18nKey := "navigation.accessMenus"
	path, componentPath := "/access/menus", "access/menus"
	page := menu.Menu{
		PlatformID: adminPlatformID, ParentID: &root.ID, MenuType: menu.TypePage, Name: "自定义菜单管理", Code: menu.PermissionList,
		I18nKey: &pageI18nKey, Path: &path, ComponentPath: &componentPath,
		SortOrder: 11, IsEnabled: yesno.Yes, IsHidden: yesno.No,
	}
	if err := db.Create(&page).Error; err != nil {
		t.Fatalf("create menu page: %v", err)
	}
	action := menu.Menu{
		PlatformID: adminPlatformID, ParentID: &page.ID, MenuType: menu.TypeAction, Name: "自定义新增菜单", Code: menu.PermissionCreate,
		SortOrder: 12, IsEnabled: yesno.Yes, IsHidden: yesno.Yes,
	}
	if err := db.Create(&action).Error; err != nil {
		t.Fatalf("create menu action: %v", err)
	}
	grant := menu.RoleMenu{RoleID: createdRole.ID, MenuID: action.ID}
	if err := db.Create(&grant).Error; err != nil {
		t.Fatalf("create role menu grant: %v", err)
	}
	if err := db.Exec(`ALTER TABLE rbac_menu ADD COLUMN view_key VARCHAR(128) NULL`).Error; err != nil {
		t.Fatalf("add obsolete view_key column: %v", err)
	}

	if err := menu.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("repeat EnsureSchema: %v", err)
	}

	var storedRoot, storedPage, storedAction menu.Menu
	for _, target := range []struct {
		id   int64
		item *menu.Menu
	}{{root.ID, &storedRoot}, {page.ID, &storedPage}, {action.ID, &storedAction}} {
		if err := db.First(target.item, target.id).Error; err != nil {
			t.Fatalf("read active menu %d: %v", target.id, err)
		}
	}
	if storedRoot.Name != root.Name || storedRoot.SortOrder != root.SortOrder ||
		storedPage.Name != page.Name || storedPage.SortOrder != page.SortOrder ||
		storedAction.Name != action.Name || storedAction.SortOrder != action.SortOrder {
		t.Fatalf("EnsureSchema overwrote presentation fields: root=%+v page=%+v action=%+v", storedRoot, storedPage, storedAction)
	}
	var activeGrantCount int64
	if err := db.Model(&menu.RoleMenu{}).Where("id = ?", grant.ID).Count(&activeGrantCount).Error; err != nil {
		t.Fatalf("count active grant: %v", err)
	}
	if activeGrantCount != 1 {
		t.Fatalf("active grant count = %d, want 1", activeGrantCount)
	}
}

func TestPreparePlatformSchemaBackfillsHistoryAndPreservesGrants(t *testing.T) {
	connection, ctx := openMenuSchema(t)
	db := connection.GORM.WithContext(ctx)
	adminPlatformID := menuAdminPlatformID(t, db)

	for _, statement := range []string{
		`DROP INDEX IF EXISTS ux_rbac_menu_code_active`,
		`DROP INDEX IF EXISTS ux_rbac_menu_page_path_active`,
		`DROP INDEX IF EXISTS ix_rbac_menu_parent_active`,
		`ALTER TABLE rbac_menu DROP CONSTRAINT IF EXISTS fk_rbac_menu_parent_platform`,
		`ALTER TABLE rbac_menu DROP CONSTRAINT IF EXISTS uq_rbac_menu_id_platform`,
		`ALTER TABLE rbac_menu DROP CONSTRAINT IF EXISTS fk_rbac_menu_platform`,
		`ALTER TABLE rbac_menu DROP COLUMN platform_id`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("prepare old menu schema with %q: %v", statement, err)
		}
	}

	createdRole := role.Role{Code: "platform_history", Name: "Platform history", IsEnabled: yesno.Yes}
	if err := db.Create(&createdRole).Error; err != nil {
		t.Fatal(err)
	}
	var activeID, deletedID int64
	if err := db.Raw(`
		INSERT INTO rbac_menu
			(parent_id, menu_type, name, code, i18n_key, path, component_path, icon, sort_order, is_enabled, is_hidden, created_at, updated_at)
		VALUES
			(NULL, 'page', 'Old active', 'old:active', 'navigation.oldActive', '/old-active', 'old-active', NULL, 1, 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id`).Scan(&activeID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Raw(`
		INSERT INTO rbac_menu
			(parent_id, menu_type, name, code, i18n_key, path, component_path, icon, sort_order, is_enabled, is_hidden, created_at, updated_at, deleted_at)
		VALUES
			(NULL, 'page', 'Old deleted', 'old:deleted', 'navigation.oldDeleted', '/old-deleted', 'old-deleted', NULL, 2, 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id`).Scan(&deletedID).Error; err != nil {
		t.Fatal(err)
	}
	grant := menu.RoleMenu{RoleID: createdRole.ID, MenuID: activeID}
	if err := db.Create(&grant).Error; err != nil {
		t.Fatal(err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		if err := menu.PreparePlatformSchema(ctx, connection.GORM); err != nil {
			t.Fatalf("PreparePlatformSchema attempt %d: %v", attempt+1, err)
		}
		if err := menu.EnsureSchema(ctx, connection.GORM); err != nil {
			t.Fatalf("EnsureSchema attempt %d: %v", attempt+1, err)
		}
	}

	var rows []struct {
		ID         int64
		PlatformID int64
	}
	if err := db.Unscoped().Table("rbac_menu").Select("id, platform_id").Where("id IN ?", []int64{activeID, deletedID}).Order("id").Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].PlatformID != adminPlatformID || rows[1].PlatformID != adminPlatformID {
		t.Fatalf("backfilled rows = %+v, admin platform = %d", rows, adminPlatformID)
	}
	var storedGrant menu.RoleMenu
	if err := db.First(&storedGrant, grant.ID).Error; err != nil || storedGrant.MenuID != activeID {
		t.Fatalf("stored grant = %+v, err=%v", storedGrant, err)
	}
}

func TestMenuSchemaRejectsCrossPlatformParent(t *testing.T) {
	connection, ctx := openMenuSchema(t)
	db := connection.GORM.WithContext(ctx)
	adminPlatformID := menuAdminPlatformID(t, db)
	admin := authplatform.Platform{}
	if err := db.First(&admin, adminPlatformID).Error; err != nil {
		t.Fatal(err)
	}
	canvas := admin
	canvas.ID = 0
	canvas.Code = "canvas"
	canvas.Name = "Canvas"
	canvas.IsBuiltin = yesno.No
	canvas.AllowRegister = yesno.Yes
	canvas.CreatedAt = time.Now().UTC()
	canvas.UpdatedAt = canvas.CreatedAt
	canvas.DeletedAt = gorm.DeletedAt{}
	if err := db.Create(&canvas).Error; err != nil {
		t.Fatal(err)
	}

	i18nKey, path, componentPath := "navigation.test", "/test", "test"
	parent := menu.Menu{PlatformID: adminPlatformID, MenuType: menu.TypePage, Name: "Admin test", Code: "test", I18nKey: &i18nKey, Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	child := menu.Menu{PlatformID: canvas.ID, ParentID: &parent.ID, MenuType: menu.TypeAction, Name: "Canvas button", Code: "test:button", IsEnabled: yesno.Yes, IsHidden: yesno.Yes}
	err := db.Create(&child).Error
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) || postgresError.ConstraintName != "fk_rbac_menu_parent_platform" {
		t.Fatalf("cross-platform parent error = %v", err)
	}
}

func menuAdminPlatformID(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var id int64
	if err := db.Raw(`SELECT id FROM auth_platform WHERE code = 'admin' AND deleted_at IS NULL`).Scan(&id).Error; err != nil || id < 1 {
		t.Fatalf("find Admin platform id: id=%d err=%v", id, err)
	}
	return id
}
