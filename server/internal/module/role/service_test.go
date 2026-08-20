package role_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/menu"
	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
)

func TestServiceListValidatesAndNormalizesQuery(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := role.NewService(role.NewRepository(tx))
	unique := " service_list_keyword "
	created := role.Role{Code: "service_list_keyword", Name: "Service List", IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&created).Error; err != nil {
		t.Fatal(err)
	}
	result, err := service.List(ctx, role.ListQuery{Page: 1, PageSize: 20, Keyword: unique})
	if err != nil || result.Page != 1 || result.PageSize != 20 || result.Total != 1 || len(result.List) != 1 {
		t.Fatalf("List() = %+v,%v", result, err)
	}

	invalidEnabled := yesno.Value(2)
	for _, query := range []role.ListQuery{
		{Page: 0, PageSize: 20},
		{Page: 1, PageSize: 0},
		{Page: 1, PageSize: 101},
		{Page: 1, PageSize: 20, IsEnabled: &invalidEnabled},
		{Page: 1, PageSize: 20, Keyword: strings.Repeat("界", 65)},
	} {
		_, err := service.List(ctx, query)
		var appErr *apperror.Error
		if !errors.As(err, &appErr) || appErr.Code != apperror.CodeInvalidRequest {
			t.Errorf("List(%+v) error = %v", query, err)
		}
	}
}

func TestServiceListReturnsNonNilEmptyPage(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	result, err := role.NewService(role.NewRepository(tx)).List(ctx, role.ListQuery{
		Page: 1, PageSize: 20, Keyword: "missing_service_list_value",
	})
	if err != nil || result.List == nil || result.Total != 0 {
		t.Fatalf("empty List() = %#v,%v", result, err)
	}
}

func TestServiceCreateNormalizesAndInitializesRole(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := role.NewService(role.NewRepository(tx))
	unique := time.Now().UnixNano()
	id, err := service.Create(ctx, role.CreateInput{Code: fmt.Sprintf(" ai_tester_%d ", unique), Name: " AI 测试员 "})
	if err != nil {
		t.Fatal(err)
	}
	var stored role.Role
	if err := tx.WithContext(ctx).First(&stored, id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Code != fmt.Sprintf("ai_tester_%d", unique) || stored.Name != "AI 测试员" || stored.IsDefault != yesno.No || stored.IsEnabled != yesno.Yes {
		t.Fatalf("created role = %+v", stored)
	}
	var grantCount int64
	if err := tx.WithContext(ctx).Model(&menu.RoleMenu{}).Where("role_id = ?", id).Count(&grantCount).Error; err != nil || grantCount != 0 {
		t.Fatalf("initial grants = %d,%v", grantCount, err)
	}
}

func TestServiceCreateRejectsInvalidAndConflictingProfiles(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := role.NewService(role.NewRepository(tx))
	for _, input := range []role.CreateInput{
		{Code: "ab", Name: "Valid"},
		{Code: "Bad", Name: "Valid"},
		{Code: role.CodeSuperAdmin, Name: "Valid"},
		{Code: "valid_code", Name: ""},
		{Code: "valid_code", Name: strings.Repeat("界", 65)},
	} {
		if _, err := service.Create(ctx, input); roleErrorCode(err) != role.CodeRoleInvalidState {
			t.Errorf("Create(%+v) error = %v", input, err)
		}
	}

	for _, conflict := range []struct {
		name     string
		input    func(string, string) role.CreateInput
		wantCode int
	}{
		{name: "code", input: func(code, name string) role.CreateInput { return role.CreateInput{Code: code, Name: name + " Other"} }, wantCode: role.CodeRoleCodeConflict},
		{name: "name", input: func(code, name string) role.CreateInput { return role.CreateInput{Code: code + "_other", Name: name} }, wantCode: role.CodeRoleNameConflict},
	} {
		t.Run(conflict.name, func(t *testing.T) {
			conflictTX, conflictContext := openRoleTransaction(t)
			conflictService := role.NewService(role.NewRepository(conflictTX))
			unique := time.Now().UnixNano()
			code := fmt.Sprintf("profile_%d", unique)
			name := fmt.Sprintf("Profile %d", unique)
			if _, err := conflictService.Create(conflictContext, role.CreateInput{Code: code, Name: name}); err != nil {
				t.Fatal(err)
			}
			if _, err := conflictService.Create(conflictContext, conflict.input(code, name)); roleErrorCode(err) != conflict.wantCode {
				t.Fatalf("conflict error = %v", err)
			}
		})
	}
}

func TestServiceUpdateChangesOnlyCustomRoleName(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := role.NewService(role.NewRepository(tx))
	unique := time.Now().UnixNano()
	code := fmt.Sprintf("update_%d", unique)
	id, err := service.Create(ctx, role.CreateInput{Code: code, Name: "Before"})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Update(ctx, id, role.UpdateInput{Name: " After "}); err != nil {
		t.Fatal(err)
	}
	var stored role.Role
	if err := tx.WithContext(ctx).First(&stored, id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Code != code || stored.Name != "After" || stored.IsEnabled != yesno.Yes || stored.IsDefault != yesno.No {
		t.Fatalf("updated role = %+v", stored)
	}
	updatedAt := stored.UpdatedAt
	if err := service.Update(ctx, id, role.UpdateInput{Name: "After"}); err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).First(&stored, id).Error; err != nil || !stored.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("same-name update rewrote role: %+v,%v", stored, err)
	}
}

func TestServiceUpdateProtectsSystemAndMissingRoles(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := role.NewService(role.NewRepository(tx))
	if err := service.EnsureSystemRoles(ctx); err != nil {
		t.Fatal(err)
	}
	superAdmin, err := role.NewRepository(tx).FindByCode(ctx, role.CodeSuperAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Update(ctx, superAdmin.ID, role.UpdateInput{Name: "Changed"}); roleErrorCode(err) != role.CodeRoleSystemProtected {
		t.Fatalf("system update error = %v", err)
	}
	if err := service.Update(ctx, 999999999, role.UpdateInput{Name: "Changed"}); roleErrorCode(err) != role.CodeRoleNotFound {
		t.Fatalf("missing update error = %v", err)
	}
}

func TestServiceUpdateStatusProtectsRolesAndPreservesRelations(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := role.NewService(role.NewRepository(tx))
	if err := service.EnsureSystemRoles(ctx); err != nil {
		t.Fatal(err)
	}
	repository := role.NewRepository(tx)
	superAdmin, _ := repository.FindByCode(ctx, role.CodeSuperAdmin)
	registered, _ := repository.FindByCode(ctx, role.CodeRegisteredUser)
	if err := service.UpdateStatus(ctx, superAdmin.ID, yesno.No); roleErrorCode(err) != role.CodeRoleSystemProtected {
		t.Fatalf("disable super admin error = %v", err)
	}
	if err := service.UpdateStatus(ctx, registered.ID, yesno.No); roleErrorCode(err) != role.CodeRoleDefaultProtected {
		t.Fatalf("disable default error = %v", err)
	}

	customID, err := service.Create(ctx, role.CreateInput{Code: fmt.Sprintf("status_%d", time.Now().UnixNano()), Name: fmt.Sprintf("Status %d", time.Now().UnixNano())})
	if err != nil {
		t.Fatal(err)
	}
	createdUser := user.User{Username: fmt.Sprintf("status_user_%d", time.Now().UnixNano()), Email: fmt.Sprintf("status_%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&createdUser).Error; err != nil {
		t.Fatal(err)
	}
	userRole := role.UserRole{UserID: createdUser.ID, RoleID: customID}
	if err := tx.WithContext(ctx).Create(&userRole).Error; err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/status-%d", time.Now().UnixNano())
	view := "system-menus"
	page := menu.Menu{MenuType: menu.TypePage, Code: fmt.Sprintf("status:%d:list", time.Now().UnixNano()), I18nKey: "navigation.systemMenus", Path: &path, ViewKey: &view, IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&page).Error; err != nil {
		t.Fatal(err)
	}
	grant := menu.RoleMenu{RoleID: customID, MenuID: page.ID}
	if err := tx.WithContext(ctx).Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateStatus(ctx, customID, yesno.No); err != nil {
		t.Fatal(err)
	}
	var stored role.Role
	if err := tx.WithContext(ctx).First(&stored, customID).Error; err != nil || stored.IsEnabled != yesno.No {
		t.Fatalf("disabled role = %+v,%v", stored, err)
	}
	disabledAt := stored.UpdatedAt
	if err := service.UpdateStatus(ctx, customID, yesno.No); err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).First(&stored, customID).Error; err != nil || !stored.UpdatedAt.Equal(disabledAt) {
		t.Fatalf("same status rewrote role = %+v,%v", stored, err)
	}
	if err := service.UpdateStatus(ctx, customID, yesno.Yes); err != nil {
		t.Fatal(err)
	}
	var relationCount, grantCount int64
	if err := tx.WithContext(ctx).Model(&role.UserRole{}).Where("id = ?", userRole.ID).Count(&relationCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Model(&menu.RoleMenu{}).Where("id = ?", grant.ID).Count(&grantCount).Error; err != nil {
		t.Fatal(err)
	}
	if relationCount != 1 || grantCount != 1 {
		t.Fatalf("status change removed relations: users=%d grants=%d", relationCount, grantCount)
	}
}

func TestServiceSetDefaultMaintainsExactlyOneEnabledDefault(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := role.NewService(role.NewRepository(tx))
	if err := service.EnsureSystemRoles(ctx); err != nil {
		t.Fatal(err)
	}
	customID, err := service.Create(ctx, role.CreateInput{Code: fmt.Sprintf("default_%d", time.Now().UnixNano()), Name: fmt.Sprintf("Default %d", time.Now().UnixNano())})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SetDefault(ctx, customID); err != nil {
		t.Fatal(err)
	}
	found, err := role.NewRepository(tx).FindDefault(ctx)
	if err != nil || found.ID != customID {
		t.Fatalf("default = %+v,%v", found, err)
	}
	updatedAt := found.UpdatedAt
	if err := service.SetDefault(ctx, customID); err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).First(&found, customID).Error; err != nil || !found.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("same default rewrote role = %+v,%v", found, err)
	}

	superAdmin, _ := role.NewRepository(tx).FindByCode(ctx, role.CodeSuperAdmin)
	if err := service.SetDefault(ctx, superAdmin.ID); roleErrorCode(err) != role.CodeRoleSystemProtected {
		t.Fatalf("super admin default error = %v", err)
	}
	if err := service.UpdateStatus(ctx, customID, yesno.No); roleErrorCode(err) != role.CodeRoleDefaultProtected {
		t.Fatalf("disable selected default error = %v", err)
	}
	otherID, err := service.Create(ctx, role.CreateInput{Code: fmt.Sprintf("disabled_default_%d", time.Now().UnixNano()), Name: fmt.Sprintf("Disabled Default %d", time.Now().UnixNano())})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateStatus(ctx, otherID, yesno.No); err != nil {
		t.Fatal(err)
	}
	if err := service.SetDefault(ctx, otherID); roleErrorCode(err) != role.CodeRoleInvalidState {
		t.Fatalf("disabled default error = %v", err)
	}
	if err := service.SetDefault(ctx, 999999999); roleErrorCode(err) != role.CodeRoleNotFound {
		t.Fatalf("missing default error = %v", err)
	}
}

func TestServiceDeleteProtectsSystemDefaultAndAttachedRoles(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := role.NewService(role.NewRepository(tx))
	if err := service.EnsureSystemRoles(ctx); err != nil {
		t.Fatal(err)
	}
	superAdmin, _ := role.NewRepository(tx).FindByCode(ctx, role.CodeSuperAdmin)
	registered, _ := role.NewRepository(tx).FindByCode(ctx, role.CodeRegisteredUser)
	for _, id := range []int64{superAdmin.ID, registered.ID} {
		if err := service.Delete(ctx, id); roleErrorCode(err) != role.CodeRoleSystemProtected {
			t.Fatalf("delete system role %d error = %v", id, err)
		}
	}
	if err := service.Delete(ctx, 999999999); roleErrorCode(err) != role.CodeRoleNotFound {
		t.Fatalf("delete missing error = %v", err)
	}

	customID, err := service.Create(ctx, role.CreateInput{Code: fmt.Sprintf("delete_attached_%d", time.Now().UnixNano()), Name: fmt.Sprintf("Delete Attached %d", time.Now().UnixNano())})
	if err != nil {
		t.Fatal(err)
	}
	createdUser := user.User{Username: fmt.Sprintf("delete_user_%d", time.Now().UnixNano()), Email: fmt.Sprintf("delete_user_%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", IsEnabled: yesno.No}
	if err := tx.WithContext(ctx).Create(&createdUser).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Create(&role.UserRole{UserID: createdUser.ID, RoleID: customID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, customID); roleErrorCode(err) != role.CodeRoleUsersAttached {
		t.Fatalf("delete attached role error = %v", err)
	}
}

func TestServiceDeleteSoftDeletesRoleAndGrantsWithOneTimestamp(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := role.NewService(role.NewRepository(tx))
	roleID, err := service.Create(ctx, role.CreateInput{Code: fmt.Sprintf("delete_%d", time.Now().UnixNano()), Name: fmt.Sprintf("Delete %d", time.Now().UnixNano())})
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/delete-%d", time.Now().UnixNano())
	view := "system-menus"
	page := menu.Menu{MenuType: menu.TypePage, Code: fmt.Sprintf("delete:%d:list", time.Now().UnixNano()), I18nKey: "navigation.systemMenus", Path: &path, ViewKey: &view, IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&page).Error; err != nil {
		t.Fatal(err)
	}
	grant := menu.RoleMenu{RoleID: roleID, MenuID: page.ID}
	if err := tx.WithContext(ctx).Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, roleID); err != nil {
		t.Fatal(err)
	}
	var deletedRole role.Role
	var deletedGrant menu.RoleMenu
	if err := tx.WithContext(ctx).Unscoped().First(&deletedRole, roleID).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Unscoped().First(&deletedGrant, grant.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !deletedRole.DeletedAt.Valid || !deletedGrant.DeletedAt.Valid ||
		!deletedRole.DeletedAt.Time.Equal(deletedGrant.DeletedAt.Time) ||
		!deletedRole.UpdatedAt.Equal(deletedRole.DeletedAt.Time) || !deletedGrant.UpdatedAt.Equal(deletedRole.DeletedAt.Time) {
		t.Fatalf("soft-delete timestamps: role=%+v grant=%+v", deletedRole, deletedGrant)
	}
}

func TestServiceDeleteRollsBackWhenRoleWriteFails(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := role.NewService(role.NewRepository(tx))
	roleID, err := service.Create(ctx, role.CreateInput{Code: fmt.Sprintf("delete_rollback_%d", time.Now().UnixNano()), Name: fmt.Sprintf("Delete Rollback %d", time.Now().UnixNano())})
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/delete-rollback-%d", time.Now().UnixNano())
	view := "system-menus"
	page := menu.Menu{MenuType: menu.TypePage, Code: fmt.Sprintf("delete:rollback:%d", time.Now().UnixNano()), I18nKey: "navigation.systemMenus", Path: &path, ViewKey: &view, IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&page).Error; err != nil {
		t.Fatal(err)
	}
	grant := menu.RoleMenu{RoleID: roleID, MenuID: page.ID}
	if err := tx.WithContext(ctx).Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Exec(`ALTER TABLE sys_role ADD CONSTRAINT ck_test_role_delete_rollback CHECK (deleted_at IS NULL)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, roleID); roleErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("delete rollback error = %v", err)
	}
	var activeRoleCount, activeGrantCount int64
	if err := tx.WithContext(ctx).Model(&role.Role{}).Where("id = ?", roleID).Count(&activeRoleCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Model(&menu.RoleMenu{}).Where("id = ?", grant.ID).Count(&activeGrantCount).Error; err != nil {
		t.Fatal(err)
	}
	if activeRoleCount != 1 || activeGrantCount != 1 {
		t.Fatalf("rollback counts: role=%d grant=%d", activeRoleCount, activeGrantCount)
	}
}

func TestServicePermissionsQueriesAndSavesMinimalDirectGrants(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := role.NewService(role.NewRepository(tx))
	menuService := menu.NewService(menu.NewRepository(tx))
	if err := menuService.EnsureBuiltin(ctx); err != nil {
		t.Fatal(err)
	}
	roleID, err := service.Create(ctx, role.CreateInput{Code: fmt.Sprintf("permission_%d", time.Now().UnixNano()), Name: fmt.Sprintf("Permission %d", time.Now().UnixNano())})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateStatus(ctx, roleID, yesno.No); err != nil {
		t.Fatal(err)
	}
	var page, action menu.Menu
	if err := tx.WithContext(ctx).Where("code = ?", menu.PermissionRoleList).Take(&page).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Where("code = ?", menu.PermissionRoleAuthorize).Take(&action).Error; err != nil {
		t.Fatal(err)
	}
	count, err := service.UpdatePermissions(ctx, roleID, []int64{page.ID, action.ID})
	if err != nil || count != 1 {
		t.Fatalf("UpdatePermissions() = %d,%v", count, err)
	}
	permissions, err := service.Permissions(ctx, roleID)
	if err != nil || len(permissions.MenuTree) == 0 || !reflect.DeepEqual(permissions.MenuIDs, []int64{action.ID}) {
		t.Fatalf("Permissions() = %+v,%v", permissions, err)
	}
	var before menu.RoleMenu
	if err := tx.WithContext(ctx).Where("role_id = ?", roleID).Take(&before).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdatePermissions(ctx, roleID, []int64{action.ID}); err != nil {
		t.Fatal(err)
	}
	var after menu.RoleMenu
	if err := tx.WithContext(ctx).First(&after, before.ID).Error; err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("unchanged grant was rewritten: before=%+v after=%+v err=%v", before, after, err)
	}
	if count, err := service.UpdatePermissions(ctx, roleID, []int64{}); err != nil || count != 0 {
		t.Fatalf("clear permissions = %d,%v", count, err)
	}
}

func TestServicePermissionsRejectsSuperAdminAndInvalidMenus(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := role.NewService(role.NewRepository(tx))
	if err := service.EnsureSystemRoles(ctx); err != nil {
		t.Fatal(err)
	}
	superAdmin, _ := role.NewRepository(tx).FindByCode(ctx, role.CodeSuperAdmin)
	if _, err := service.Permissions(ctx, superAdmin.ID); roleErrorCode(err) != role.CodeRoleSuperAdminAuthorization {
		t.Fatalf("super admin permissions error = %v", err)
	}
	if _, err := service.UpdatePermissions(ctx, superAdmin.ID, []int64{}); roleErrorCode(err) != role.CodeRoleSuperAdminAuthorization {
		t.Fatalf("super admin update permissions error = %v", err)
	}
	customID, err := service.Create(ctx, role.CreateInput{Code: fmt.Sprintf("invalid_grant_%d", time.Now().UnixNano()), Name: fmt.Sprintf("Invalid Grant %d", time.Now().UnixNano())})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdatePermissions(ctx, customID, []int64{999999999}); roleErrorCode(err) != role.CodeRoleInvalidPermission {
		t.Fatalf("missing menu grant error = %v", err)
	}
}

func roleErrorCode(err error) int {
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		return 0
	}
	return appErr.Code
}
