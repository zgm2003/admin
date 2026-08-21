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
	"admin/server/internal/module/access"
	"admin/server/internal/module/menu"
	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestFindSourceWithVersionLoadsOneAuthoritativeRBACFact(t *testing.T) {
	fixture := openAccessRepositoryFixture(t)
	source, err := fixture.repository.FindSourceWithVersion(fixture.ctx, fixture.user.ID)
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
	source, err := fixture.repository.FindSourceWithVersion(fixture.ctx, fixture.user.ID)
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
	source, err := fixture.repository.FindSourceWithVersion(fixture.ctx, fixture.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !source.SuperAdmin || !reflect.DeepEqual(source.RoleCodes, []string{role.CodeSuperAdmin}) || len(source.GrantedMenuIDs) != 0 {
		t.Fatalf("super-admin source = %+v", source)
	}
}

func TestFindSourceWithVersionRejectsUnavailableUserOrMissingVersion(t *testing.T) {
	for _, mutate := range []func(*accessRepositoryFixture) error{
		func(fixture *accessRepositoryFixture) error {
			return fixture.db.WithContext(fixture.ctx).Model(&user.User{}).Where("id = ?", fixture.user.ID).Update("is_enabled", yesno.No).Error
		},
		func(fixture *accessRepositoryFixture) error {
			return fixture.db.WithContext(fixture.ctx).Delete(&access.Version{}, fixture.user.ID).Error
		},
	} {
		fixture := openAccessRepositoryFixture(t)
		if err := mutate(fixture); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.repository.FindSourceWithVersion(fixture.ctx, fixture.user.ID); err == nil {
			t.Fatal("unavailable access source was accepted")
		}
	}
}

type accessRepositoryFixture struct {
	db           *gorm.DB
	ctx          context.Context
	repository   *access.Repository
	user         user.User
	assignedRole role.Role
	userRole     role.UserRole
	action       menu.Menu
}

func openAccessRepositoryFixture(t *testing.T) *accessRepositoryFixture {
	t.Helper()
	connection, ctx := openIsolatedAccessDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM,
		&user.User{}, &role.Role{}, &role.UserRole{}, &menu.Menu{}, &menu.RoleMenu{}, &access.Version{}); err != nil {
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
	assignedRole := role.Role{Code: fmt.Sprintf("access_role_%d", now.UnixNano()), Name: "Access Role", IsDefault: yesno.No, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	if err := connection.GORM.WithContext(ctx).Create(&assignedRole).Error; err != nil {
		t.Fatal(err)
	}
	createdUser := user.User{Username: fmt.Sprintf("access_user_%d", now.UnixNano()), Email: fmt.Sprintf("access_%d@example.com", now.UnixNano()), PasswordHash: "hash", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
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
	root := menu.Menu{MenuType: menu.TypeDirectory, Code: "system", I18nKey: "navigation.system", SortOrder: 10, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	if err := connection.GORM.WithContext(ctx).Create(&root).Error; err != nil {
		t.Fatal(err)
	}
	path, viewKey := "/system/users", "system-users"
	page := menu.Menu{ParentID: &root.ID, MenuType: menu.TypePage, Code: "system:user:list", I18nKey: "navigation.systemUsers", Path: &path, ViewKey: &viewKey, SortOrder: 10, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	if err := connection.GORM.WithContext(ctx).Create(&page).Error; err != nil {
		t.Fatal(err)
	}
	action := menu.Menu{ParentID: &page.ID, MenuType: menu.TypeAction, Code: "system:user:create", I18nKey: "permission.userCreate", SortOrder: 10, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	if err := connection.GORM.WithContext(ctx).Create(&action).Error; err != nil {
		t.Fatal(err)
	}
	if err := connection.GORM.WithContext(ctx).Create(&menu.RoleMenu{RoleID: assignedRole.ID, MenuID: action.ID, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	return &accessRepositoryFixture{db: connection.GORM, ctx: ctx, repository: access.NewRepository(connection.GORM), user: createdUser, assignedRole: assignedRole, userRole: userRole, action: action}
}

func openIsolatedAccessDatabase(t *testing.T) (*database.Connection, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration test")
	}
	if err := godotenv.Load("../../../.env"); err != nil && !os.IsNotExist(err) {
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
