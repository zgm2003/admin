package menu

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/module/accessstate"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestServiceListReturnsCompleteSortedTree(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	repository := NewRepository(tx)
	rootB := createRepositoryDirectory(t, repository, ctx, "reports", 20)
	rootA := createRepositoryDirectory(t, repository, ctx, "settings", 10)
	path := "/settings"
	componentPath := "settings"
	page := Menu{PlatformID: rootA.PlatformID, ParentID: &rootA.ID, MenuType: TypePage, Name: "Settings", Code: "settings:list", I18nKey: stringPointer("navigation.systemMenus"), Path: &path, ComponentPath: &componentPath, SortOrder: 10, IsEnabled: yesno.No, IsHidden: yesno.No}
	if err := repository.Create(ctx, &page); err != nil {
		t.Fatal(err)
	}

	catalog, err := service.List(ctx, ListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	tree := catalog.MenuTree
	if len(catalog.Platforms) != 1 || catalog.Platforms[0].Code != "admin" || tree[0].PlatformID != catalog.Platforms[0].ID || tree[0].PlatformCode != "admin" {
		t.Fatalf("catalog platforms = %+v tree = %+v", catalog.Platforms, tree)
	}
	if len(tree) != 2 || tree[0].ID != rootA.ID || tree[1].ID != rootB.ID {
		t.Fatalf("tree roots = %+v", tree)
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].ID != page.ID || tree[0].Children[0].Children == nil {
		t.Fatalf("tree child = %+v", tree[0].Children)
	}
}

func TestServiceCreateEnforcesPlatformAndSupportsRootPage(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	adminPlatformID := testAdminPlatformID(t, tx, ctx)
	canvas := createRepositoryPlatform(t, tx, ctx, "canvas", "Canvas", yesno.No, false)
	deleted := createRepositoryPlatform(t, tx, ctx, "deleted", "Deleted", yesno.Yes, true)

	adminRootID, err := service.Create(ctx, CreateInput{
		PlatformID: adminPlatformID, MenuType: TypeDirectory, Name: "Admin", Code: "admin-root",
		I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes, IsHidden: yesno.No,
	})
	if err != nil {
		t.Fatal(err)
	}
	path, componentPath := "/test", "test"
	canvasPageID, err := service.Create(ctx, CreateInput{
		PlatformID: canvas.ID, MenuType: TypePage, Name: "Canvas Test", Code: "canvas:test",
		I18nKey: stringPointer("navigation.system"), Path: &path, ComponentPath: &componentPath,
		IsEnabled: yesno.Yes, IsHidden: yesno.No,
	})
	if err != nil || canvasPageID < 1 {
		t.Fatalf("Create(Canvas root page) = %d,%v", canvasPageID, err)
	}
	if _, err := service.Create(ctx, CreateInput{
		PlatformID: canvas.ID, ParentID: &canvasPageID, MenuType: TypeAction, Name: "Canvas Button",
		Code: "canvas:test:button", IsEnabled: yesno.Yes, IsHidden: yesno.Yes,
	}); err != nil {
		t.Fatalf("Create(Canvas action) error = %v", err)
	}
	if _, err := service.Create(ctx, CreateInput{
		PlatformID: canvas.ID, ParentID: &adminRootID, MenuType: TypePage, Name: "Cross Platform",
		Code: "canvas:cross", I18nKey: stringPointer("navigation.system"), Path: stringPointer("/cross"),
		ComponentPath: stringPointer("cross"), IsEnabled: yesno.Yes, IsHidden: yesno.No,
	}); menuServiceErrorCode(err) != CodeMenuInvalidParent {
		t.Fatalf("cross-platform parent error = %v", err)
	}
	if _, err := service.Create(ctx, CreateInput{
		PlatformID: canvas.ID, MenuType: TypeAction, Name: "Root Action", Code: "canvas:root-action",
		IsEnabled: yesno.Yes, IsHidden: yesno.Yes,
	}); menuServiceErrorCode(err) != CodeMenuInvalidParent {
		t.Fatalf("root action error = %v", err)
	}
	if _, err := service.Create(ctx, CreateInput{
		PlatformID: deleted.ID, MenuType: TypeDirectory, Name: "Deleted", Code: "deleted-root",
		I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes, IsHidden: yesno.No,
	}); menuServiceErrorCode(err) != CodeMenuPlatformUnavailable {
		t.Fatalf("deleted platform error = %v", err)
	}
}

func TestServiceCreateSupportsExplicitNullRootAndDisabledChild(t *testing.T) {
	_, ctx, service := openCleanMenuService(t)
	rootID, err := createAdminMenu(t, service, ctx, CreateInput{
		ParentID: nil, MenuType: TypeDirectory, Name: "Reports", Code: "reports", I18nKey: stringPointer("navigation.system"),
		Icon: stringPointer("lucide:folder"), SortOrder: 10, IsEnabled: yesno.No,
	})
	if err != nil || rootID < 1 {
		t.Fatalf("Create(root) = %d,%v", rootID, err)
	}
	path := "/reports"
	componentPath := "reports"
	if _, err := createAdminMenu(t, service, ctx, CreateInput{
		ParentID: &rootID, MenuType: TypePage, Name: "Reports", Code: "reports:list", I18nKey: stringPointer("navigation.systemMenus"),
		Path: &path, ComponentPath: &componentPath, SortOrder: 10, IsEnabled: yesno.Yes, IsHidden: yesno.No,
	}); menuServiceErrorCode(err) != CodeMenuParentDisabled {
		t.Fatalf("enabled child below disabled root error = %v", err)
	}
	childID, err := createAdminMenu(t, service, ctx, CreateInput{
		ParentID: &rootID, MenuType: TypePage, Name: "Reports", Code: "reports:list", I18nKey: stringPointer("navigation.systemMenus"),
		Path: &path, ComponentPath: &componentPath, SortOrder: 10, IsEnabled: yesno.No, IsHidden: yesno.No,
	})
	if err != nil || childID < 1 {
		t.Fatalf("Create(disabled child) = %d,%v", childID, err)
	}
}

func TestServiceCreateRejectsInvalidFieldsParentsAndConflicts(t *testing.T) {
	_, ctx, service := openCleanMenuService(t)
	rootID, err := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Reports", Code: "reports", I18nKey: stringPointer("navigation.system"), SortOrder: 10, IsEnabled: yesno.Yes})
	if err != nil {
		t.Fatal(err)
	}
	path := "/reports"
	componentPath := "reports"
	pageID, err := createAdminMenu(t, service, ctx, CreateInput{ParentID: &rootID, MenuType: TypePage, Name: "Reports", Code: "reports:list", I18nKey: stringPointer("navigation.systemMenus"), Path: &path, ComponentPath: &componentPath, SortOrder: 10, IsEnabled: yesno.Yes, IsHidden: yesno.No})
	if err != nil {
		t.Fatal(err)
	}

	missingID := int64(999999)
	tests := []struct {
		name  string
		input CreateInput
		code  int
	}{
		{name: "missing parent", input: CreateInput{ParentID: &missingID, MenuType: TypePage, Name: "Missing", Code: "missing:list", I18nKey: stringPointer("navigation.systemMenus"), Path: stringPointer("/missing"), ComponentPath: stringPointer("missing"), IsEnabled: yesno.Yes, IsHidden: yesno.No}, code: CodeMenuInvalidParent},
		{name: "page under page", input: CreateInput{ParentID: &pageID, MenuType: TypePage, Name: "Nested", Code: "nested:list", I18nKey: stringPointer("navigation.systemMenus"), Path: stringPointer("/nested"), ComponentPath: stringPointer("nested"), IsEnabled: yesno.Yes, IsHidden: yesno.No}, code: CodeMenuInvalidParent},
		{name: "action under directory", input: CreateInput{ParentID: &rootID, MenuType: TypeAction, Name: "Create report", Code: "reports:create", IsEnabled: yesno.Yes, IsHidden: yesno.Yes}, code: CodeMenuInvalidParent},
		{name: "root action", input: CreateInput{MenuType: TypeAction, Name: "Root", Code: "root:action", IsEnabled: yesno.Yes, IsHidden: yesno.Yes}, code: CodeMenuInvalidParent},
		{name: "invalid code", input: CreateInput{MenuType: TypeDirectory, Name: "Reports", Code: "Reports", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes}, code: CodeMenuInvalidFields},
		{name: "invalid i18n key", input: CreateInput{MenuType: TypeDirectory, Name: "Unknown", Code: "unknown", I18nKey: stringPointer("navigation.system_users"), IsEnabled: yesno.Yes}, code: CodeMenuInvalidFields},
		{name: "invalid icon whitespace", input: CreateInput{MenuType: TypeDirectory, Name: "Icons", Code: "icons", I18nKey: stringPointer("navigation.system"), Icon: stringPointer(" Unknown "), IsEnabled: yesno.Yes}, code: CodeMenuInvalidFields},
		{name: "invalid component path", input: CreateInput{ParentID: &rootID, MenuType: TypePage, Name: "Unknown", Code: "unknown:view", I18nKey: stringPointer("navigation.systemMenus"), Path: stringPointer("/unknown-view"), ComponentPath: stringPointer("/unknown-view"), IsEnabled: yesno.Yes, IsHidden: yesno.No}, code: CodeMenuInvalidFields},
		{name: "static path", input: CreateInput{ParentID: &rootID, MenuType: TypePage, Name: "Dashboard", Code: "dashboard:list", I18nKey: stringPointer("navigation.systemMenus"), Path: stringPointer("/dashboard"), ComponentPath: &componentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No}, code: CodeMenuInvalidFields},
		{name: "negative sort", input: CreateInput{MenuType: TypeDirectory, Name: "Sort", Code: "sort", I18nKey: stringPointer("navigation.system"), SortOrder: -1, IsEnabled: yesno.Yes}, code: CodeMenuInvalidFields},
		{name: "invalid yes no", input: CreateInput{MenuType: TypeDirectory, Name: "Status", Code: "status", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Value(2)}, code: CodeMenuInvalidFields},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := createAdminMenu(t, service, ctx, test.input); menuServiceErrorCode(err) != test.code {
				t.Fatalf("Create() error = %v, want code %d", err, test.code)
			}
		})
	}

	if _, err := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Reports", Code: " reports ", I18nKey: stringPointer(" navigation.system "), SortOrder: 10, IsEnabled: yesno.Yes}); menuServiceErrorCode(err) != CodeMenuInvalidFields {
		t.Fatalf("whitespace code error = %v", err)
	}
	if _, err := createAdminMenu(t, service, ctx, CreateInput{ParentID: &rootID, MenuType: TypePage, Name: "Reports", Code: "reports:second", I18nKey: stringPointer("navigation.systemMenus"), Path: stringPointer(" /reports "), ComponentPath: &componentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No}); menuServiceErrorCode(err) != CodeMenuInvalidFields {
		t.Fatalf("whitespace path error = %v", err)
	}
	if _, err := createAdminMenu(t, service, ctx, CreateInput{ParentID: &rootID, MenuType: TypePage, Name: "Reports", Code: "reports:second", I18nKey: stringPointer("reports.orders.list"), Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No}); menuServiceErrorCode(err) != CodeMenuPathConflict {
		t.Fatalf("duplicate path error = %v", err)
	}
}

func TestServiceUpdateMovesCandidateTreeAndKeepsCodeStable(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	rootID, _ := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Reports", Code: "reports", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes})
	otherID, _ := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Settings", Code: "settings", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes})
	path := "/reports"
	componentPath := "reports"
	pageID, _ := createAdminMenu(t, service, ctx, CreateInput{ParentID: &rootID, MenuType: TypePage, Name: "Reports", Code: "reports:list", I18nKey: stringPointer("navigation.systemMenus"), Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No})
	newPath := "/settings/reports"
	newComponentPath := "settings/reports"
	icon := "lucide:panel-left"
	if err := service.Update(ctx, pageID, UpdateInput{
		ParentID: &otherID, MenuType: TypePage, Name: "Reports", I18nKey: stringPointer("navigation.systemMenus"),
		Path: &newPath, ComponentPath: &newComponentPath, Icon: &icon, SortOrder: 9, IsHidden: yesno.Yes,
	}); err != nil {
		t.Fatal(err)
	}
	var stored Menu
	if err := tx.WithContext(ctx).First(&stored, pageID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Code != "reports:list" || stored.ParentID == nil || *stored.ParentID != otherID || value(stored.Path) != newPath ||
		value(stored.ComponentPath) != newComponentPath || stored.IsHidden != yesno.Yes {
		t.Fatalf("updated menu = %+v", stored)
	}
}

func TestServiceUpdateRejectsCyclesStructureGrantsAndDisabledAncestors(t *testing.T) {
	t.Run("cycle through descendant", func(t *testing.T) {
		_, ctx, service := openCleanMenuService(t)
		rootID, _ := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Reports", Code: "reports", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes})
		childID, _ := createAdminMenu(t, service, ctx, CreateInput{ParentID: &rootID, MenuType: TypeDirectory, Name: "Section", Code: "reports:section", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes})
		err := service.Update(ctx, rootID, UpdateInput{ParentID: &childID, MenuType: TypeDirectory, Name: "Reports", I18nKey: stringPointer("navigation.system"), SortOrder: 1})
		if menuServiceErrorCode(err) != CodeMenuCycleDetected {
			t.Fatalf("cycle error = %v", err)
		}
	})

	t.Run("existing child conflicts with new type", func(t *testing.T) {
		_, ctx, service := openCleanMenuService(t)
		rootID, _ := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Settings", Code: "settings", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes})
		reportsID, _ := createAdminMenu(t, service, ctx, CreateInput{ParentID: &rootID, MenuType: TypeDirectory, Name: "Reports", Code: "reports", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes})
		path := "/reports"
		componentPath := "reports"
		_, _ = createAdminMenu(t, service, ctx, CreateInput{ParentID: &reportsID, MenuType: TypePage, Name: "Reports", Code: "reports:list", I18nKey: stringPointer("navigation.systemMenus"), Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No})
		sectionPath := "/reports/section"
		sectionComponentPath := "reports/section"
		err := service.Update(ctx, reportsID, UpdateInput{ParentID: &rootID, MenuType: TypePage, Name: "Reports", I18nKey: stringPointer("navigation.systemMenus"), Path: &sectionPath, ComponentPath: &sectionComponentPath, SortOrder: 1, IsHidden: yesno.No})
		if menuServiceErrorCode(err) != CodeMenuStructureConflict {
			t.Fatalf("structure error = %v", err)
		}
	})

	t.Run("direct grant blocks directory conversion", func(t *testing.T) {
		tx, ctx, service := openCleanMenuService(t)
		rootID, _ := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Reports", Code: "reports", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes})
		path := "/reports"
		componentPath := "reports"
		pageID, _ := createAdminMenu(t, service, ctx, CreateInput{ParentID: &rootID, MenuType: TypePage, Name: "Reports", Code: "reports:list", I18nKey: stringPointer("navigation.systemMenus"), Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No})
		createdRole := testRole{Code: fmt.Sprintf("menu_service_role_%d", time.Now().UnixNano()), Name: "Menu Service Role", IsDefault: yesno.No, IsEnabled: yesno.Yes}
		if err := tx.WithContext(ctx).Create(&createdRole).Error; err != nil {
			t.Fatal(err)
		}
		if err := tx.WithContext(ctx).Create(&RoleMenu{RoleID: createdRole.ID, MenuID: pageID}).Error; err != nil {
			t.Fatal(err)
		}
		err := service.Update(ctx, pageID, UpdateInput{MenuType: TypeDirectory, Name: "Reports", I18nKey: stringPointer("navigation.system"), SortOrder: 1, IsHidden: yesno.No})
		if menuServiceErrorCode(err) != CodeMenuStructureConflict {
			t.Fatalf("grant conflict error = %v", err)
		}
	})

	t.Run("enabled subtree cannot move below disabled parent", func(t *testing.T) {
		_, ctx, service := openCleanMenuService(t)
		rootID, _ := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Reports", Code: "reports", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes})
		disabledID, _ := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Disabled", Code: "disabled", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.No})
		err := service.Update(ctx, rootID, UpdateInput{ParentID: &disabledID, MenuType: TypeDirectory, Name: "Reports", I18nKey: stringPointer("navigation.system"), SortOrder: 1})
		if menuServiceErrorCode(err) != CodeMenuParentDisabled {
			t.Fatalf("disabled parent error = %v", err)
		}
	})
}

func TestServiceUpdateAllowsChangingAnyActiveMenuRecord(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	rootID, err := createAdminMenu(t, service, ctx, CreateInput{
		MenuType: TypeDirectory, Name: "Reports", Code: "reports", I18nKey: stringPointer("navigation.system"),
		Icon: stringPointer("lucide:folder"), SortOrder: 10, IsEnabled: yesno.Yes, IsHidden: yesno.No,
	})
	if err != nil {
		t.Fatal(err)
	}
	path := "/reports"
	componentPath := "reports"
	pageID, err := createAdminMenu(t, service, ctx, CreateInput{
		ParentID: &rootID, MenuType: TypePage, Name: "Reports", Code: "reports:list", I18nKey: stringPointer("reports.orders.list"),
		Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No,
	})
	if err != nil {
		t.Fatal(err)
	}
	newPath := "/reports/order-items"
	newComponentPath := "reports/order-items"
	if err := service.Update(ctx, pageID, UpdateInput{
		ParentID: &rootID, MenuType: TypePage, Name: "Order items", I18nKey: stringPointer("reports.orderItems.list"),
		Path: &newPath, ComponentPath: &newComponentPath, Icon: stringPointer("lucide:shield-check"),
		SortOrder: 55, IsHidden: yesno.Yes,
	}); err != nil {
		t.Fatalf("update ordinary page: %v", err)
	}
	var stored Menu
	if err := tx.WithContext(ctx).First(&stored, pageID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Name != "Order items" || value(stored.I18nKey) != "reports.orderItems.list" || value(stored.Path) != newPath ||
		value(stored.ComponentPath) != newComponentPath || stored.IsHidden != yesno.Yes {
		t.Fatalf("updated ordinary page = %+v", stored)
	}
}

func TestServiceUpdateRollsBackDatabaseWriteFailure(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	id, err := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Reports", Code: "reports", I18nKey: stringPointer("navigation.system"), Icon: stringPointer("lucide:folder"), SortOrder: 10, IsEnabled: yesno.Yes})
	if err != nil {
		t.Fatal(err)
	}
	var before Menu
	if err := tx.WithContext(ctx).First(&before, id).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Exec(`ALTER TABLE rbac_menu ADD CONSTRAINT ck_test_menu_update_rollback CHECK (i18n_key <> 'navigation.systemMenus')`).Error; err != nil {
		t.Fatal(err)
	}
	err = service.Update(ctx, id, UpdateInput{MenuType: TypeDirectory, Name: "Reports", I18nKey: stringPointer("navigation.systemMenus"), Icon: stringPointer("lucide:cpu"), SortOrder: 99, IsHidden: yesno.No})
	if menuServiceErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("write failure error = %v", err)
	}
	var after Menu
	if err := tx.WithContext(ctx).First(&after, id).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("row changed after failed update: before=%+v after=%+v", before, after)
	}
}

func TestServiceUpdateStatusDisablesSubtreeAndPreservesGrants(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	rootID, _ := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Reports", Code: "reports", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes})
	childID, _ := createAdminMenu(t, service, ctx, CreateInput{ParentID: &rootID, MenuType: TypeDirectory, Name: "Section", Code: "reports:section", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes})
	path := "/reports"
	componentPath := "reports"
	pageID, _ := createAdminMenu(t, service, ctx, CreateInput{ParentID: &childID, MenuType: TypePage, Name: "Reports", Code: "reports:list", I18nKey: stringPointer("navigation.systemMenus"), Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No})
	createdRole := testRole{Code: fmt.Sprintf("menu_status_role_%d", time.Now().UnixNano()), Name: "Menu Status Role", IsDefault: yesno.No, IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&createdRole).Error; err != nil {
		t.Fatal(err)
	}
	grant := RoleMenu{RoleID: createdRole.ID, MenuID: pageID}
	if err := tx.WithContext(ctx).Create(&grant).Error; err != nil {
		t.Fatal(err)
	}

	if err := service.UpdateStatus(ctx, rootID, yesno.No); err != nil {
		t.Fatal(err)
	}
	var menus []Menu
	if err := tx.WithContext(ctx).Where("id IN ?", []int64{rootID, childID, pageID}).Order("id").Find(&menus).Error; err != nil {
		t.Fatal(err)
	}
	for _, item := range menus {
		if item.IsEnabled != yesno.No {
			t.Fatalf("disabled subtree row = %+v", item)
		}
	}
	var storedGrant RoleMenu
	if err := tx.WithContext(ctx).First(&storedGrant, grant.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedGrant.DeletedAt.Valid {
		t.Fatal("status change deleted role-menu grant")
	}
}

func TestServiceUpdateStatusEnablesOnlyCurrentNodeAndChecksAncestors(t *testing.T) {
	t.Run("rejects child below disabled ancestor", func(t *testing.T) {
		_, ctx, service := openCleanMenuService(t)
		rootID, _ := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Reports", Code: "reports", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.No})
		childID, _ := createAdminMenu(t, service, ctx, CreateInput{ParentID: &rootID, MenuType: TypeDirectory, Name: "Section", Code: "reports:section", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.No})
		if err := service.UpdateStatus(ctx, childID, yesno.Yes); menuServiceErrorCode(err) != CodeMenuParentDisabled {
			t.Fatalf("enable child error = %v", err)
		}
		var child Menu
		if err := service.repository.db.WithContext(ctx).First(&child, childID).Error; err != nil {
			t.Fatal(err)
		}
		if child.IsEnabled != yesno.No {
			t.Fatal("failed enable changed the child")
		}
	})

	t.Run("enables only parent", func(t *testing.T) {
		_, ctx, service := openCleanMenuService(t)
		rootID, _ := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Reports", Code: "reports", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.No})
		childID, _ := createAdminMenu(t, service, ctx, CreateInput{ParentID: &rootID, MenuType: TypeDirectory, Name: "Section", Code: "reports:section", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.No})
		if err := service.UpdateStatus(ctx, rootID, yesno.Yes); err != nil {
			t.Fatal(err)
		}
		var child Menu
		if err := service.repository.db.WithContext(ctx).First(&child, childID).Error; err != nil {
			t.Fatal(err)
		}
		if child.IsEnabled != yesno.No {
			t.Fatal("enabling parent also enabled a disabled child")
		}
		if err := service.UpdateStatus(ctx, rootID, yesno.Yes); err != nil {
			t.Fatalf("same-state enable error = %v", err)
		}
	})
}

func TestServiceUpdateStatusAndDeleteAllowOrdinaryMenus(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	disabledID, err := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Reports", Code: "reports", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes, IsHidden: yesno.No})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateStatus(ctx, disabledID, yesno.No); err != nil {
		t.Fatalf("disable ordinary menu: %v", err)
	}
	deletedID, err := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Settings", Code: "settings", I18nKey: stringPointer("navigation.settings"), IsEnabled: yesno.Yes, IsHidden: yesno.No})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, deletedID); err != nil {
		t.Fatalf("delete ordinary menu: %v", err)
	}
	var disabled Menu
	if err := tx.WithContext(ctx).First(&disabled, disabledID).Error; err != nil || disabled.IsEnabled != yesno.No {
		t.Fatalf("disabled ordinary menu = %+v, %v", disabled, err)
	}
	var deleted Menu
	if err := tx.WithContext(ctx).Unscoped().First(&deleted, deletedID).Error; err != nil || !deleted.DeletedAt.Valid {
		t.Fatalf("deleted ordinary menu = %+v, %v", deleted, err)
	}
}

func TestServiceDeleteSoftDeletesSubtreeAndRoleMenusTogether(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	rootID, _ := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Reports", Code: "reports", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes})
	path := "/reports"
	componentPath := "reports"
	pageID, _ := createAdminMenu(t, service, ctx, CreateInput{ParentID: &rootID, MenuType: TypePage, Name: "Reports", Code: "reports:list", I18nKey: stringPointer("navigation.systemMenus"), Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No})
	actionID, _ := createAdminMenu(t, service, ctx, CreateInput{ParentID: &pageID, MenuType: TypeAction, Name: "Create report", Code: "reports:create", IsEnabled: yesno.Yes, IsHidden: yesno.Yes})
	createdRole := testRole{Code: fmt.Sprintf("menu_delete_role_%d", time.Now().UnixNano()), Name: "Menu Delete Role", IsDefault: yesno.No, IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&createdRole).Error; err != nil {
		t.Fatal(err)
	}
	grants := []RoleMenu{{RoleID: createdRole.ID, MenuID: pageID}, {RoleID: createdRole.ID, MenuID: actionID}}
	for index := range grants {
		if err := tx.WithContext(ctx).Create(&grants[index]).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := service.Delete(ctx, rootID); err != nil {
		t.Fatal(err)
	}
	var menus []Menu
	if err := tx.WithContext(ctx).Unscoped().Where("id IN ?", []int64{rootID, pageID, actionID}).Order("id").Find(&menus).Error; err != nil {
		t.Fatal(err)
	}
	if len(menus) != 3 || !menus[0].DeletedAt.Valid || !menus[1].DeletedAt.Valid || !menus[2].DeletedAt.Valid {
		t.Fatalf("deleted menus = %+v", menus)
	}
	deletedAt := menus[0].DeletedAt.Time
	for _, item := range menus {
		if !item.DeletedAt.Time.Equal(deletedAt) || !item.UpdatedAt.Equal(deletedAt) {
			t.Fatalf("delete timestamp mismatch: %+v", item)
		}
	}
	var storedGrants []RoleMenu
	if err := tx.WithContext(ctx).Unscoped().Where("role_id = ?", createdRole.ID).Find(&storedGrants).Error; err != nil {
		t.Fatal(err)
	}
	for _, grant := range storedGrants {
		if !grant.DeletedAt.Valid || !grant.DeletedAt.Time.Equal(deletedAt) || !grant.UpdatedAt.Equal(deletedAt) {
			t.Fatalf("deleted grant = %+v", grant)
		}
	}
	catalog, err := service.List(ctx, ListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	listed := catalog.MenuTree
	if len(listed) != 0 {
		t.Fatalf("deleted menus remain in List(): %+v", listed)
	}
}

func TestServiceDeleteRollsBackRoleMenusWhenMenuWriteFails(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	rootID, _ := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Reports", Code: "reports", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes})
	createdRole := testRole{Code: fmt.Sprintf("menu_delete_rollback_role_%d", time.Now().UnixNano()), Name: "Menu Delete Rollback Role", IsDefault: yesno.No, IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&createdRole).Error; err != nil {
		t.Fatal(err)
	}
	grant := RoleMenu{RoleID: createdRole.ID, MenuID: rootID}
	if err := tx.WithContext(ctx).Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Exec(`ALTER TABLE rbac_menu ADD CONSTRAINT ck_test_menu_delete_rollback CHECK (deleted_at IS NULL)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, rootID); menuServiceErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("delete write failure error = %v", err)
	}
	var menuRow Menu
	if err := tx.WithContext(ctx).First(&menuRow, rootID).Error; err != nil {
		t.Fatal(err)
	}
	var roleMenuRow RoleMenu
	if err := tx.WithContext(ctx).First(&roleMenuRow, grant.ID).Error; err != nil {
		t.Fatal(err)
	}
	if menuRow.DeletedAt.Valid || roleMenuRow.DeletedAt.Valid {
		t.Fatalf("delete rollback left changed rows: menu=%+v roleMenu=%+v", menuRow, roleMenuRow)
	}
}

func TestServiceMenuMutationsAdvanceAllActiveAccessVersions(t *testing.T) {
	tx, ctx := openMenuTransaction(t)
	if err := tx.WithContext(ctx).Unscoped().Exec("DELETE FROM rbac_role_menu").Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Unscoped().Exec("DELETE FROM rbac_menu").Error; err != nil {
		t.Fatal(err)
	}
	service, accessStates, _ := newMenuMutationTestService(t, NewRepository(tx))
	first := createMenuAccessUser(t, tx, ctx, yesno.Yes, false)
	second := createMenuAccessUser(t, tx, ctx, yesno.Yes, false)
	disabled := createMenuAccessUser(t, tx, ctx, yesno.No, false)

	id, err := createAdminMenu(t, service, ctx, CreateInput{
		MenuType: TypeDirectory, Name: "Reports", Code: "reports", I18nKey: stringPointer("navigation.system"), SortOrder: 10, IsEnabled: yesno.Yes,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertMenuAccessVersions(t, tx, ctx, accessStates, []testUser{first, second}, 2)
	if got := readMenuAccessVersion(t, tx, ctx, disabled.ID); got != 1 {
		t.Fatalf("disabled user access version = %d", got)
	}

	update := UpdateInput{MenuType: TypeDirectory, Name: "Reports", I18nKey: stringPointer("navigation.system"), Icon: stringPointer("lucide:folder"), SortOrder: 20}
	if err := service.Update(ctx, id, update); err != nil {
		t.Fatal(err)
	}
	assertMenuAccessVersions(t, tx, ctx, accessStates, []testUser{first, second}, 3)
	var afterUpdate Menu
	if err := tx.WithContext(ctx).Take(&afterUpdate, id).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.Update(ctx, id, update); err != nil {
		t.Fatal(err)
	}
	if got := readMenuAccessVersion(t, tx, ctx, first.ID); got != 3 {
		t.Fatalf("no-op update access version = %d", got)
	}
	var afterNoOp Menu
	if err := tx.WithContext(ctx).Take(&afterNoOp, id).Error; err != nil || !afterNoOp.UpdatedAt.Equal(afterUpdate.UpdatedAt) {
		t.Fatalf("no-op update rewrote menu: %+v,%v", afterNoOp, err)
	}

	if err := service.UpdateStatus(ctx, id, yesno.No); err != nil {
		t.Fatal(err)
	}
	assertMenuAccessVersions(t, tx, ctx, accessStates, []testUser{first, second}, 4)
	if err := service.UpdateStatus(ctx, id, yesno.No); err != nil {
		t.Fatal(err)
	}
	if got := readMenuAccessVersion(t, tx, ctx, first.ID); got != 4 {
		t.Fatalf("no-op status access version = %d", got)
	}

	if err := service.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	assertMenuAccessVersions(t, tx, ctx, accessStates, []testUser{first, second}, 5)
}

func TestServiceMenuRedisFailurePreventsPostgreSQLMutation(t *testing.T) {
	tx, ctx := openMenuTransaction(t)
	service, _, redisClient := newMenuMutationTestService(t, NewRepository(tx))
	active := createMenuAccessUser(t, tx, ctx, yesno.Yes, false)
	if err := redisClient.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Blocked", Code: "redis:blocked", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes}); menuServiceErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Create() error = %v", err)
	}
	var count int64
	if err := tx.WithContext(ctx).Model(&Menu{}).Where("code = ?", "redis:blocked").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 || readMenuAccessVersion(t, tx, ctx, active.ID) != 1 {
		t.Fatalf("PostgreSQL mutated without Redis coordination: menuCount=%d", count)
	}
}

func TestConcurrentMenuMutationRechecksChangedActiveUsers(t *testing.T) {
	db, ctx := openMenuDatabase(t)
	repository := NewRepository(db)
	base := Menu{PlatformID: testAdminPlatformID(t, db, ctx), MenuType: TypeDirectory, Name: "Base", Code: "base", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes}
	if err := repository.Create(ctx, &base); err != nil {
		t.Fatal(err)
	}
	active := createMenuAccessUser(t, db, ctx, yesno.Yes, false)
	blocker := db.WithContext(ctx).Begin()
	if blocker.Error != nil {
		t.Fatal(blocker.Error)
	}
	t.Cleanup(func() { _ = blocker.Rollback().Error })
	if _, err := NewRepository(blocker).LockActiveMenus(ctx); err != nil {
		t.Fatal(err)
	}
	service, states, _ := newMenuMutationTestService(t, NewRepository(db))
	done := make(chan error, 1)
	go func() {
		_, err := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Reports", Code: "reports", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes})
		done <- err
	}()
	waitForMenuAccessState(t, states, active.ID, accessstate.StateInvalidating)
	if err := db.WithContext(ctx).Model(&testUser{}).Where("id = ?", active.ID).Update("is_enabled", yesno.No).Error; err != nil {
		t.Fatal(err)
	}
	if err := blocker.Commit().Error; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("menu mutation did not finish")
	}
	if got := readMenuAccessVersion(t, db, ctx, active.ID); got != 1 {
		t.Fatalf("disabled candidate access version = %d", got)
	}
	state, found, err := states.Read(ctx, active.ID)
	if err != nil || !found || state.State != accessstate.StateReady || state.Version != 1 {
		t.Fatalf("restored access state = %+v found=%v error=%v", state, found, err)
	}
}

func TestServiceMenuTransactionFailureRestoresAccessStateAndVersion(t *testing.T) {
	tx, ctx := openMenuTransaction(t)
	service, states, _ := newMenuMutationTestService(t, NewRepository(tx))
	active := createMenuAccessUser(t, tx, ctx, yesno.Yes, false)
	if err := tx.WithContext(ctx).Exec(`ALTER TABLE rbac_menu ADD CONSTRAINT ck_test_menu_create_rollback CHECK (code <> 'rollback:create')`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Rollback", Code: "rollback:create", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes}); menuServiceErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Create() error = %v", err)
	}
	if got := readMenuAccessVersion(t, tx, ctx, active.ID); got != 1 {
		t.Fatalf("rolled-back access version = %d", got)
	}
	state, found, err := states.Read(ctx, active.ID)
	if err != nil || !found || state.State != accessstate.StateReady || state.Version != 1 {
		t.Fatalf("restored access state = %+v found=%v error=%v", state, found, err)
	}
}

func TestServiceMenuPublishFailureLeavesCommittedVersionUnreachable(t *testing.T) {
	db, ctx := openMenuDatabase(t)
	service, states, redisClient := newMenuMutationTestService(t, NewRepository(db))
	active := createMenuAccessUser(t, db, ctx, yesno.Yes, false)
	if err := db.WithContext(ctx).Exec(`
		CREATE FUNCTION delay_menu_create_publish() RETURNS trigger AS $$
		BEGIN
			PERFORM pg_sleep(0.5);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER delay_menu_create_publish
		BEFORE INSERT ON rbac_menu
		FOR EACH ROW EXECUTE FUNCTION delay_menu_create_publish()`).Error; err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := createAdminMenu(t, service, ctx, CreateInput{MenuType: TypeDirectory, Name: "Publish", Code: "publish:failure", I18nKey: stringPointer("navigation.system"), IsEnabled: yesno.Yes})
		done <- err
	}()
	waitForMenuAccessState(t, states, active.ID, accessstate.StateInvalidating)
	if err := redisClient.Delete(ctx, accessstate.StateKey(active.ID)); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if menuServiceErrorCode(err) != apperror.CodeDependencyUnavailable {
			t.Fatalf("Create() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("menu mutation did not finish")
	}
	var count int64
	if err := db.WithContext(ctx).Model(&Menu{}).Where("code = ?", "publish:failure").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 || readMenuAccessVersion(t, db, ctx, active.ID) != 2 {
		t.Fatalf("committed PostgreSQL state: menuCount=%d", count)
	}
	if _, found, err := states.Read(ctx, active.ID); err != nil || found {
		t.Fatalf("old access state remained reachable: found=%v error=%v", found, err)
	}
}

func openCleanMenuService(t *testing.T) (*gorm.DB, context.Context, *Service) {
	t.Helper()
	tx, ctx := openMenuTransaction(t)
	if err := tx.WithContext(ctx).Unscoped().Exec("DELETE FROM rbac_role_menu").Error; err != nil {
		t.Fatalf("clear role menus: %v", err)
	}
	if err := tx.WithContext(ctx).Unscoped().Exec("DELETE FROM rbac_menu").Error; err != nil {
		t.Fatalf("clear menus: %v", err)
	}
	return tx, ctx, newMenuTestService(t, NewRepository(tx))
}

func createAdminMenu(t *testing.T, service *Service, ctx context.Context, input CreateInput) (int64, error) {
	t.Helper()
	input.PlatformID = testAdminPlatformID(t, service.repository.db, ctx)
	return service.Create(ctx, input)
}

func newMenuTestService(t *testing.T, repository *Repository) *Service {
	t.Helper()
	service, _, _ := newMenuMutationTestService(t, repository)
	return service
}

func newMenuMutationTestService(t *testing.T, repository *Repository) (*Service, *accessstate.Store, *projectredis.Client) {
	t.Helper()
	redisClient := openMenuTestRedis(t)
	accessStates := accessstate.NewStore(redisClient)
	return NewService(repository, accessstate.NewInvalidator(accessStates)), accessStates, redisClient
}

func openMenuTestRedis(t *testing.T) *projectredis.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("Redis integration test")
	}
	if err := godotenv.Load("../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("load server .env: %v", err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatalf("load worker config: %v", err)
	}
	redisURL, err := url.Parse(settings.RedisURL)
	if err != nil {
		t.Fatalf("parse Redis URL: %v", err)
	}
	redisURL.Path = "/12"
	redisURL.RawPath = ""
	client, err := projectredis.Open(context.Background(), redisURL.String())
	if err != nil {
		t.Fatalf("open test Redis database 12: %v", err)
	}
	if err := client.ScanDelete(context.Background(), "authz:access-state:*"); err != nil {
		_ = client.Close()
		t.Fatalf("clean test Redis database 12: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func readMenuAccessVersion(t *testing.T, tx *gorm.DB, ctx context.Context, userID int64) int64 {
	t.Helper()
	var version int64
	result := tx.WithContext(ctx).Raw("SELECT version FROM rbac_access_version WHERE user_id = ?", userID).Scan(&version)
	if result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("read access version: rows=%d error=%v", result.RowsAffected, result.Error)
	}
	return version
}

func assertMenuAccessVersions(t *testing.T, tx *gorm.DB, ctx context.Context, states *accessstate.Store, users []testUser, version int64) {
	t.Helper()
	for _, target := range users {
		if got := readMenuAccessVersion(t, tx, ctx, target.ID); got != version {
			t.Fatalf("user %d access version = %d, want %d", target.ID, got, version)
		}
		state, found, err := states.Read(context.Background(), target.ID)
		if err != nil || !found || state.State != accessstate.StateReady || state.Version != version {
			t.Fatalf("user %d access state = %+v found=%v error=%v", target.ID, state, found, err)
		}
	}
}

func waitForMenuAccessState(t *testing.T, states *accessstate.Store, userID int64, wanted string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		state, found, err := states.Read(context.Background(), userID)
		if err == nil && found && state.State == wanted {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("access state for user %d did not become %s", userID, wanted)
}

func menuServiceErrorCode(err error) int {
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		return 0
	}
	return appErr.Code
}
