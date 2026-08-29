package role

import (
	"reflect"
	"testing"

	"admin/server/internal/module/rbac/menu"
	"admin/server/internal/shared/yesno"
)

func TestPermissionIndexBuildsStableTreeAndNormalizesDirectGrants(t *testing.T) {
	rootID, pageID := int64(1), int64(2)
	rows := []menu.Menu{
		{ID: 4, PlatformID: 1, ParentID: &pageID, MenuType: menu.TypeAction, Name: "修改角色", Code: "rbac:role:update", SortOrder: 20, IsEnabled: yesno.Yes, IsHidden: yesno.Yes},
		{ID: rootID, PlatformID: 1, MenuType: menu.TypeDirectory, Name: "权限与认证", Code: "access", I18nKey: roleStringPointer("navigation.access"), SortOrder: 100, IsEnabled: yesno.Yes},
		{ID: 3, PlatformID: 1, ParentID: &pageID, MenuType: menu.TypeAction, Name: "新增角色", Code: "rbac:role:create", SortOrder: 10, IsEnabled: yesno.No, IsHidden: yesno.Yes},
		{ID: pageID, PlatformID: 1, ParentID: &rootID, MenuType: menu.TypePage, Name: "角色管理", Code: "rbac:role:list", I18nKey: roleStringPointer("navigation.accessRoles"), SortOrder: 20, IsEnabled: yesno.Yes},
	}
	index, err := buildPermissionIndex(rows)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := index.tree(1)
	if err != nil || len(tree) != 1 || len(tree[0].Children) != 1 || len(tree[0].Children[0].Children) != 2 || tree[0].Children[0].Children[0].ID != 3 {
		t.Fatalf("tree = %+v,%v", tree, err)
	}
	if tree[0].Name != "权限与认证" || tree[0].Children[0].Name != "角色管理" || tree[0].Children[0].Children[0].Name != "新增角色" {
		t.Fatalf("tree names = %+v", tree)
	}
	for _, test := range []struct {
		input []int64
		want  []int64
	}{
		{input: []int64{}, want: []int64{}},
		{input: []int64{2}, want: []int64{2}},
		{input: []int64{3}, want: []int64{3}},
		{input: []int64{2, 3}, want: []int64{3}},
		{input: []int64{2, 3, 4}, want: []int64{3, 4}},
	} {
		got, err := index.normalizeRequested(test.input)
		if err != nil || !reflect.DeepEqual(got, test.want) {
			t.Errorf("normalize(%v) = %v,%v, want %v", test.input, got, err, test.want)
		}
	}
	for _, invalid := range [][]int64{{1}, {2, 2}, {999}} {
		if _, err := index.normalizeRequested(invalid); err == nil {
			t.Errorf("normalize accepted %v", invalid)
		}
	}
}

func TestPermissionIndexSeparatesPlatformsAndSupportsRootPages(t *testing.T) {
	adminRootID, adminPageID, canvasPageID := int64(1), int64(2), int64(3)
	rows := []menu.Menu{
		{ID: adminRootID, PlatformID: 1, MenuType: menu.TypeDirectory, Name: "Admin", Code: "test", I18nKey: roleStringPointer("navigation.access"), IsEnabled: yesno.Yes},
		{ID: adminPageID, PlatformID: 1, ParentID: &adminRootID, MenuType: menu.TypePage, Name: "Admin Test", Code: "test:list", I18nKey: roleStringPointer("navigation.accessRoles"), IsEnabled: yesno.Yes},
		{ID: canvasPageID, PlatformID: 2, MenuType: menu.TypePage, Name: "Canvas Test", Code: "test", I18nKey: roleStringPointer("navigation.access"), IsEnabled: yesno.Yes},
		{ID: 4, PlatformID: 2, ParentID: &canvasPageID, MenuType: menu.TypeAction, Name: "Canvas Button", Code: "test:button", IsEnabled: yesno.Yes, IsHidden: yesno.Yes},
	}
	index, err := buildPermissionIndex(rows)
	if err != nil {
		t.Fatal(err)
	}
	adminTree, err := index.tree(1)
	if err != nil || len(adminTree) != 1 || adminTree[0].ID != adminRootID || len(adminTree[0].Children) != 1 {
		t.Fatalf("Admin tree = %+v,%v", adminTree, err)
	}
	canvasTree, err := index.tree(2)
	if err != nil || len(canvasTree) != 1 || canvasTree[0].ID != canvasPageID || len(canvasTree[0].Children) != 1 {
		t.Fatalf("Canvas tree = %+v,%v", canvasTree, err)
	}
	if normalized, err := index.normalizeRequested([]int64{adminPageID, 4}); err != nil || !reflect.DeepEqual(normalized, []int64{adminPageID, 4}) {
		t.Fatalf("mixed-platform grants = %v,%v", normalized, err)
	}
}

func TestPermissionIndexRejectsInvalidTreesAndStoredGrants(t *testing.T) {
	rootID := int64(1)
	valid := menu.Menu{ID: rootID, PlatformID: 1, MenuType: menu.TypeDirectory, Name: "权限与认证", Code: "access", I18nKey: roleStringPointer("navigation.access"), IsEnabled: yesno.Yes}
	for _, rows := range [][]menu.Menu{
		{valid, valid},
		{{ID: 1, PlatformID: 1, MenuType: menu.TypeAction, Name: "授权", Code: "rbac:role:authorize", IsEnabled: yesno.Yes, IsHidden: yesno.Yes}},
		{valid, {ID: 2, PlatformID: 1, ParentID: func() *int64 { value := int64(99); return &value }(), MenuType: menu.TypePage, Name: "角色管理", Code: "rbac:role:list", I18nKey: roleStringPointer("navigation.accessRoles"), IsEnabled: yesno.Yes}},
		{valid, {ID: 2, PlatformID: 2, ParentID: &rootID, MenuType: menu.TypePage, Name: "角色管理", Code: "rbac:role:list", I18nKey: roleStringPointer("navigation.accessRoles"), IsEnabled: yesno.Yes}},
	} {
		if _, err := buildPermissionIndex(rows); err == nil {
			t.Errorf("invalid tree accepted: %+v", rows)
		}
	}
	index, err := buildPermissionIndex([]menu.Menu{valid})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := index.validateStored([]menu.RoleMenu{{ID: 1, MenuID: rootID}}); err == nil {
		t.Fatal("directory stored grant accepted")
	}
}

func roleStringPointer(value string) *string {
	return &value
}
