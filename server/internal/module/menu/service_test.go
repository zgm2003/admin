package menu

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

func TestServiceListReturnsCompleteSortedTree(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	repository := NewRepository(tx)
	rootB := createRepositoryDirectory(t, repository, ctx, "reports", 20)
	rootA := createRepositoryDirectory(t, repository, ctx, "settings", 10)
	path := "/settings"
	viewKey := "system-menus"
	page := Menu{ParentID: &rootA.ID, MenuType: TypePage, Code: "settings:list", I18nKey: "navigation.systemMenus", Path: &path, ViewKey: &viewKey, SortOrder: 10, IsEnabled: yesno.No}
	if err := repository.Create(ctx, &page); err != nil {
		t.Fatal(err)
	}

	tree, err := service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 2 || tree[0].ID != rootA.ID || tree[1].ID != rootB.ID {
		t.Fatalf("tree roots = %+v", tree)
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].ID != page.ID || tree[0].Children[0].Children == nil {
		t.Fatalf("tree child = %+v", tree[0].Children)
	}
}

func TestServiceCreateSupportsExplicitNullRootAndDisabledChild(t *testing.T) {
	_, ctx, service := openCleanMenuService(t)
	rootID, err := service.Create(ctx, CreateInput{
		ParentID: nil, MenuType: TypeDirectory, Code: "reports", I18nKey: "navigation.system",
		Icon: stringPointer("Folder"), SortOrder: 10, IsEnabled: yesno.No,
	})
	if err != nil || rootID < 1 {
		t.Fatalf("Create(root) = %d,%v", rootID, err)
	}
	path := "/reports"
	viewKey := "system-menus"
	if _, err := service.Create(ctx, CreateInput{
		ParentID: &rootID, MenuType: TypePage, Code: "reports:list", I18nKey: "navigation.systemMenus",
		Path: &path, ViewKey: &viewKey, SortOrder: 10, IsEnabled: yesno.Yes,
	}); menuServiceErrorCode(err) != CodeMenuParentDisabled {
		t.Fatalf("enabled child below disabled root error = %v", err)
	}
	childID, err := service.Create(ctx, CreateInput{
		ParentID: &rootID, MenuType: TypePage, Code: "reports:list", I18nKey: "navigation.systemMenus",
		Path: &path, ViewKey: &viewKey, SortOrder: 10, IsEnabled: yesno.No,
	})
	if err != nil || childID < 1 {
		t.Fatalf("Create(disabled child) = %d,%v", childID, err)
	}
}

func TestServiceCreateRejectsInvalidFieldsParentsAndConflicts(t *testing.T) {
	_, ctx, service := openCleanMenuService(t)
	rootID, err := service.Create(ctx, CreateInput{MenuType: TypeDirectory, Code: "reports", I18nKey: "navigation.system", SortOrder: 10, IsEnabled: yesno.Yes})
	if err != nil {
		t.Fatal(err)
	}
	path := "/reports"
	viewKey := "system-menus"
	pageID, err := service.Create(ctx, CreateInput{ParentID: &rootID, MenuType: TypePage, Code: "reports:list", I18nKey: "navigation.systemMenus", Path: &path, ViewKey: &viewKey, SortOrder: 10, IsEnabled: yesno.Yes})
	if err != nil {
		t.Fatal(err)
	}

	missingID := int64(999999)
	tests := []struct {
		name  string
		input CreateInput
		code  int
	}{
		{name: "missing parent", input: CreateInput{ParentID: &missingID, MenuType: TypePage, Code: "missing:list", I18nKey: "navigation.systemMenus", Path: stringPointer("/missing"), ViewKey: &viewKey, IsEnabled: yesno.Yes}, code: CodeMenuInvalidParent},
		{name: "page under page", input: CreateInput{ParentID: &pageID, MenuType: TypePage, Code: "nested:list", I18nKey: "navigation.systemMenus", Path: stringPointer("/nested"), ViewKey: &viewKey, IsEnabled: yesno.Yes}, code: CodeMenuInvalidParent},
		{name: "action under directory", input: CreateInput{ParentID: &rootID, MenuType: TypeAction, Code: "reports:create", I18nKey: "permission.menuCreate", IsEnabled: yesno.Yes}, code: CodeMenuInvalidParent},
		{name: "page root", input: CreateInput{MenuType: TypePage, Code: "root:list", I18nKey: "navigation.systemMenus", Path: stringPointer("/root-list"), ViewKey: &viewKey, IsEnabled: yesno.Yes}, code: CodeMenuInvalidParent},
		{name: "invalid code", input: CreateInput{MenuType: TypeDirectory, Code: "Reports", I18nKey: "navigation.system", IsEnabled: yesno.Yes}, code: CodeMenuInvalidFields},
		{name: "unknown title", input: CreateInput{MenuType: TypeDirectory, Code: "unknown", I18nKey: "navigation.unknown", IsEnabled: yesno.Yes}, code: CodeMenuInvalidFields},
		{name: "unknown icon", input: CreateInput{MenuType: TypeDirectory, Code: "icons", I18nKey: "navigation.system", Icon: stringPointer("Unknown"), IsEnabled: yesno.Yes}, code: CodeMenuInvalidFields},
		{name: "unknown view", input: CreateInput{ParentID: &rootID, MenuType: TypePage, Code: "unknown:view", I18nKey: "navigation.systemMenus", Path: stringPointer("/unknown-view"), ViewKey: stringPointer("unknown"), IsEnabled: yesno.Yes}, code: CodeMenuInvalidFields},
		{name: "static path", input: CreateInput{ParentID: &rootID, MenuType: TypePage, Code: "dashboard:list", I18nKey: "navigation.systemMenus", Path: stringPointer("/dashboard"), ViewKey: &viewKey, IsEnabled: yesno.Yes}, code: CodeMenuInvalidFields},
		{name: "negative sort", input: CreateInput{MenuType: TypeDirectory, Code: "sort", I18nKey: "navigation.system", SortOrder: -1, IsEnabled: yesno.Yes}, code: CodeMenuInvalidFields},
		{name: "invalid yes no", input: CreateInput{MenuType: TypeDirectory, Code: "status", I18nKey: "navigation.system", IsEnabled: yesno.Value(2)}, code: CodeMenuInvalidFields},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := service.Create(ctx, test.input); menuServiceErrorCode(err) != test.code {
				t.Fatalf("Create() error = %v, want code %d", err, test.code)
			}
		})
	}

	if _, err := service.Create(ctx, CreateInput{MenuType: TypeDirectory, Code: " reports ", I18nKey: " navigation.system ", SortOrder: 10, IsEnabled: yesno.Yes}); menuServiceErrorCode(err) != CodeMenuCodeConflict {
		t.Fatalf("trimmed duplicate code error = %v", err)
	}
	if _, err := service.Create(ctx, CreateInput{ParentID: &rootID, MenuType: TypePage, Code: "reports:second", I18nKey: "navigation.systemMenus", Path: stringPointer(" /reports "), ViewKey: &viewKey, IsEnabled: yesno.Yes}); menuServiceErrorCode(err) != CodeMenuPathConflict {
		t.Fatalf("trimmed duplicate path error = %v", err)
	}
}

func TestServiceUpdateMovesCandidateTreeAndKeepsCodeStable(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	rootID, _ := service.Create(ctx, CreateInput{MenuType: TypeDirectory, Code: "reports", I18nKey: "navigation.system", IsEnabled: yesno.Yes})
	otherID, _ := service.Create(ctx, CreateInput{MenuType: TypeDirectory, Code: "settings", I18nKey: "navigation.system", IsEnabled: yesno.Yes})
	path := "/reports"
	viewKey := "system-menus"
	pageID, _ := service.Create(ctx, CreateInput{ParentID: &rootID, MenuType: TypePage, Code: "reports:list", I18nKey: "navigation.systemMenus", Path: &path, ViewKey: &viewKey, IsEnabled: yesno.Yes})
	newPath := "/settings/reports"
	icon := "Menu"
	if err := service.Update(ctx, pageID, UpdateInput{
		ParentID: &otherID, MenuType: TypePage, I18nKey: "navigation.systemMenus",
		Path: &newPath, ViewKey: &viewKey, Icon: &icon, SortOrder: 9,
	}); err != nil {
		t.Fatal(err)
	}
	var stored Menu
	if err := tx.WithContext(ctx).First(&stored, pageID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Code != "reports:list" || stored.ParentID == nil || *stored.ParentID != otherID || value(stored.Path) != newPath {
		t.Fatalf("updated menu = %+v", stored)
	}
}

func TestServiceUpdateRejectsCyclesStructureGrantsAndDisabledAncestors(t *testing.T) {
	t.Run("cycle through descendant", func(t *testing.T) {
		_, ctx, service := openCleanMenuService(t)
		rootID, _ := service.Create(ctx, CreateInput{MenuType: TypeDirectory, Code: "reports", I18nKey: "navigation.system", IsEnabled: yesno.Yes})
		childID, _ := service.Create(ctx, CreateInput{ParentID: &rootID, MenuType: TypeDirectory, Code: "reports:section", I18nKey: "navigation.system", IsEnabled: yesno.Yes})
		err := service.Update(ctx, rootID, UpdateInput{ParentID: &childID, MenuType: TypeDirectory, I18nKey: "navigation.system", SortOrder: 1})
		if menuServiceErrorCode(err) != CodeMenuCycleDetected {
			t.Fatalf("cycle error = %v", err)
		}
	})

	t.Run("existing child conflicts with new type", func(t *testing.T) {
		_, ctx, service := openCleanMenuService(t)
		rootID, _ := service.Create(ctx, CreateInput{MenuType: TypeDirectory, Code: "settings", I18nKey: "navigation.system", IsEnabled: yesno.Yes})
		reportsID, _ := service.Create(ctx, CreateInput{ParentID: &rootID, MenuType: TypeDirectory, Code: "reports", I18nKey: "navigation.system", IsEnabled: yesno.Yes})
		path := "/reports"
		view := "system-menus"
		_, _ = service.Create(ctx, CreateInput{ParentID: &reportsID, MenuType: TypePage, Code: "reports:list", I18nKey: "navigation.systemMenus", Path: &path, ViewKey: &view, IsEnabled: yesno.Yes})
		sectionPath := "/reports/section"
		err := service.Update(ctx, reportsID, UpdateInput{ParentID: &rootID, MenuType: TypePage, I18nKey: "navigation.systemMenus", Path: &sectionPath, ViewKey: &view, SortOrder: 1})
		if menuServiceErrorCode(err) != CodeMenuStructureConflict {
			t.Fatalf("structure error = %v", err)
		}
	})

	t.Run("direct grant blocks directory conversion", func(t *testing.T) {
		tx, ctx, service := openCleanMenuService(t)
		rootID, _ := service.Create(ctx, CreateInput{MenuType: TypeDirectory, Code: "reports", I18nKey: "navigation.system", IsEnabled: yesno.Yes})
		path := "/reports"
		view := "system-menus"
		pageID, _ := service.Create(ctx, CreateInput{ParentID: &rootID, MenuType: TypePage, Code: "reports:list", I18nKey: "navigation.systemMenus", Path: &path, ViewKey: &view, IsEnabled: yesno.Yes})
		createdRole := testRole{Code: fmt.Sprintf("menu_service_role_%d", time.Now().UnixNano()), Name: "Menu Service Role", IsDefault: yesno.No, IsEnabled: yesno.Yes}
		if err := tx.WithContext(ctx).Create(&createdRole).Error; err != nil {
			t.Fatal(err)
		}
		if err := tx.WithContext(ctx).Create(&RoleMenu{RoleID: createdRole.ID, MenuID: pageID}).Error; err != nil {
			t.Fatal(err)
		}
		err := service.Update(ctx, pageID, UpdateInput{MenuType: TypeDirectory, I18nKey: "navigation.system", SortOrder: 1})
		if menuServiceErrorCode(err) != CodeMenuStructureConflict {
			t.Fatalf("grant conflict error = %v", err)
		}
	})

	t.Run("enabled subtree cannot move below disabled parent", func(t *testing.T) {
		_, ctx, service := openCleanMenuService(t)
		rootID, _ := service.Create(ctx, CreateInput{MenuType: TypeDirectory, Code: "reports", I18nKey: "navigation.system", IsEnabled: yesno.Yes})
		disabledID, _ := service.Create(ctx, CreateInput{MenuType: TypeDirectory, Code: "disabled", I18nKey: "navigation.system", IsEnabled: yesno.No})
		err := service.Update(ctx, rootID, UpdateInput{ParentID: &disabledID, MenuType: TypeDirectory, I18nKey: "navigation.system", SortOrder: 1})
		if menuServiceErrorCode(err) != CodeMenuParentDisabled {
			t.Fatalf("disabled parent error = %v", err)
		}
	})
}

func TestServiceUpdateProtectsBuiltinFieldsButAllowsOperatorFields(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	if err := service.EnsureBuiltin(ctx); err != nil {
		t.Fatal(err)
	}
	items := loadBuiltinMenus(t, tx, ctx)
	system := items[BuiltinSystemCode]
	list := items[PermissionList]
	create := items[PermissionCreate]
	if err := service.Update(ctx, system.ID, UpdateInput{MenuType: TypeDirectory, I18nKey: system.I18nKey, Icon: stringPointer("Folder"), SortOrder: 77}); err != nil {
		t.Fatalf("update system operator fields: %v", err)
	}
	if err := service.Update(ctx, list.ID, UpdateInput{ParentID: list.ParentID, MenuType: TypePage, I18nKey: list.I18nKey, Path: list.Path, ViewKey: list.ViewKey, Icon: stringPointer("Cpu"), SortOrder: 55}); err != nil {
		t.Fatalf("update list operator fields: %v", err)
	}
	if err := service.Update(ctx, create.ID, UpdateInput{ParentID: create.ParentID, MenuType: TypeAction, I18nKey: create.I18nKey, SortOrder: 44}); err != nil {
		t.Fatalf("update action sort: %v", err)
	}

	tests := []struct {
		name  string
		id    int64
		input UpdateInput
	}{
		{name: "type", id: list.ID, input: UpdateInput{ParentID: list.ParentID, MenuType: TypeDirectory, I18nKey: list.I18nKey, Icon: stringPointer("Folder")}},
		{name: "parent", id: list.ID, input: UpdateInput{MenuType: TypePage, I18nKey: list.I18nKey, Path: list.Path, ViewKey: list.ViewKey}},
		{name: "i18n", id: list.ID, input: UpdateInput{ParentID: list.ParentID, MenuType: TypePage, I18nKey: "permission.menuCreate", Path: list.Path, ViewKey: list.ViewKey}},
		{name: "path", id: list.ID, input: UpdateInput{ParentID: list.ParentID, MenuType: TypePage, I18nKey: list.I18nKey, Path: stringPointer("/other"), ViewKey: list.ViewKey}},
		{name: "view", id: list.ID, input: UpdateInput{ParentID: list.ParentID, MenuType: TypePage, I18nKey: list.I18nKey, Path: list.Path, ViewKey: stringPointer("other-view")}},
		{name: "action icon", id: create.ID, input: UpdateInput{ParentID: create.ParentID, MenuType: TypeAction, I18nKey: create.I18nKey, Icon: stringPointer("Key")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := service.Update(ctx, test.id, test.input); menuServiceErrorCode(err) != CodeMenuBuiltinProtected {
				t.Fatalf("Update() error = %v", err)
			}
		})
	}
}

func TestServiceUpdateRollsBackDatabaseWriteFailure(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	id, err := service.Create(ctx, CreateInput{MenuType: TypeDirectory, Code: "reports", I18nKey: "navigation.system", Icon: stringPointer("Folder"), SortOrder: 10, IsEnabled: yesno.Yes})
	if err != nil {
		t.Fatal(err)
	}
	var before Menu
	if err := tx.WithContext(ctx).First(&before, id).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Exec(`ALTER TABLE sys_menu ADD CONSTRAINT ck_test_menu_update_rollback CHECK (i18n_key <> 'navigation.systemMenus')`).Error; err != nil {
		t.Fatal(err)
	}
	err = service.Update(ctx, id, UpdateInput{MenuType: TypeDirectory, I18nKey: "navigation.systemMenus", Icon: stringPointer("Cpu"), SortOrder: 99})
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
	rootID, _ := service.Create(ctx, CreateInput{MenuType: TypeDirectory, Code: "reports", I18nKey: "navigation.system", IsEnabled: yesno.Yes})
	childID, _ := service.Create(ctx, CreateInput{ParentID: &rootID, MenuType: TypeDirectory, Code: "reports:section", I18nKey: "navigation.system", IsEnabled: yesno.Yes})
	path := "/reports"
	view := "system-menus"
	pageID, _ := service.Create(ctx, CreateInput{ParentID: &childID, MenuType: TypePage, Code: "reports:list", I18nKey: "navigation.systemMenus", Path: &path, ViewKey: &view, IsEnabled: yesno.Yes})
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
		rootID, _ := service.Create(ctx, CreateInput{MenuType: TypeDirectory, Code: "reports", I18nKey: "navigation.system", IsEnabled: yesno.No})
		childID, _ := service.Create(ctx, CreateInput{ParentID: &rootID, MenuType: TypeDirectory, Code: "reports:section", I18nKey: "navigation.system", IsEnabled: yesno.No})
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
		rootID, _ := service.Create(ctx, CreateInput{MenuType: TypeDirectory, Code: "reports", I18nKey: "navigation.system", IsEnabled: yesno.No})
		childID, _ := service.Create(ctx, CreateInput{ParentID: &rootID, MenuType: TypeDirectory, Code: "reports:section", I18nKey: "navigation.system", IsEnabled: yesno.No})
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

func TestServiceUpdateStatusAndDeleteProtectBuiltinMenus(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	if err := service.EnsureBuiltin(ctx); err != nil {
		t.Fatal(err)
	}
	items := loadBuiltinMenus(t, tx, ctx)
	if err := service.UpdateStatus(ctx, items[BuiltinSystemCode].ID, yesno.No); menuServiceErrorCode(err) != CodeMenuBuiltinProtected {
		t.Fatalf("builtin disable error = %v", err)
	}
	if err := service.Delete(ctx, items[PermissionList].ID); menuServiceErrorCode(err) != CodeMenuBuiltinProtected {
		t.Fatalf("builtin delete error = %v", err)
	}
}

func TestServiceDeleteSoftDeletesSubtreeAndRoleMenusTogether(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	rootID, _ := service.Create(ctx, CreateInput{MenuType: TypeDirectory, Code: "reports", I18nKey: "navigation.system", IsEnabled: yesno.Yes})
	path := "/reports"
	view := "system-menus"
	pageID, _ := service.Create(ctx, CreateInput{ParentID: &rootID, MenuType: TypePage, Code: "reports:list", I18nKey: "navigation.systemMenus", Path: &path, ViewKey: &view, IsEnabled: yesno.Yes})
	actionID, _ := service.Create(ctx, CreateInput{ParentID: &pageID, MenuType: TypeAction, Code: "reports:create", I18nKey: "permission.menuCreate", IsEnabled: yesno.Yes})
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
	listed, err := service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("deleted menus remain in List(): %+v", listed)
	}
}

func TestServiceDeleteRollsBackRoleMenusWhenMenuWriteFails(t *testing.T) {
	tx, ctx, service := openCleanMenuService(t)
	rootID, _ := service.Create(ctx, CreateInput{MenuType: TypeDirectory, Code: "reports", I18nKey: "navigation.system", IsEnabled: yesno.Yes})
	createdRole := testRole{Code: fmt.Sprintf("menu_delete_rollback_role_%d", time.Now().UnixNano()), Name: "Menu Delete Rollback Role", IsDefault: yesno.No, IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&createdRole).Error; err != nil {
		t.Fatal(err)
	}
	grant := RoleMenu{RoleID: createdRole.ID, MenuID: rootID}
	if err := tx.WithContext(ctx).Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Exec(`ALTER TABLE sys_menu ADD CONSTRAINT ck_test_menu_delete_rollback CHECK (deleted_at IS NULL)`).Error; err != nil {
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

func openCleanMenuService(t *testing.T) (*gorm.DB, context.Context, *Service) {
	t.Helper()
	tx, ctx := openMenuTransaction(t)
	if err := tx.WithContext(ctx).Unscoped().Exec("DELETE FROM sys_role_menu").Error; err != nil {
		t.Fatalf("clear role menus: %v", err)
	}
	if err := tx.WithContext(ctx).Unscoped().Exec("DELETE FROM sys_menu").Error; err != nil {
		t.Fatalf("clear menus: %v", err)
	}
	return tx, ctx, NewService(NewRepository(tx))
}

func menuServiceErrorCode(err error) int {
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		return 0
	}
	return appErr.Code
}
