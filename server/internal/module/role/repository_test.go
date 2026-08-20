package role_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/auth"
	"admin/server/internal/module/menu"
	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestEnsureSystemRolesCreatesAndValidatesRoles(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	repository := role.NewRepository(tx)
	service := role.NewService(repository)

	if err := service.EnsureSystemRoles(ctx); err != nil {
		t.Fatalf("EnsureSystemRoles() error = %v", err)
	}
	if err := service.EnsureSystemRoles(ctx); err != nil {
		t.Fatalf("second EnsureSystemRoles() error = %v", err)
	}

	var roles []role.Role
	if err := tx.WithContext(ctx).Where("code IN ?", []string{role.CodeSuperAdmin, role.CodeRegisteredUser}).Order("code").Find(&roles).Error; err != nil {
		t.Fatalf("query system roles: %v", err)
	}
	if len(roles) != 2 {
		t.Fatalf("system role count = %d", len(roles))
	}
	want := map[string]struct {
		name      string
		isDefault yesno.Value
	}{
		role.CodeSuperAdmin:     {name: "超级管理员", isDefault: yesno.No},
		role.CodeRegisteredUser: {name: "普通用户", isDefault: yesno.Yes},
	}
	for _, stored := range roles {
		expected := want[stored.Code]
		if stored.Name != expected.name || stored.IsDefault != expected.isDefault || stored.IsEnabled != yesno.Yes {
			t.Errorf("role %s = %+v", stored.Code, stored)
		}
	}
}

func TestEnsureSystemRolesRejectsMutatedSystemRole(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	repository := role.NewRepository(tx)
	service := role.NewService(repository)
	if err := service.EnsureSystemRoles(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Model(&role.Role{}).Where("code = ?", role.CodeRegisteredUser).Update("name", "错误名称").Error; err != nil {
		t.Fatal(err)
	}

	if err := service.EnsureSystemRoles(ctx); err == nil {
		t.Fatal("mutated system role was silently accepted")
	}
}

func TestFindDefaultRequiresExactlyOneEnabledRole(t *testing.T) {
	t.Run("finds the configured default", func(t *testing.T) {
		tx, ctx := openRoleTransaction(t)
		repository := role.NewRepository(tx)
		if err := role.NewService(repository).EnsureSystemRoles(ctx); err != nil {
			t.Fatal(err)
		}
		found, err := repository.FindDefault(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if found.Code != role.CodeRegisteredUser {
			t.Fatalf("default role = %q", found.Code)
		}
	})

	t.Run("rejects no enabled default", func(t *testing.T) {
		tx, ctx := openRoleTransaction(t)
		repository := role.NewRepository(tx)
		if err := role.NewService(repository).EnsureSystemRoles(ctx); err != nil {
			t.Fatal(err)
		}
		if err := tx.WithContext(ctx).Model(&role.Role{}).Where("code = ?", role.CodeRegisteredUser).Update("is_enabled", yesno.No).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := repository.FindDefault(ctx); err == nil {
			t.Fatal("missing enabled default role was accepted")
		}
	})

	t.Run("rejects multiple enabled defaults", func(t *testing.T) {
		tx, ctx := openRoleTransaction(t)
		repository := role.NewRepository(tx)
		if err := role.NewService(repository).EnsureSystemRoles(ctx); err != nil {
			t.Fatal(err)
		}
		if err := tx.WithContext(ctx).Exec("DROP INDEX ux_sys_role_default_active").Error; err != nil {
			t.Fatal(err)
		}
		if err := tx.WithContext(ctx).Model(&role.Role{}).Where("code = ?", role.CodeSuperAdmin).Update("is_default", yesno.Yes).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := repository.FindDefault(ctx); err == nil {
			t.Fatal("multiple enabled default roles were accepted")
		}
	})
}

func TestFindByCodeRejectsDeletedOrDisabledRole(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		tx, ctx := openRoleTransaction(t)
		repository := role.NewRepository(tx)
		if err := role.NewService(repository).EnsureSystemRoles(ctx); err != nil {
			t.Fatal(err)
		}
		if err := tx.WithContext(ctx).Model(&role.Role{}).Where("code = ?", role.CodeSuperAdmin).Update("is_enabled", yesno.No).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := repository.FindByCode(ctx, role.CodeSuperAdmin); err == nil {
			t.Fatal("disabled role was returned")
		}
	})

	t.Run("deleted", func(t *testing.T) {
		tx, ctx := openRoleTransaction(t)
		repository := role.NewRepository(tx)
		if err := role.NewService(repository).EnsureSystemRoles(ctx); err != nil {
			t.Fatal(err)
		}
		if err := tx.WithContext(ctx).Where("code = ?", role.CodeSuperAdmin).Delete(&role.Role{}).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := repository.FindByCode(ctx, role.CodeSuperAdmin); err == nil {
			t.Fatal("deleted role was returned")
		}
	})
}

func TestHasActiveUserWithRole(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	repository := role.NewRepository(tx)
	if err := role.NewService(repository).EnsureSystemRoles(ctx); err != nil {
		t.Fatal(err)
	}
	testRole := role.Role{
		Code: fmt.Sprintf("role_attachment_%d", time.Now().UnixNano()), Name: "Role Attachment Test",
		IsDefault: yesno.No, IsEnabled: yesno.Yes,
	}
	if err := tx.WithContext(ctx).Create(&testRole).Error; err != nil {
		t.Fatal(err)
	}
	createdUser := user.User{
		Username:     fmt.Sprintf("role-test-%d", time.Now().UnixNano()),
		Email:        fmt.Sprintf("role-test-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "not-a-real-hash",
		IsEnabled:    yesno.Yes,
	}
	if err := tx.WithContext(ctx).Create(&createdUser).Error; err != nil {
		t.Fatal(err)
	}
	relation := role.UserRole{UserID: createdUser.ID, RoleID: testRole.ID}
	if err := tx.WithContext(ctx).Create(&relation).Error; err != nil {
		t.Fatal(err)
	}

	found, err := repository.HasActiveUserWithRole(ctx, testRole.ID)
	if err != nil || !found {
		t.Fatalf("HasActiveUserWithRole() = %v,%v", found, err)
	}
	if err := tx.WithContext(ctx).Delete(&relation).Error; err != nil {
		t.Fatal(err)
	}
	found, err = repository.HasActiveUserWithRole(ctx, testRole.ID)
	if err != nil || found {
		t.Fatalf("after relation deletion = %v,%v", found, err)
	}
}

func TestRepositoryListUsesStableFiltersAndExactCounts(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	repository := role.NewRepository(tx)
	unique := fmt.Sprintf("role_list_%d", time.Now().UnixNano())
	createdAt := time.Now().UTC().Add(-time.Hour)
	first := role.Role{Code: unique + "_a", Name: unique + " First", IsDefault: yesno.No, IsEnabled: yesno.Yes, CreatedAt: createdAt}
	second := role.Role{Code: unique + "_b", Name: unique + " Second", IsDefault: yesno.No, IsEnabled: yesno.No, CreatedAt: createdAt}
	if err := tx.WithContext(ctx).Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Model(&role.Role{}).Where("id = ?", second.ID).Update("is_enabled", yesno.No).Error; err != nil {
		t.Fatal(err)
	}
	second.IsEnabled = yesno.No
	disabledUser := user.User{
		Username: unique, Email: unique + "@example.com", PasswordHash: "not-a-real-hash", IsEnabled: yesno.No,
	}
	if err := tx.WithContext(ctx).Create(&disabledUser).Error; err != nil {
		t.Fatal(err)
	}
	relation := role.UserRole{UserID: disabledUser.ID, RoleID: first.ID}
	if err := tx.WithContext(ctx).Create(&relation).Error; err != nil {
		t.Fatal(err)
	}
	path := "/" + unique
	view := "system-menus"
	page := menu.Menu{MenuType: menu.TypePage, Code: unique + ":list", I18nKey: "navigation.systemMenus", Path: &path, ViewKey: &view, IsEnabled: yesno.No}
	if err := tx.WithContext(ctx).Create(&page).Error; err != nil {
		t.Fatal(err)
	}
	grant := menu.RoleMenu{RoleID: first.ID, MenuID: page.ID}
	if err := tx.WithContext(ctx).Create(&grant).Error; err != nil {
		t.Fatal(err)
	}

	query := role.ListQuery{Page: 1, PageSize: 10, Keyword: unique}
	total, err := repository.Count(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	items, err := repository.List(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(items) != 2 || items[0].ID != first.ID || items[1].ID != second.ID {
		t.Fatalf("list total/items = %d/%+v", total, items)
	}
	if items[0].UserCount != 1 || items[0].PermissionCount != 1 {
		t.Fatalf("first counts = users %d permissions %d", items[0].UserCount, items[0].PermissionCount)
	}

	if err := tx.WithContext(ctx).Delete(&relation).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Delete(&grant).Error; err != nil {
		t.Fatal(err)
	}
	items, err = repository.List(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].UserCount != 0 || items[0].PermissionCount != 0 {
		t.Fatalf("deleted relation/grant still counted: %+v", items[0])
	}

	enabled := yesno.Yes
	total, err = repository.Count(ctx, role.ListQuery{Page: 1, PageSize: 10, Keyword: unique, IsEnabled: &enabled})
	if err != nil || total != 1 {
		t.Fatalf("enabled count = %d,%v", total, err)
	}
	empty, err := repository.List(ctx, role.ListQuery{Page: 99, PageSize: 10, Keyword: unique})
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty page = %#v,%v", empty, err)
	}
}

func TestRepositoryListTreatsPercentAndUnderscoreLiterally(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	repository := role.NewRepository(tx)
	unique := fmt.Sprintf("literal_%d", time.Now().UnixNano())
	values := []role.Role{
		{Code: unique + "_percent", Name: "Value 100% " + unique, IsEnabled: yesno.Yes},
		{Code: unique + "_underscore", Name: "Value_under " + unique, IsEnabled: yesno.Yes},
		{Code: unique + "_plain", Name: "Value plain " + unique, IsEnabled: yesno.Yes},
	}
	for index := range values {
		if err := tx.WithContext(ctx).Create(&values[index]).Error; err != nil {
			t.Fatal(err)
		}
	}
	for keyword, wantID := range map[string]int64{"100%": values[0].ID, "Value_under": values[1].ID} {
		items, err := repository.List(ctx, role.ListQuery{Page: 1, PageSize: 10, Keyword: keyword})
		if err != nil || len(items) != 1 || items[0].ID != wantID {
			t.Fatalf("literal keyword %q = %+v,%v", keyword, items, err)
		}
	}
}

func TestEnsureSystemRolesRejectsPartialSystemRoleStateAtomically(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := role.NewService(role.NewRepository(tx))
	if err := service.EnsureSystemRoles(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Where("code = ?", role.CodeRegisteredUser).Delete(&role.Role{}).Error; err != nil {
		t.Fatal(err)
	}

	if err := service.EnsureSystemRoles(ctx); err == nil {
		t.Fatal("partial system role state was accepted")
	}
	var activeRegisteredCount int64
	if err := tx.WithContext(ctx).Model(&role.Role{}).Where("code = ?", role.CodeRegisteredUser).Count(&activeRegisteredCount).Error; err != nil {
		t.Fatal(err)
	}
	if activeRegisteredCount != 0 {
		t.Fatalf("deleted registered_user was silently restored: count=%d", activeRegisteredCount)
	}
}

func TestEnsureSystemRolesPreservesCustomDefault(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	service := role.NewService(role.NewRepository(tx))
	if err := service.EnsureSystemRoles(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Model(&role.Role{}).Where("code = ?", role.CodeRegisteredUser).Update("is_default", yesno.No).Error; err != nil {
		t.Fatal(err)
	}
	custom := role.Role{Code: "custom_default", Name: "Custom Default", IsDefault: yesno.Yes, IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&custom).Error; err != nil {
		t.Fatal(err)
	}

	if err := service.EnsureSystemRoles(ctx); err != nil {
		t.Fatalf("custom default was rejected: %v", err)
	}
	found, err := role.NewRepository(tx).FindDefault(ctx)
	if err != nil || found.ID != custom.ID {
		t.Fatalf("default after restart = %+v,%v", found, err)
	}
}

func openRoleTransaction(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration test")
	}
	if err := godotenv.Load("../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("load server .env: %v", err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatalf("load worker config: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	connection, err := database.Open(ctx, settings.PostgresDSN)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	if err := database.AutoMigrate(ctx, connection.GORM, &user.User{}, &role.Role{}, &role.UserRole{}, &menu.Menu{}, &menu.RoleMenu{}, &auth.Session{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := role.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("Ensure role schema: %v", err)
	}
	if err := auth.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	if err := menu.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("Ensure menu schema: %v", err)
	}
	tx := connection.GORM.WithContext(ctx).Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	return tx, ctx
}
