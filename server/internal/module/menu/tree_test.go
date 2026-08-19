package menu

import (
	"testing"
	"time"

	"admin/server/internal/shared/yesno"
)

func TestBuildMenuIndexBuildsStableTreeWithNonNilLeafChildren(t *testing.T) {
	now := time.Now().UTC()
	pagePath := "/system/menus"
	viewKey := "system-menus"
	menus := []Menu{
		{ID: 4, ParentID: int64Pointer(2), MenuType: TypeAction, Code: PermissionCreate, I18nKey: "permission.menuCreate", SortOrder: 10, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now},
		{ID: 3, ParentID: int64Pointer(2), MenuType: TypeAction, Code: PermissionDelete, I18nKey: "permission.menuDelete", SortOrder: 20, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now},
		{ID: 2, ParentID: int64Pointer(1), MenuType: TypePage, Code: PermissionList, I18nKey: "navigation.systemMenus", Path: &pagePath, ViewKey: &viewKey, SortOrder: 10, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now},
		{ID: 5, MenuType: TypeDirectory, Code: "reports", I18nKey: "navigation.system", SortOrder: 200, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now},
		{ID: 1, MenuType: TypeDirectory, Code: BuiltinSystemCode, I18nKey: "navigation.system", SortOrder: 100, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now},
	}

	index, err := buildMenuIndex(menus)
	if err != nil {
		t.Fatal(err)
	}
	if err := index.validateEnabledAncestors(); err != nil {
		t.Fatal(err)
	}
	tree, err := index.buildManagedTree()
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 2 || tree[0].ID != 1 || tree[1].ID != 5 {
		t.Fatalf("roots = %+v", tree)
	}
	if len(tree[0].Children) != 1 || len(tree[0].Children[0].Children) != 2 {
		t.Fatalf("system tree = %+v", tree[0])
	}
	if tree[0].Children[0].Children[0].ID != 4 || tree[0].Children[0].Children[1].ID != 3 {
		t.Fatalf("sorted actions = %+v", tree[0].Children[0].Children)
	}
	if tree[0].Children[0].Children[0].Children == nil || tree[1].Children == nil {
		t.Fatal("leaf children must be a non-nil empty slice")
	}
	if !tree[0].IsBuiltin || tree[1].IsBuiltin {
		t.Fatal("computed builtin flags are incorrect")
	}

	descendants, err := index.descendants(1)
	if err != nil || len(descendants) != 3 {
		t.Fatalf("descendants(1) = %v,%v", descendants, err)
	}
	ancestors, err := index.ancestors(4)
	if err != nil || len(ancestors) != 2 || ancestors[0] != 2 || ancestors[1] != 1 {
		t.Fatalf("ancestors(4) = %v,%v", ancestors, err)
	}
}

func TestBuildMenuIndexRejectsInvalidStoredTrees(t *testing.T) {
	now := time.Now().UTC()
	validRoot := Menu{ID: 1, MenuType: TypeDirectory, Code: "reports", I18nKey: "navigation.system", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	pagePath := "/system/menus"
	viewKey := "system-menus"
	validPage := Menu{ID: 2, ParentID: int64Pointer(1), MenuType: TypePage, Code: "reports:list", I18nKey: "navigation.systemMenus", Path: &pagePath, ViewKey: &viewKey, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	tests := []struct {
		name  string
		menus []Menu
	}{
		{name: "duplicate id", menus: []Menu{validRoot, validRoot}},
		{name: "orphan", menus: []Menu{{ID: 2, ParentID: int64Pointer(99), MenuType: TypeDirectory, Code: "reports", I18nKey: "navigation.system", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}}},
		{name: "root page", menus: []Menu{{ID: 1, MenuType: TypePage, Code: "reports:list", I18nKey: "navigation.systemMenus", Path: &pagePath, ViewKey: &viewKey, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}}},
		{name: "action under directory", menus: []Menu{validRoot, {ID: 2, ParentID: int64Pointer(1), MenuType: TypeAction, Code: "reports:create", I18nKey: "permission.menuCreate", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}}},
		{name: "action has child", menus: []Menu{validRoot, validPage, {ID: 3, ParentID: int64Pointer(2), MenuType: TypeAction, Code: "reports:create", I18nKey: "permission.menuCreate", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}, {ID: 4, ParentID: int64Pointer(3), MenuType: TypeAction, Code: "reports:update", I18nKey: "permission.menuUpdate", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}}},
		{name: "cycle", menus: []Menu{{ID: 1, ParentID: int64Pointer(2), MenuType: TypeDirectory, Code: "reports", I18nKey: "navigation.system", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}, {ID: 2, ParentID: int64Pointer(1), MenuType: TypeDirectory, Code: "settings", I18nKey: "navigation.system", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}}},
		{name: "unknown title", menus: []Menu{{ID: 1, MenuType: TypeDirectory, Code: "reports", I18nKey: "navigation.unknown", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}}},
		{name: "invalid action icon", menus: []Menu{validRoot, validPage, {ID: 3, ParentID: int64Pointer(2), MenuType: TypeAction, Code: "reports:create", I18nKey: "permission.menuCreate", Icon: stringPointer("Key"), IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := buildMenuIndex(test.menus); err == nil {
				t.Fatal("buildMenuIndex accepted an invalid tree")
			}
		})
	}
}

func TestMenuIndexRejectsEnabledNodeBelowDisabledAncestor(t *testing.T) {
	now := time.Now().UTC()
	menus := []Menu{
		{ID: 1, MenuType: TypeDirectory, Code: "reports", I18nKey: "navigation.system", IsEnabled: yesno.No, CreatedAt: now, UpdatedAt: now},
		{ID: 2, ParentID: int64Pointer(1), MenuType: TypeDirectory, Code: "reports:section", I18nKey: "navigation.system", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now},
	}
	index, err := buildMenuIndex(menus)
	if err != nil {
		t.Fatal(err)
	}
	if err := index.validateEnabledAncestors(); err == nil {
		t.Fatal("enabled child below disabled ancestor was accepted")
	}
}

func int64Pointer(value int64) *int64 {
	return &value
}
