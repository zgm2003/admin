package database_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/database/testschema"
	authplatform "admin/server/internal/module/auth/platform"
	"admin/server/internal/module/permission/access"
	"admin/server/internal/module/permission/menu"
	"admin/server/internal/module/permission/role"
	"admin/server/internal/module/user/account"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestAdminRBACBaselineMigration(t *testing.T) {
	db, ctx := openRBACMigrationSchema(t)
	fixture := createRBACMigrationFixture(t, db, ctx)
	script := readRBACBaselineMigration(t)
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("execute RBAC baseline migration: %v", err)
	}
	assertRBACMigrationState(t, db, ctx, fixture)

	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("execute RBAC baseline migration twice: %v", err)
	}
	assertRBACMigrationState(t, db, ctx, fixture)
}

func TestAdminRBACBaselineMigrationRejectsCanvasConflict(t *testing.T) {
	db, ctx := openRBACMigrationSchema(t)
	fixture := createRBACMigrationFixture(t, db, ctx)
	if err := db.WithContext(ctx).Create(&menu.Menu{PlatformID: fixture.canvasID, MenuType: menu.TypePage, Name: "Existing", Code: "canvas:test:list", I18nKey: rbacStringPointer("navigation.test"), Path: rbacStringPointer("/other"), ComponentPath: rbacStringPointer("other"), IsEnabled: yesno.Yes, IsHidden: yesno.No}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(readRBACBaselineMigration(t)).Error; err == nil || !strings.Contains(strings.ToLower(err.Error()), "canvas") {
		t.Fatalf("expected Canvas conflict, got %v", err)
	}
	var old menu.Menu
	if err := db.WithContext(ctx).Where("code = ?", "canvas:test").Take(&old).Error; err != nil {
		t.Fatalf("old Canvas row disappeared after rollback: %v", err)
	}
}

type rbacMigrationFixture struct {
	adminID  int64
	canvasID int64
	rootID   int64
	pageID   int64
	roleID   int64
	userID   int64
}

func openRBACMigrationSchema(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	if err := godotenv.Load(filepath.Join(rbacRepoRoot(t), "server", ".env")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_admin_permission_baseline")
	if err := database.AutoMigrate(ctx, db, &account.User{}, &authplatform.Platform{}, &role.Role{}, &role.UserRole{}, &menu.Menu{}, &menu.RoleMenu{}, &permission.Version{}); err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := role.EnsureSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := menu.EnsureSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := permission.EnsureSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func createRBACMigrationFixture(t *testing.T, db *gorm.DB, ctx context.Context) rbacMigrationFixture {
	t.Helper()
	var admin authplatform.Platform
	canvas := authplatform.Platform{Code: "canvas", Name: "Canvas", PolicyVersion: 1, AccessTTLSeconds: 900, RefreshTTLSeconds: 900, SessionCacheTTLSeconds: 900, AccessCacheTTLSeconds: 900, BindDevice: yesno.No, BindIP: yesno.No, MaxSessions: 10, AllowRegister: yesno.Yes, IsEnabled: yesno.Yes, IsBuiltin: yesno.No}
	if err := db.WithContext(ctx).Where("code = ?", "admin").Take(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Create(&canvas).Error; err != nil {
		t.Fatal(err)
	}
	root := menu.Menu{PlatformID: admin.ID, MenuType: menu.TypeDirectory, Name: "Account", Code: "account", I18nKey: rbacStringPointer("navigation.account"), IsEnabled: yesno.Yes, IsHidden: yesno.No}
	if err := db.WithContext(ctx).Create(&root).Error; err != nil {
		t.Fatal(err)
	}
	path, component := "/test", "test"
	canvasPage := menu.Menu{PlatformID: canvas.ID, MenuType: menu.TypePage, Name: "Canvas Test", Code: "canvas:test", I18nKey: rbacStringPointer("navigation.test"), Path: &path, ComponentPath: &component, IsEnabled: yesno.Yes, IsHidden: yesno.No}
	if err := db.WithContext(ctx).Create(&canvasPage).Error; err != nil {
		t.Fatal(err)
	}
	canvasAction := menu.Menu{PlatformID: canvas.ID, ParentID: &canvasPage.ID, MenuType: menu.TypeAction, Name: "Canvas Button", Code: "canvas:test:button", IsEnabled: yesno.Yes, IsHidden: yesno.Yes}
	if err := db.WithContext(ctx).Create(&canvasAction).Error; err != nil {
		t.Fatal(err)
	}
	user := account.User{Username: "migration_user", Email: "migration@example.com", PasswordHash: "hash", IsEnabled: yesno.Yes}
	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	storedRole := role.Role{Code: "migration_role", Name: "Migration Role", IsDefault: yesno.Yes, IsEnabled: yesno.Yes}
	if err := db.WithContext(ctx).Create(&storedRole).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Create(&role.UserRole{UserID: user.ID, RoleID: storedRole.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Create(&menu.RoleMenu{RoleID: storedRole.ID, MenuID: canvasAction.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Create(&permission.Version{UserID: user.ID, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	return rbacMigrationFixture{adminID: admin.ID, canvasID: canvas.ID, rootID: root.ID, pageID: canvasPage.ID, roleID: storedRole.ID, userID: user.ID}
}

func assertRBACMigrationState(t *testing.T, db *gorm.DB, ctx context.Context, fixture rbacMigrationFixture) {
	t.Helper()
	var profilePage menu.Menu
	if err := db.WithContext(ctx).Where("code = ?", "account:profile:list").Take(&profilePage).Error; err != nil {
		t.Fatal(err)
	}
	if profilePage.PlatformID != fixture.adminID || profilePage.ParentID == nil || *profilePage.ParentID != fixture.rootID || profilePage.Path == nil || *profilePage.Path != "/account/profile" || profilePage.IsHidden != yesno.Yes {
		t.Fatalf("profile page = %+v", profilePage)
	}
	var loginLogPage menu.Menu
	if err := db.WithContext(ctx).Where("code = ?", "account:user:loginlog:list").Take(&loginLogPage).Error; err != nil {
		t.Fatal(err)
	}
	if loginLogPage.PlatformID != fixture.adminID || loginLogPage.ParentID == nil || *loginLogPage.ParentID != fixture.rootID || loginLogPage.MenuType != menu.TypePage || loginLogPage.Path == nil || *loginLogPage.Path != "/account/login-logs" || loginLogPage.ComponentPath == nil || *loginLogPage.ComponentPath != "user/login-logs" || loginLogPage.IsHidden != yesno.No {
		t.Fatalf("login log page = %+v", loginLogPage)
	}
	var actions []menu.Menu
	if err := db.WithContext(ctx).Where("parent_id = ? AND code IN ?", profilePage.ID, []string{"account:profile:update", "account:password:update"}).Order("code").Find(&actions).Error; err != nil {
		t.Fatal(err)
	}
	if len(actions) != 2 || actions[0].Code != "account:password:update" || actions[1].Code != "account:profile:update" {
		t.Fatalf("profile actions = %+v", actions)
	}
	var canvasRows []menu.Menu
	if err := db.WithContext(ctx).Where("platform_id = ?", fixture.canvasID).Order("id").Find(&canvasRows).Error; err != nil {
		t.Fatal(err)
	}
	if len(canvasRows) != 2 || canvasRows[0].ID != fixture.pageID || canvasRows[0].Code != "canvas:test:list" || canvasRows[1].ParentID == nil || *canvasRows[1].ParentID != fixture.pageID {
		t.Fatalf("Canvas rows = %+v", canvasRows)
	}
	var grantCount int64
	if err := db.WithContext(ctx).Model(&menu.RoleMenu{}).Where("role_id = ? AND menu_id = ? AND deleted_at IS NULL", fixture.roleID, canvasRows[1].ID).Count(&grantCount).Error; err != nil || grantCount != 1 {
		t.Fatalf("Canvas grant count = %d, error=%v", grantCount, err)
	}
	var version int64
	if err := db.WithContext(ctx).Table("permission_access_version").Where("user_id = ?", fixture.userID).Pluck("version", &version).Error; err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("access version = %d, want 2", version)
	}
}

func readRBACBaselineMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(rbacRepoRoot(t), "docs", "database", "2026-08-29-admin-rbac-baseline.sql"))
	if err != nil {
		t.Fatalf("read RBAC migration: %v", err)
	}
	return string(data)
}

func rbacRepoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
}

func rbacStringPointer(value string) *string { return &value }
