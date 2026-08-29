package role_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/module/rbac/access"
	"admin/server/internal/module/rbac/menu"
	"admin/server/internal/module/rbac/role"
	"admin/server/internal/module/rbac/state"
	"admin/server/internal/module/user/account"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestServiceListValidatesAndNormalizesQuery(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := newRoleTestService(t, role.NewRepository(tx))
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
	result, err := newRoleTestService(t, role.NewRepository(tx)).List(ctx, role.ListQuery{
		Page: 1, PageSize: 20, Keyword: "missing_service_list_value",
	})
	if err != nil || result.List == nil || result.Total != 0 {
		t.Fatalf("empty List() = %#v,%v", result, err)
	}
}

func TestServiceCreateNormalizesAndInitializesRole(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := newRoleTestService(t, role.NewRepository(tx))
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
	service := newRoleTestService(t, role.NewRepository(tx))
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
			conflictService := newRoleTestService(t, role.NewRepository(conflictTX))
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
	service := newRoleTestService(t, role.NewRepository(tx))
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
	service := newRoleTestService(t, role.NewRepository(tx))
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
	service, accessStates, _ := newRoleMutationTestService(t, role.NewRepository(tx))
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
	createdUser := account.User{Username: fmt.Sprintf("status_user_%d", time.Now().UnixNano()), Email: fmt.Sprintf("status_%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&createdUser).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Create(&access.Version{UserID: createdUser.ID, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	userRole := role.UserRole{UserID: createdUser.ID, RoleID: customID}
	if err := tx.WithContext(ctx).Create(&userRole).Error; err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/status-%d", time.Now().UnixNano())
	componentPath := "access/menus"
	page := menu.Menu{PlatformID: roleTestAdminPlatformID(t, tx, ctx), MenuType: menu.TypePage, Name: "Status", Code: fmt.Sprintf("status:%d:list", time.Now().UnixNano()), I18nKey: roleTestStringPointer("navigation.systemMenus"), Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No}
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
	assertRoleAccessState(t, accessStates, createdUser.ID, 2)
	var stored role.Role
	if err := tx.WithContext(ctx).First(&stored, customID).Error; err != nil || stored.IsEnabled != yesno.No {
		t.Fatalf("disabled role = %+v,%v", stored, err)
	}
	disabledAt := stored.UpdatedAt
	if err := service.UpdateStatus(ctx, customID, yesno.No); err != nil {
		t.Fatal(err)
	}
	if got := readRoleAccessVersion(t, tx, ctx, createdUser.ID); got != 2 {
		t.Fatalf("same-status access version = %d", got)
	}
	if err := tx.WithContext(ctx).First(&stored, customID).Error; err != nil || !stored.UpdatedAt.Equal(disabledAt) {
		t.Fatalf("same status rewrote role = %+v,%v", stored, err)
	}
	if err := service.UpdateStatus(ctx, customID, yesno.Yes); err != nil {
		t.Fatal(err)
	}
	assertRoleAccessState(t, accessStates, createdUser.ID, 3)
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

func TestServiceRoleProfileAndDefaultChangesDoNotAdvanceAccessVersion(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := newRoleTestService(t, role.NewRepository(tx))
	if err := service.EnsureSystemRoles(ctx); err != nil {
		t.Fatal(err)
	}
	roleID, err := service.Create(ctx, role.CreateInput{Code: fmt.Sprintf("profile_only_%d", time.Now().UnixNano()), Name: "Before"})
	if err != nil {
		t.Fatal(err)
	}
	boundUser := createRoleAccessUser(t, tx, ctx, roleID, yesno.Yes, false)
	if err := service.Update(ctx, roleID, role.UpdateInput{Name: "After"}); err != nil {
		t.Fatal(err)
	}
	if err := service.SetDefault(ctx, roleID); err != nil {
		t.Fatal(err)
	}
	if got := readRoleAccessVersion(t, tx, ctx, boundUser.ID); got != 1 {
		t.Fatalf("profile/default access version = %d", got)
	}
}

func TestServiceUpdateStatusRedisFailurePreventsPostgreSQLMutation(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service, _, redisClient := newRoleMutationTestService(t, role.NewRepository(tx))
	roleID, err := service.Create(ctx, role.CreateInput{Code: fmt.Sprintf("redis_status_%d", time.Now().UnixNano()), Name: "Redis status"})
	if err != nil {
		t.Fatal(err)
	}
	boundUser := createRoleAccessUser(t, tx, ctx, roleID, yesno.Yes, false)
	if err := redisClient.Close(); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateStatus(ctx, roleID, yesno.No); roleErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	var stored role.Role
	if err := tx.WithContext(ctx).Take(&stored, roleID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.IsEnabled != yesno.Yes || readRoleAccessVersion(t, tx, ctx, boundUser.ID) != 1 {
		t.Fatalf("PostgreSQL mutated without Redis coordination: role=%+v", stored)
	}
}

func TestServiceUpdateStatusRollbackRestoresAccessStateAndVersion(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service, accessStates, _ := newRoleMutationTestService(t, role.NewRepository(tx))
	roleID, err := service.Create(ctx, role.CreateInput{Code: fmt.Sprintf("rollback_status_%d", time.Now().UnixNano()), Name: "Rollback status"})
	if err != nil {
		t.Fatal(err)
	}
	boundUser := createRoleAccessUser(t, tx, ctx, roleID, yesno.Yes, false)
	if err := tx.WithContext(ctx).Exec(`ALTER TABLE rbac_role ADD CONSTRAINT ck_test_role_status_rollback CHECK (is_enabled = 1)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateStatus(ctx, roleID, yesno.No); roleErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if got := readRoleAccessVersion(t, tx, ctx, boundUser.ID); got != 1 {
		t.Fatalf("rolled-back access version = %d", got)
	}
	assertRoleAccessState(t, accessStates, boundUser.ID, 1)
}

func TestConcurrentRoleMutationRechecksChangedCandidate(t *testing.T) {
	db, ctx := openRoleDatabase(t)
	setupService := newRoleTestService(t, role.NewRepository(db))
	if err := setupService.EnsureSystemRoles(ctx); err != nil {
		t.Fatal(err)
	}
	roleID, err := setupService.Create(ctx, role.CreateInput{Code: fmt.Sprintf("candidate_%d", time.Now().UnixNano()), Name: "Candidate"})
	if err != nil {
		t.Fatal(err)
	}
	boundUser := createRoleAccessUser(t, db, ctx, roleID, yesno.Yes, false)
	var relation role.UserRole
	if err := db.WithContext(ctx).Where("user_id = ? AND role_id = ?", boundUser.ID, roleID).Take(&relation).Error; err != nil {
		t.Fatal(err)
	}

	blocker := db.WithContext(ctx).Begin()
	if blocker.Error != nil {
		t.Fatal(blocker.Error)
	}
	t.Cleanup(func() { _ = blocker.Rollback().Error })
	if _, err := role.NewRepository(blocker).LockActiveRole(ctx, roleID); err != nil {
		t.Fatal(err)
	}

	service, accessStates, _ := newRoleMutationTestService(t, role.NewRepository(db))
	done := make(chan error, 1)
	go func() { done <- service.UpdateStatus(ctx, roleID, yesno.No) }()
	waitForRoleAccessState(t, accessStates, boundUser.ID, accessstate.StateInvalidating)
	if err := db.WithContext(ctx).Delete(&relation).Error; err != nil {
		t.Fatal(err)
	}
	if err := blocker.Commit().Error; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("UpdateStatus() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("role mutation did not finish")
	}
	var stored role.Role
	if err := db.WithContext(ctx).Take(&stored, roleID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.IsEnabled != yesno.No {
		t.Fatalf("role status = %d", stored.IsEnabled)
	}
	if got := readRoleAccessVersion(t, db, ctx, boundUser.ID); got != 1 {
		t.Fatalf("removed candidate access version = %d", got)
	}
	assertRoleAccessState(t, accessStates, boundUser.ID, 1)
}

func TestServiceUpdateStatusPublishFailureLeavesCommittedVersionUnreachable(t *testing.T) {
	db, ctx := openRoleDatabase(t)
	service, accessStates, redisClient := newRoleMutationTestService(t, role.NewRepository(db))
	roleID, err := service.Create(ctx, role.CreateInput{Code: fmt.Sprintf("publish_%d", time.Now().UnixNano()), Name: "Publish"})
	if err != nil {
		t.Fatal(err)
	}
	boundUser := createRoleAccessUser(t, db, ctx, roleID, yesno.Yes, false)
	if err := db.WithContext(ctx).Exec(`
		CREATE FUNCTION delay_role_status_publish() RETURNS trigger AS $$
		BEGIN
			PERFORM pg_sleep(0.5);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER delay_role_status_publish
		BEFORE UPDATE OF is_enabled ON rbac_role
		FOR EACH ROW EXECUTE FUNCTION delay_role_status_publish()`).Error; err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- service.UpdateStatus(ctx, roleID, yesno.No) }()
	waitForRoleAccessState(t, accessStates, boundUser.ID, accessstate.StateInvalidating)
	if err := redisClient.Delete(ctx, accessstate.StateKey(boundUser.ID)); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if roleErrorCode(err) != apperror.CodeDependencyUnavailable {
			t.Fatalf("UpdateStatus() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("role mutation did not finish")
	}
	var stored role.Role
	if err := db.WithContext(ctx).Take(&stored, roleID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.IsEnabled != yesno.No || readRoleAccessVersion(t, db, ctx, boundUser.ID) != 2 {
		t.Fatalf("committed PostgreSQL state = %+v", stored)
	}
	if _, found, err := accessStates.Read(ctx, boundUser.ID); err != nil || found {
		t.Fatalf("old access state remained reachable: found=%v error=%v", found, err)
	}
}

func TestServiceSetDefaultMaintainsExactlyOneEnabledDefault(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := newRoleTestService(t, role.NewRepository(tx))
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
	service, accessStates, _ := newRoleMutationTestService(t, role.NewRepository(tx))
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
	createdUser := account.User{Username: fmt.Sprintf("delete_user_%d", time.Now().UnixNano()), Email: fmt.Sprintf("delete_user_%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&createdUser).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Create(&access.Version{UserID: createdUser.ID, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Create(&role.UserRole{UserID: createdUser.ID, RoleID: customID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, customID); roleErrorCode(err) != role.CodeRoleUsersAttached {
		t.Fatalf("delete attached role error = %v", err)
	}
	assertRoleAccessState(t, accessStates, createdUser.ID, 1)
}

func TestServiceDeleteSoftDeletesRoleAndGrantsWithOneTimestamp(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := newRoleTestService(t, role.NewRepository(tx))
	roleID, err := service.Create(ctx, role.CreateInput{Code: fmt.Sprintf("delete_%d", time.Now().UnixNano()), Name: fmt.Sprintf("Delete %d", time.Now().UnixNano())})
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/delete-%d", time.Now().UnixNano())
	componentPath := "access/menus"
	page := menu.Menu{PlatformID: roleTestAdminPlatformID(t, tx, ctx), MenuType: menu.TypePage, Name: "Delete", Code: fmt.Sprintf("delete:%d:list", time.Now().UnixNano()), I18nKey: roleTestStringPointer("navigation.systemMenus"), Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No}
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
	service := newRoleTestService(t, role.NewRepository(tx))
	roleID, err := service.Create(ctx, role.CreateInput{Code: fmt.Sprintf("delete_rollback_%d", time.Now().UnixNano()), Name: fmt.Sprintf("Delete Rollback %d", time.Now().UnixNano())})
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/delete-rollback-%d", time.Now().UnixNano())
	componentPath := "access/menus"
	page := menu.Menu{PlatformID: roleTestAdminPlatformID(t, tx, ctx), MenuType: menu.TypePage, Name: "Delete rollback", Code: fmt.Sprintf("delete:rollback:%d", time.Now().UnixNano()), I18nKey: roleTestStringPointer("navigation.systemMenus"), Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No}
	if err := tx.WithContext(ctx).Create(&page).Error; err != nil {
		t.Fatal(err)
	}
	grant := menu.RoleMenu{RoleID: roleID, MenuID: page.ID}
	if err := tx.WithContext(ctx).Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Exec(`ALTER TABLE rbac_role ADD CONSTRAINT ck_test_role_delete_rollback CHECK (deleted_at IS NULL)`).Error; err != nil {
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
	service, accessStates, _ := newRoleMutationTestService(t, role.NewRepository(tx))
	adminPlatformID := roleTestAdminPlatformID(t, tx, ctx)
	canvasPlatform := createRoleTestPlatform(t, tx, ctx, "canvas", "Canvas", yesno.No)
	root := menu.Menu{PlatformID: adminPlatformID, MenuType: menu.TypeDirectory, Name: "权限与认证", Code: "access", I18nKey: roleTestStringPointer("navigation.access"), IsEnabled: yesno.Yes, IsHidden: yesno.No}
	if err := tx.WithContext(ctx).Create(&root).Error; err != nil {
		t.Fatal(err)
	}
	path := "/system/roles"
	componentPath := "system/roles"
	page := menu.Menu{PlatformID: adminPlatformID, ParentID: &root.ID, MenuType: menu.TypePage, Name: "角色管理", Code: role.PermissionList, I18nKey: roleTestStringPointer("navigation.accessRoles"), Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No}
	if err := tx.WithContext(ctx).Create(&page).Error; err != nil {
		t.Fatal(err)
	}
	action := menu.Menu{PlatformID: adminPlatformID, ParentID: &page.ID, MenuType: menu.TypeAction, Name: "配置角色权限", Code: role.PermissionAuthorize, IsEnabled: yesno.Yes, IsHidden: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&action).Error; err != nil {
		t.Fatal(err)
	}
	canvasPath := "/test"
	canvasComponentPath := "test"
	canvasPage := menu.Menu{PlatformID: canvasPlatform.ID, MenuType: menu.TypePage, Name: "Test", Code: role.PermissionList, I18nKey: roleTestStringPointer("navigation.test"), Path: &canvasPath, ComponentPath: &canvasComponentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No}
	if err := tx.WithContext(ctx).Create(&canvasPage).Error; err != nil {
		t.Fatal(err)
	}
	canvasAction := menu.Menu{PlatformID: canvasPlatform.ID, ParentID: &canvasPage.ID, MenuType: menu.TypeAction, Name: "Test Button", Code: role.PermissionAuthorize, IsEnabled: yesno.Yes, IsHidden: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&canvasAction).Error; err != nil {
		t.Fatal(err)
	}
	roleID, err := service.Create(ctx, role.CreateInput{Code: fmt.Sprintf("permission_%d", time.Now().UnixNano()), Name: fmt.Sprintf("Permission %d", time.Now().UnixNano())})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateStatus(ctx, roleID, yesno.No); err != nil {
		t.Fatal(err)
	}
	boundUser := createRoleAccessUser(t, tx, ctx, roleID, yesno.Yes, false)
	count, err := service.UpdatePermissions(ctx, roleID, []int64{page.ID, action.ID, canvasPage.ID, canvasAction.ID})
	if err != nil || count != 2 {
		t.Fatalf("UpdatePermissions() = %d,%v", count, err)
	}
	assertRoleAccessState(t, accessStates, boundUser.ID, 2)
	permissions, err := service.Permissions(ctx, roleID)
	if err != nil || len(permissions.Platforms) != 2 || !reflect.DeepEqual(permissions.MenuIDs, []int64{action.ID, canvasAction.ID}) {
		t.Fatalf("Permissions() = %+v,%v", permissions, err)
	}
	adminPermissions, canvasPermissions := permissions.Platforms[0], permissions.Platforms[1]
	if adminPermissions.ID != adminPlatformID || adminPermissions.Code != "admin" || len(adminPermissions.MenuTree) != 1 || adminPermissions.MenuTree[0].ID != root.ID {
		t.Fatalf("Admin permissions = %+v", adminPermissions)
	}
	if canvasPermissions.ID != canvasPlatform.ID || canvasPermissions.Code != "canvas" || canvasPermissions.IsEnabled != yesno.No || len(canvasPermissions.MenuTree) != 1 || canvasPermissions.MenuTree[0].ID != canvasPage.ID || len(canvasPermissions.MenuTree[0].Children) != 1 {
		t.Fatalf("Canvas permissions = %+v", canvasPermissions)
	}
	var before menu.RoleMenu
	if err := tx.WithContext(ctx).Where("role_id = ?", roleID).Take(&before).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdatePermissions(ctx, roleID, []int64{action.ID, canvasAction.ID}); err != nil {
		t.Fatal(err)
	}
	if got := readRoleAccessVersion(t, tx, ctx, boundUser.ID); got != 2 {
		t.Fatalf("idempotent permission access version = %d", got)
	}
	var after menu.RoleMenu
	if err := tx.WithContext(ctx).First(&after, before.ID).Error; err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("unchanged grant was rewritten: before=%+v after=%+v err=%v", before, after, err)
	}
	if count, err := service.UpdatePermissions(ctx, roleID, []int64{}); err != nil || count != 0 {
		t.Fatalf("clear permissions = %d,%v", count, err)
	}
	assertRoleAccessState(t, accessStates, boundUser.ID, 3)
}

func TestServicePermissionsRejectsSuperAdminAndInvalidMenus(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := newRoleTestService(t, role.NewRepository(tx))
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

func newRoleTestService(t *testing.T, repository *role.Repository) *role.Service {
	t.Helper()
	service, _, _ := newRoleMutationTestService(t, repository)
	return service
}

func newRoleMutationTestService(t *testing.T, repository *role.Repository) (*role.Service, *accessstate.Store, *projectredis.Client) {
	t.Helper()
	redisClient := openRoleTestRedis(t)
	accessStates := accessstate.NewStore(redisClient)
	return role.NewService(repository, accessstate.NewInvalidator(accessStates)), accessStates, redisClient
}

func openRoleTestRedis(t *testing.T) *projectredis.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("Redis integration test")
	}
	if err := godotenv.Load("../../../../.env"); err != nil && !os.IsNotExist(err) {
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
	redisURL.Path = "/13"
	redisURL.RawPath = ""
	client, err := projectredis.Open(context.Background(), redisURL.String())
	if err != nil {
		t.Fatalf("open test Redis database 13: %v", err)
	}
	if err := client.ScanDelete(context.Background(), "authz:access-state:*"); err != nil {
		_ = client.Close()
		t.Fatalf("clean test Redis database 13: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func readRoleAccessVersion(t *testing.T, tx *gorm.DB, ctx context.Context, userID int64) int64 {
	t.Helper()
	var version int64
	result := tx.WithContext(ctx).Raw("SELECT version FROM rbac_access_version WHERE user_id = ?", userID).Scan(&version)
	if result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("read access version: rows=%d error=%v", result.RowsAffected, result.Error)
	}
	return version
}

func assertRoleAccessState(t *testing.T, store *accessstate.Store, userID, version int64) {
	t.Helper()
	state, found, err := store.Read(context.Background(), userID)
	if err != nil || !found || state.State != accessstate.StateReady || state.Version != version {
		t.Fatalf("access state = %+v found=%v error=%v", state, found, err)
	}
}

func waitForRoleAccessState(t *testing.T, store *accessstate.Store, userID int64, wanted string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		state, found, err := store.Read(context.Background(), userID)
		if err == nil && found && state.State == wanted {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("access state for user %d did not become %s", userID, wanted)
}

func roleErrorCode(err error) int {
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		return 0
	}
	return appErr.Code
}

func roleTestStringPointer(value string) *string {
	return &value
}
