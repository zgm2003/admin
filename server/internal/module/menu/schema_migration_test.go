package menu_test

import (
	"testing"

	"admin/server/internal/module/menu"
	"admin/server/internal/module/role"
	"admin/server/internal/shared/yesno"
)

func TestMenuEnsureSchemaPreservesActiveCatalogAndGrants(t *testing.T) {
	connection, ctx := openMenuSchema(t)
	db := connection.GORM.WithContext(ctx)

	createdRole := role.Role{Code: "schema_keeper", Name: "Schema Keeper", IsEnabled: yesno.Yes}
	if err := db.Create(&createdRole).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	accessI18nKey := "navigation.access"
	root := menu.Menu{
		MenuType: menu.TypeDirectory, Name: "自定义权限中心", Code: "access",
		I18nKey: &accessI18nKey, SortOrder: 210, IsEnabled: yesno.Yes, IsHidden: yesno.No,
	}
	if err := db.Create(&root).Error; err != nil {
		t.Fatalf("create access root: %v", err)
	}
	pageI18nKey := "navigation.accessMenus"
	path, componentPath := "/access/menus", "access/menus"
	page := menu.Menu{
		ParentID: &root.ID, MenuType: menu.TypePage, Name: "自定义菜单管理", Code: menu.PermissionList,
		I18nKey: &pageI18nKey, Path: &path, ComponentPath: &componentPath,
		SortOrder: 11, IsEnabled: yesno.Yes, IsHidden: yesno.No,
	}
	if err := db.Create(&page).Error; err != nil {
		t.Fatalf("create menu page: %v", err)
	}
	action := menu.Menu{
		ParentID: &page.ID, MenuType: menu.TypeAction, Name: "自定义新增菜单", Code: menu.PermissionCreate,
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
