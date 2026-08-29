package access_test

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/auth/platform"
	"admin/server/internal/module/rbac/access"
	"admin/server/internal/module/rbac/menu"
	"admin/server/internal/module/rbac/role"
	"admin/server/internal/module/user/account"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestFindSourceWithVersionLoadsOneAuthoritativeRBACFact(t *testing.T) {
	fixture := openAccessRepositoryFixture(t)
	source, err := fixture.repository.FindSourceWithVersion(fixture.ctx, fixture.user.ID, fixture.adminPlatformID)
	if err != nil {
		t.Fatal(err)
	}
	if source.Version != 3 || !reflect.DeepEqual(source.RoleCodes, []string{fixture.assignedRole.Code}) || source.SuperAdmin {
		t.Fatalf("source identity = %+v", source)
	}
	if !reflect.DeepEqual(source.GrantedMenuIDs, []int64{fixture.action.ID}) {
		t.Fatalf("granted menu IDs = %v", source.GrantedMenuIDs)
	}
	if len(source.Menus) != 3 || source.Menus[0].MenuType != access.MenuDirectory {
		t.Fatalf("source menus = %+v", source.Menus)
	}
}

func TestFindSourceWithVersionAllowsEnabledUserWithoutRoles(t *testing.T) {
	fixture := openAccessRepositoryFixture(t)
	if err := fixture.db.WithContext(fixture.ctx).Delete(&fixture.userRole).Error; err != nil {
		t.Fatal(err)
	}
	source, err := fixture.repository.FindSourceWithVersion(fixture.ctx, fixture.user.ID, fixture.adminPlatformID)
	if err != nil {
		t.Fatal(err)
	}
	if source.RoleCodes == nil || source.GrantedMenuIDs == nil || len(source.RoleCodes) != 0 || len(source.GrantedMenuIDs) != 0 || source.SuperAdmin {
		t.Fatalf("roleless source = %+v", source)
	}
}

func TestFindSourceWithVersionRecognizesSuperAdminWithoutDirectGrants(t *testing.T) {
	fixture := openAccessRepositoryFixture(t)
	if err := fixture.db.WithContext(fixture.ctx).Delete(&fixture.userRole).Error; err != nil {
		t.Fatal(err)
	}
	superRole := role.Role{Code: role.CodeSuperAdmin, Name: "Super Administrator", IsDefault: yesno.No, IsEnabled: yesno.Yes}
	if err := fixture.db.WithContext(fixture.ctx).Create(&superRole).Error; err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.WithContext(fixture.ctx).Create(&role.UserRole{UserID: fixture.user.ID, RoleID: superRole.ID}).Error; err != nil {
		t.Fatal(err)
	}
	source, err := fixture.repository.FindSourceWithVersion(fixture.ctx, fixture.user.ID, fixture.adminPlatformID)
	if err != nil {
		t.Fatal(err)
	}
	if !source.SuperAdmin || !reflect.DeepEqual(source.RoleCodes, []string{role.CodeSuperAdmin}) || len(source.GrantedMenuIDs) != 0 {
		t.Fatalf("super-admin source = %+v", source)
	}
	if len(source.Menus) != 3 {
		t.Fatalf("super-admin Admin menus = %+v", source.Menus)
	}
	canvasSource, err := fixture.repository.FindSourceWithVersion(fixture.ctx, fixture.user.ID, fixture.canvasPlatformID)
	if err != nil {
		t.Fatal(err)
	}
	if !canvasSource.SuperAdmin || len(canvasSource.Menus) != 2 || canvasSource.Menus[0].ID != fixture.canvasPage.ID || len(canvasSource.GrantedMenuIDs) != 0 {
		t.Fatalf("super-admin Canvas source = %+v", canvasSource)
	}
}

func TestFindSourceWithVersionRejectsUnavailableUserOrMissingVersion(t *testing.T) {
	for _, mutate := range []func(*accessRepositoryFixture) error{
		func(fixture *accessRepositoryFixture) error {
			return fixture.db.WithContext(fixture.ctx).Model(&account.User{}).Where("id = ?", fixture.user.ID).Update("is_enabled", yesno.No).Error
		},
		func(fixture *accessRepositoryFixture) error {
			return fixture.db.WithContext(fixture.ctx).Delete(&access.Version{}, fixture.user.ID).Error
		},
	} {
		fixture := openAccessRepositoryFixture(t)
		if err := mutate(fixture); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.repository.FindSourceWithVersion(fixture.ctx, fixture.user.ID, fixture.adminPlatformID); err == nil {
			t.Fatal("unavailable access source was accepted")
		}
	}
}

func TestRepositoryFiltersMenusAndDeduplicatesGrantsByPlatform(t *testing.T) {
	fixture := openAccessRepositoryFixture(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	canvasRole := role.Role{Code: fmt.Sprintf("canvas_role_%d", now.UnixNano()), Name: "Canvas Role", IsDefault: yesno.No, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	if err := fixture.db.WithContext(fixture.ctx).Create(&canvasRole).Error; err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.WithContext(fixture.ctx).Create(&role.UserRole{UserID: fixture.user.ID, RoleID: canvasRole.ID, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	for _, roleID := range []int64{fixture.assignedRole.ID, canvasRole.ID} {
		if err := fixture.db.WithContext(fixture.ctx).Create(&menu.RoleMenu{RoleID: roleID, MenuID: fixture.canvasAction.ID, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
	}

	adminSource, err := fixture.repository.FindSourceWithVersion(fixture.ctx, fixture.user.ID, fixture.adminPlatformID)
	if err != nil {
		t.Fatal(err)
	}
	if len(adminSource.Menus) != 3 || !reflect.DeepEqual(adminSource.GrantedMenuIDs, []int64{fixture.action.ID}) {
		t.Fatalf("Admin source leaked another platform: %+v", adminSource)
	}
	canvasSource, err := fixture.repository.FindSourceWithVersion(fixture.ctx, fixture.user.ID, fixture.canvasPlatformID)
	if err != nil {
		t.Fatal(err)
	}
	if len(canvasSource.Menus) != 2 || canvasSource.Menus[0].ID != fixture.canvasPage.ID ||
		!reflect.DeepEqual(canvasSource.GrantedMenuIDs, []int64{fixture.canvasAction.ID}) {
		t.Fatalf("Canvas source = %+v", canvasSource)
	}
	if !reflect.DeepEqual(canvasSource.RoleCodes, []string{fixture.assignedRole.Code, canvasRole.Code}) {
		t.Fatalf("Canvas role codes = %v", canvasSource.RoleCodes)
	}
}

type accessRepositoryFixture struct {
	db               *gorm.DB
	ctx              context.Context
	repository       *access.Repository
	user             account.User
	assignedRole     role.Role
	userRole         role.UserRole
	action           menu.Menu
	adminPlatformID  int64
	canvasPlatformID int64
	canvasPage       menu.Menu
	canvasAction     menu.Menu
}

func openAccessRepositoryFixture(t *testing.T) *accessRepositoryFixture {
	t.Helper()
	connection, ctx := openIsolatedAccessDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &authplatform.Platform{}); err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	var adminPlatform authplatform.Platform
	if err := connection.GORM.WithContext(ctx).Where("code = ?", authplatform.BuiltinAdminCode).Take(&adminPlatform).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(ctx, connection.GORM,
		&account.User{}, &role.Role{}, &role.UserRole{}, &menu.Menu{}, &menu.RoleMenu{}, &access.Version{}); err != nil {
		t.Fatal(err)
	}
	if err := role.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	if err := menu.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	if err := access.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	canvasPlatform := adminPlatform
	canvasPlatform.ID = 0
	canvasPlatform.Code = "canvas"
	canvasPlatform.Name = "Canvas"
	canvasPlatform.IsBuiltin = yesno.No
	canvasPlatform.CreatedAt = now
	canvasPlatform.UpdatedAt = now
	canvasPlatform.DeletedAt = gorm.DeletedAt{}
	if err := connection.GORM.WithContext(ctx).Create(&canvasPlatform).Error; err != nil {
		t.Fatal(err)
	}
	assignedRole := role.Role{Code: fmt.Sprintf("access_role_%d", now.UnixNano()), Name: "Access Role", IsDefault: yesno.No, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	if err := connection.GORM.WithContext(ctx).Create(&assignedRole).Error; err != nil {
		t.Fatal(err)
	}
	createdUser := account.User{Username: fmt.Sprintf("access_user_%d", now.UnixNano()), Email: fmt.Sprintf("access_%d@example.com", now.UnixNano()), PasswordHash: "hash", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	if err := connection.GORM.WithContext(ctx).Create(&createdUser).Error; err != nil {
		t.Fatal(err)
	}
	if err := connection.GORM.WithContext(ctx).Create(&access.Version{UserID: createdUser.ID, Version: 3, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	userRole := role.UserRole{UserID: createdUser.ID, RoleID: assignedRole.ID, CreatedAt: now, UpdatedAt: now}
	if err := connection.GORM.WithContext(ctx).Create(&userRole).Error; err != nil {
		t.Fatal(err)
	}
	accountI18nKey := "navigation.account"
	root := menu.Menu{PlatformID: adminPlatform.ID, MenuType: menu.TypeDirectory, Name: "用户与账号", Code: "account", I18nKey: &accountI18nKey, SortOrder: 10, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	if err := connection.GORM.WithContext(ctx).Create(&root).Error; err != nil {
		t.Fatal(err)
	}
	path, componentPath := "/account/users", "account/users"
	pageI18nKey := "navigation.accountUsers"
	page := menu.Menu{PlatformID: adminPlatform.ID, ParentID: &root.ID, MenuType: menu.TypePage, Name: "用户管理", Code: "account:user:list", I18nKey: &pageI18nKey, Path: &path, ComponentPath: &componentPath, SortOrder: 10, IsEnabled: yesno.Yes, IsHidden: yesno.No, CreatedAt: now, UpdatedAt: now}
	if err := connection.GORM.WithContext(ctx).Create(&page).Error; err != nil {
		t.Fatal(err)
	}
	action := menu.Menu{PlatformID: adminPlatform.ID, ParentID: &page.ID, MenuType: menu.TypeAction, Name: "新增用户", Code: "account:user:create", SortOrder: 10, IsEnabled: yesno.Yes, IsHidden: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	if err := connection.GORM.WithContext(ctx).Create(&action).Error; err != nil {
		t.Fatal(err)
	}
	if err := connection.GORM.WithContext(ctx).Create(&menu.RoleMenu{RoleID: assignedRole.ID, MenuID: action.ID, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	canvasPath, canvasComponentPath := "/test", "test"
	canvasI18nKey := "navigation.system"
	canvasPage := menu.Menu{PlatformID: canvasPlatform.ID, MenuType: menu.TypePage, Name: "Canvas Test", Code: "canvas:test", I18nKey: &canvasI18nKey, Path: &canvasPath, ComponentPath: &canvasComponentPath, IsEnabled: yesno.Yes, IsHidden: yesno.No, CreatedAt: now, UpdatedAt: now}
	if err := connection.GORM.WithContext(ctx).Create(&canvasPage).Error; err != nil {
		t.Fatal(err)
	}
	canvasAction := menu.Menu{PlatformID: canvasPlatform.ID, ParentID: &canvasPage.ID, MenuType: menu.TypeAction, Name: "Canvas Test Button", Code: "canvas:test:button", IsEnabled: yesno.Yes, IsHidden: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	if err := connection.GORM.WithContext(ctx).Create(&canvasAction).Error; err != nil {
		t.Fatal(err)
	}
	return &accessRepositoryFixture{
		db: connection.GORM, ctx: ctx, repository: access.NewRepository(connection.GORM), user: createdUser,
		assignedRole: assignedRole, userRole: userRole, action: action, adminPlatformID: adminPlatform.ID,
		canvasPlatformID: canvasPlatform.ID, canvasPage: canvasPage, canvasAction: canvasAction,
	}
}

func openIsolatedAccessDatabase(t *testing.T) (*database.Connection, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration test")
	}
	if err := godotenv.Load("../../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	root, err := database.Open(ctx, settings.PostgresDSN)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("test_access_%d", time.Now().UnixNano())
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
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	connection := &database.Connection{GORM: gormDB, SQL: sqlDB}
	t.Cleanup(func() {
		_ = connection.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = root.GORM.WithContext(cleanupCtx).Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error
		_ = root.Close()
	})
	return connection, ctx
}
