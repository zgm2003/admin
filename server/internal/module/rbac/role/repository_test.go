package role_test

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/auth/login"
	"admin/server/internal/module/auth/platform"
	"admin/server/internal/module/rbac/access"
	"admin/server/internal/module/rbac/menu"
	"admin/server/internal/module/rbac/role"
	"admin/server/internal/module/rbac/state"
	"admin/server/internal/module/user/account"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestEnsureSystemRolesCreatesAndValidatesRoles(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	repository := role.NewRepository(tx)
	service := role.NewService(repository, nil)

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

func TestRepositoryFindPermissionPlatformsIncludesDisabledAndExcludesDeleted(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	canvas := createRoleTestPlatform(t, tx, ctx, "canvas", "Canvas", yesno.No)
	deleted := createRoleTestPlatform(t, tx, ctx, "deleted", "Deleted", yesno.Yes)
	if err := tx.WithContext(ctx).Delete(&deleted).Error; err != nil {
		t.Fatal(err)
	}

	platforms, err := role.NewRepository(tx).FindPermissionPlatforms(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(platforms) != 2 || platforms[0].Code != authplatform.BuiltinAdminCode || platforms[1].ID != canvas.ID || platforms[1].IsEnabled != yesno.No {
		t.Fatalf("permission platforms = %+v", platforms)
	}
}

func TestEnsureSystemRolesRejectsMutatedSystemRole(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	repository := role.NewRepository(tx)
	service := role.NewService(repository, nil)
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
		if err := role.NewService(repository, nil).EnsureSystemRoles(ctx); err != nil {
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
		if err := role.NewService(repository, nil).EnsureSystemRoles(ctx); err != nil {
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
		if err := role.NewService(repository, nil).EnsureSystemRoles(ctx); err != nil {
			t.Fatal(err)
		}
		if err := tx.WithContext(ctx).Exec("DROP INDEX ux_rbac_role_default_active").Error; err != nil {
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
		if err := role.NewService(repository, nil).EnsureSystemRoles(ctx); err != nil {
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
		if err := role.NewService(repository, nil).EnsureSystemRoles(ctx); err != nil {
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
	if err := role.NewService(repository, nil).EnsureSystemRoles(ctx); err != nil {
		t.Fatal(err)
	}
	testRole := role.Role{
		Code: fmt.Sprintf("role_attachment_%d", time.Now().UnixNano()), Name: "Role Attachment Test",
		IsDefault: yesno.No, IsEnabled: yesno.Yes,
	}
	if err := tx.WithContext(ctx).Create(&testRole).Error; err != nil {
		t.Fatal(err)
	}
	createdUser := account.User{
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

func TestRepositoryEffectiveAccessVersionsByRoleAreSortedAndExact(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	repository := role.NewRepository(tx)
	testRole := role.Role{Code: fmt.Sprintf("access_scope_%d", time.Now().UnixNano()), Name: "Access scope", IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&testRole).Error; err != nil {
		t.Fatal(err)
	}
	first := createRoleAccessUser(t, tx, ctx, testRole.ID, yesno.Yes, false)
	second := createRoleAccessUser(t, tx, ctx, testRole.ID, yesno.Yes, false)
	_ = createRoleAccessUser(t, tx, ctx, testRole.ID, yesno.No, false)
	_ = createRoleAccessUser(t, tx, ctx, testRole.ID, yesno.Yes, true)

	want := []accessstate.Version{{UserID: first.ID, Version: 1}, {UserID: second.ID, Version: 1}}
	candidates, err := repository.FindEffectiveAccessVersionsByRole(ctx, testRole.ID)
	if err != nil || !reflect.DeepEqual(candidates, want) {
		t.Fatalf("FindEffectiveAccessVersionsByRole() = %+v,%v", candidates, err)
	}
	locked, err := repository.LockEffectiveAccessVersionsByRole(ctx, testRole.ID)
	if err != nil || !reflect.DeepEqual(locked, want) {
		t.Fatalf("LockEffectiveAccessVersionsByRole() = %+v,%v", locked, err)
	}
	advanced, err := repository.IncrementAccessVersions(ctx, []int64{second.ID, first.ID, first.ID}, time.Now().UTC().Truncate(time.Microsecond))
	if err != nil || !reflect.DeepEqual(advanced, map[int64]int64{first.ID: 2, second.ID: 2}) {
		t.Fatalf("IncrementAccessVersions() = %+v,%v", advanced, err)
	}
}

func TestRepositoryEffectiveAccessVersionsRejectMissingVersion(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	testRole := role.Role{Code: fmt.Sprintf("missing_version_%d", time.Now().UnixNano()), Name: "Missing version", IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&testRole).Error; err != nil {
		t.Fatal(err)
	}
	created := account.User{
		Username: fmt.Sprintf("missing_version_%d", time.Now().UnixNano()),
		Email:    fmt.Sprintf("missing_version_%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", IsEnabled: yesno.Yes,
	}
	if err := tx.WithContext(ctx).Create(&created).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Create(&role.UserRole{UserID: created.ID, RoleID: testRole.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := role.NewRepository(tx).FindEffectiveAccessVersionsByRole(ctx, testRole.ID); err == nil {
		t.Fatal("missing access version was accepted")
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
	disabledUser := account.User{
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
	componentPath := "access/menus"
	page := menu.Menu{PlatformID: roleTestAdminPlatformID(t, tx, ctx), MenuType: menu.TypePage, Name: "List", Code: unique + ":list", I18nKey: roleTestStringPointer("navigation.systemMenus"), Path: &path, ComponentPath: &componentPath, IsEnabled: yesno.No, IsHidden: yesno.No}
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
	service := role.NewService(role.NewRepository(tx), nil)
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
	service := role.NewService(role.NewRepository(tx), nil)
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
	db, ctx := openRoleDatabase(t)
	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	return tx, ctx
}

func openRoleDatabase(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration test")
	}
	if err := godotenv.Load("../../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("load server .env: %v", err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatalf("load worker config: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	root, err := database.Open(ctx, settings.PostgresDSN)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	schema := fmt.Sprintf("test_role_%d", time.Now().UnixNano())
	if err := root.GORM.WithContext(ctx).Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	pgxConfig, err := pgx.ParseConfig(settings.PostgresDSN)
	if err != nil {
		t.Fatal(err)
	}
	pgxConfig.RuntimeParams["search_path"] = schema
	sqlDB := stdlib.OpenDB(*pgxConfig)
	if err := sqlDB.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = root.GORM.WithContext(cleanupCtx).Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error
		_ = root.Close()
	})
	if err := database.AutoMigrate(ctx, db, &authplatform.Platform{}); err != nil {
		t.Fatalf("AutoMigrate authentication platforms: %v", err)
	}
	if err := authplatform.EnsureSchema(ctx, db); err != nil {
		t.Fatalf("Ensure authentication platform schema: %v", err)
	}
	if err := database.AutoMigrate(ctx, db, &account.User{}, &role.Role{}, &role.UserRole{}, &menu.Menu{}, &menu.RoleMenu{}, &auth.Session{}, &access.Version{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := role.EnsureSchema(ctx, db); err != nil {
		t.Fatalf("Ensure role schema: %v", err)
	}
	if err := menu.EnsureSchema(ctx, db); err != nil {
		t.Fatalf("Ensure menu schema: %v", err)
	}
	if err := access.EnsureSchema(ctx, db); err != nil {
		t.Fatalf("Ensure access schema: %v", err)
	}
	return db, ctx
}

func roleTestAdminPlatformID(t *testing.T, db *gorm.DB, ctx context.Context) int64 {
	t.Helper()
	var platform authplatform.Platform
	if err := db.WithContext(ctx).Where("code = ?", authplatform.BuiltinAdminCode).Take(&platform).Error; err != nil {
		t.Fatalf("find test Admin platform: %v", err)
	}
	return platform.ID
}

func createRoleTestPlatform(t *testing.T, db *gorm.DB, ctx context.Context, code, name string, isEnabled yesno.Value) authplatform.Platform {
	t.Helper()
	platform := authplatform.Platform{
		Code: code, Name: name, PolicyVersion: 1,
		AccessTTLSeconds: 900, RefreshTTLSeconds: 1_209_600,
		SessionCacheTTLSeconds: 1_800, AccessCacheTTLSeconds: 1_800,
		BindDevice: yesno.No, BindIP: yesno.No, MaxSessions: 1,
		AllowRegister: yesno.Yes, IsEnabled: isEnabled, IsBuiltin: yesno.No,
	}
	if err := db.WithContext(ctx).Create(&platform).Error; err != nil {
		t.Fatalf("create test authentication platform: %v", err)
	}
	return platform
}

func createRoleAccessUser(t *testing.T, tx *gorm.DB, ctx context.Context, roleID int64, enabled yesno.Value, deleted bool) account.User {
	t.Helper()
	unique := time.Now().UnixNano()
	created := account.User{
		Username: fmt.Sprintf("role_access_%d", unique), Email: fmt.Sprintf("role_access_%d@example.com", unique),
		PasswordHash: "hash", IsEnabled: enabled,
	}
	if err := tx.WithContext(ctx).Create(&created).Error; err != nil {
		t.Fatal(err)
	}
	if enabled == yesno.No {
		if err := tx.WithContext(ctx).Model(&account.User{}).Where("id = ?", created.ID).Update("is_enabled", yesno.No).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.WithContext(ctx).Create(&access.Version{UserID: created.ID, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Create(&role.UserRole{UserID: created.ID, RoleID: roleID}).Error; err != nil {
		t.Fatal(err)
	}
	if deleted {
		if err := tx.WithContext(ctx).Delete(&created).Error; err != nil {
			t.Fatal(err)
		}
	}
	return created
}
