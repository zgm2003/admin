package menu

import (
	"testing"
	"time"

	"admin/server/internal/shared/yesno"
)

func TestBuildMenuIndexBuildsStableTreeWithNonNilLeafChildren(t *testing.T) {
	now := time.Now().UTC()
	pagePath := "/system/roles"
	componentPath := "system/roles"
	menus := []Menu{
		{ID: 4, PlatformID: 1, ParentID: int64Pointer(2), MenuType: TypeAction, Name: "修改角色", Code: "permission:role:update", SortOrder: 10, IsEnabled: yesno.Yes, IsHidden: yesno.Yes, CreatedAt: now, UpdatedAt: now},
		{ID: 3, PlatformID: 1, ParentID: int64Pointer(2), MenuType: TypeAction, Name: "删除角色", Code: "permission:role:delete", SortOrder: 20, IsEnabled: yesno.Yes, IsHidden: yesno.Yes, CreatedAt: now, UpdatedAt: now},
		{ID: 2, PlatformID: 1, ParentID: int64Pointer(1), MenuType: TypePage, Name: "角色管理", Code: "permission:role:view", I18nKey: stringPointer("navigation.accessRoles"), Path: &pagePath, ComponentPath: &componentPath, SortOrder: 10, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now},
		{ID: 5, PlatformID: 1, MenuType: TypeDirectory, Name: "报表", Code: "reports", I18nKey: stringPointer("navigation.system"), SortOrder: 200, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now},
		{ID: 1, PlatformID: 1, MenuType: TypeDirectory, Name: "系统管理", Code: "system", I18nKey: stringPointer("navigation.system"), SortOrder: 100, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now},
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
	if tree[0].Children[0].ComponentPath == nil || *tree[0].Children[0].ComponentPath != componentPath {
		t.Fatal("component path was not preserved")
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

func TestNormalizeMenuInputEnforcesPageAndActionPermissionSuffixes(t *testing.T) {
	pagePath := "/reports"
	componentPath := "reports"
	page := CreateInput{PlatformID: 1, MenuType: TypePage, Name: "报表", Code: "reports:list", I18nKey: stringPointer("navigation.system"), Path: &pagePath, ComponentPath: &componentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No}
	if _, err := normalizeCreateInput(page); err == nil {
		t.Fatal("page permission without :view suffix was accepted")
	}
	action := CreateInput{PlatformID: 1, MenuType: TypeAction, Name: "查看", Code: "reports:view", IsEnabled: yesno.Yes, IsHidden: yesno.Yes}
	if _, err := normalizeCreateInput(action); err == nil {
		t.Fatal("action permission with :view suffix was accepted")
	}
	validPage := page
	validPage.Code = "reports:view"
	if _, err := normalizeCreateInput(validPage); err != nil {
		t.Fatalf("page permission with :view suffix rejected: %v", err)
	}
	validAction := action
	validAction.Code = "reports:export"
	if _, err := normalizeCreateInput(validAction); err != nil {
		t.Fatalf("action permission with non-view suffix rejected: %v", err)
	}
}

func TestNormalizeMenuInputRejectsMismatchedPagePathAndComponentPath(t *testing.T) {
	path := "/permission/roles"
	componentPath := "access/roles"
	input := CreateInput{PlatformID: 1, MenuType: TypePage, Name: "角色管理", Code: "permission:role:view", I18nKey: stringPointer("navigation.roles"), Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No}
	if _, err := normalizeCreateInput(input); err == nil {
		t.Fatal("mismatched page path/componentPath was accepted")
	}
	validPath := "/permission/roles"
	validComponent := "permission/roles"
	update := UpdateInput{MenuType: TypePage, Name: "角色管理", I18nKey: stringPointer("navigation.roles"), Path: &validPath, ComponentPath: &validComponent, IsHidden: yesno.No}
	if _, err := normalizeUpdateInput(update); err != nil {
		t.Fatalf("matching page path/componentPath rejected: %v", err)
	}
}

func TestBuildMenuIndexRejectsInvalidStoredTrees(t *testing.T) {
	now := time.Now().UTC()
	validRoot := Menu{ID: 1, PlatformID: 1, MenuType: TypeDirectory, Name: "报表", Code: "reports", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	pagePath := "/reports"
	componentPath := "reports"
	validPage := Menu{ID: 2, PlatformID: 1, ParentID: int64Pointer(1), MenuType: TypePage, Name: "报表列表", Code: "reports:view", I18nKey: stringPointer("reports.list"), Path: &pagePath, ComponentPath: &componentPath, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	invalidPageCode := validPage
	invalidPageCode.Code = "reports:list"
	invalidPagePath := validPage
	invalidPath := "/access/roles"
	invalidPagePath.Path = &invalidPath
	tests := []struct {
		name  string
		menus []Menu
	}{
		{name: "duplicate id", menus: []Menu{validRoot, validRoot}},
		{name: "missing platform", menus: []Menu{{ID: 1, MenuType: TypeDirectory, Name: "报表", Code: "reports", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}}},
		{name: "orphan", menus: []Menu{{ID: 2, PlatformID: 1, ParentID: int64Pointer(99), MenuType: TypeDirectory, Name: "报表", Code: "reports", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}}},
		{name: "root action", menus: []Menu{{ID: 1, PlatformID: 1, MenuType: TypeAction, Name: "新增报表", Code: "reports:create", IsEnabled: yesno.Yes, IsHidden: yesno.Yes, CreatedAt: now, UpdatedAt: now}}},
		{name: "action under directory", menus: []Menu{validRoot, {ID: 2, PlatformID: 1, ParentID: int64Pointer(1), MenuType: TypeAction, Name: "新增报表", Code: "reports:create", IsEnabled: yesno.Yes, IsHidden: yesno.Yes, CreatedAt: now, UpdatedAt: now}}},
		{name: "action has child", menus: []Menu{validRoot, validPage, {ID: 3, PlatformID: 1, ParentID: int64Pointer(2), MenuType: TypeAction, Name: "新增报表", Code: "reports:create", IsEnabled: yesno.Yes, IsHidden: yesno.Yes, CreatedAt: now, UpdatedAt: now}, {ID: 4, PlatformID: 1, ParentID: int64Pointer(3), MenuType: TypeAction, Name: "修改报表", Code: "reports:update", IsEnabled: yesno.Yes, IsHidden: yesno.Yes, CreatedAt: now, UpdatedAt: now}}},
		{name: "cycle", menus: []Menu{{ID: 1, PlatformID: 1, ParentID: int64Pointer(2), MenuType: TypeDirectory, Name: "报表", Code: "reports", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}, {ID: 2, PlatformID: 1, ParentID: int64Pointer(1), MenuType: TypeDirectory, Name: "设置", Code: "settings", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}}},
		{name: "malformed title", menus: []Menu{{ID: 1, PlatformID: 1, MenuType: TypeDirectory, Name: "报表", Code: "reports", I18nKey: stringPointer("navigation_unknown"), IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}}},
		{name: "invalid action icon", menus: []Menu{validRoot, validPage, {ID: 3, PlatformID: 1, ParentID: int64Pointer(2), MenuType: TypeAction, Name: "新增报表", Code: "reports:create", Icon: stringPointer("lucide:key-round"), IsEnabled: yesno.Yes, IsHidden: yesno.Yes, CreatedAt: now, UpdatedAt: now}}},
		{name: "page permission suffix", menus: []Menu{validRoot, invalidPageCode}},
		{name: "page path component mismatch", menus: []Menu{validRoot, invalidPagePath}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := buildMenuIndex(test.menus); err == nil {
				t.Fatal("buildMenuIndex accepted an invalid tree")
			}
		})
	}
}

func TestBuildMenuIndexSupportsRootPagesAndPlatformScopedUniqueness(t *testing.T) {
	now := time.Now().UTC()
	path := "/test"
	componentPath := "test"
	menus := []Menu{
		{ID: 1, PlatformID: 1, MenuType: TypePage, Name: "Admin test", Code: "test:view", I18nKey: stringPointer("navigation.system"), Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now},
		{ID: 2, PlatformID: 2, MenuType: TypePage, Name: "Canvas test", Code: "test:view", I18nKey: stringPointer("navigation.system"), Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now},
		{ID: 3, PlatformID: 2, ParentID: int64Pointer(2), MenuType: TypeAction, Name: "Canvas button", Code: "test:button", IsEnabled: yesno.Yes, IsHidden: yesno.Yes, CreatedAt: now, UpdatedAt: now},
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
	if len(tree) != 2 || tree[0].ID != 1 || tree[1].ID != 2 || len(tree[1].Children) != 1 || tree[1].Children[0].ID != 3 {
		t.Fatalf("platform root pages = %+v", tree)
	}
}

func TestBuildMenuIndexRejectsPlatformConflicts(t *testing.T) {
	now := time.Now().UTC()
	path := "/test"
	componentPath := "test"
	root := Menu{ID: 1, PlatformID: 1, MenuType: TypePage, Name: "Admin test", Code: "test", I18nKey: stringPointer("navigation.system"), Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	tests := []struct {
		name  string
		menus []Menu
	}{
		{name: "cross-platform parent", menus: []Menu{root, {ID: 2, PlatformID: 2, ParentID: int64Pointer(1), MenuType: TypeAction, Name: "Canvas button", Code: "test:button", IsEnabled: yesno.Yes, IsHidden: yesno.Yes, CreatedAt: now, UpdatedAt: now}}},
		{name: "duplicate code in platform", menus: []Menu{root, {ID: 2, PlatformID: 1, MenuType: TypeDirectory, Name: "Duplicate", Code: "test", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}}},
		{name: "duplicate page path in platform", menus: []Menu{root, {ID: 2, PlatformID: 1, MenuType: TypePage, Name: "Duplicate", Code: "other", I18nKey: stringPointer("navigation.system"), Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := buildMenuIndex(test.menus); err == nil {
				t.Fatal("buildMenuIndex accepted a platform conflict")
			}
		})
	}
}

func TestMenuIndexRejectsEnabledNodeBelowDisabledAncestor(t *testing.T) {
	now := time.Now().UTC()
	menus := []Menu{
		{ID: 1, PlatformID: 1, MenuType: TypeDirectory, Name: "报表", Code: "reports", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.No, CreatedAt: now, UpdatedAt: now},
		{ID: 2, PlatformID: 1, ParentID: int64Pointer(1), MenuType: TypeDirectory, Name: "报表分组", Code: "reports:section", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now},
	}
	index, err := buildMenuIndex(menus)
	if err != nil {
		t.Fatal(err)
	}
	if err := index.validateEnabledAncestors(); err == nil {
		t.Fatal("enabled child below disabled ancestor was accepted")
	}
}

func TestBuildMenuIndexAcceptsProfileMenuIcon(t *testing.T) {
	now := time.Now().UTC()
	icon := "lucide:user-circle"
	menu := Menu{
		ID: 1, PlatformID: 1, MenuType: TypeDirectory, Name: "个人中心", Code: "account:profile",
		I18nKey: stringPointer("navigation.account"), Icon: &icon, IsEnabled: yesno.Yes,
		CreatedAt: now, UpdatedAt: now,
	}
	if _, err := buildMenuIndex([]Menu{menu}); err != nil {
		t.Fatalf("profile menu icon rejected: %v", err)
	}
}

func int64Pointer(value int64) *int64 {
	return &value
}
