package access_test

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/access"
	"admin/server/internal/module/auth"
	"admin/server/internal/module/menu"
	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestRepositoryHasPermissionUsesDirectGrantAndAncestorSemantics(t *testing.T) {
	fixture := openRepositoryFixture(t)

	hasCreate, err := fixture.repository.HasPermission(fixture.ctx, fixture.user.ID, fixture.create.Code)
	if err != nil {
		t.Fatal(err)
	}
	hasView, err := fixture.repository.HasPermission(fixture.ctx, fixture.user.ID, fixture.page.Code)
	if err != nil {
		t.Fatal(err)
	}
	hasDelete, err := fixture.repository.HasPermission(fixture.ctx, fixture.user.ID, fixture.delete.Code)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCreate || !hasView || hasDelete {
		t.Fatalf("permissions create=%v view=%v delete=%v, want true true false", hasCreate, hasView, hasDelete)
	}

	var stored []menu.RoleMenu
	if err := fixture.tx.WithContext(fixture.ctx).Where("role_id = ?", fixture.primaryRole.ID).Find(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored[0].MenuID != fixture.create.ID {
		t.Fatalf("stored direct grants = %+v, want only create action %d", stored, fixture.create.ID)
	}
}

func TestRepositoryHasPermissionDoesNotExpandAPageGrantToActions(t *testing.T) {
	fixture := openRepositoryFixture(t)
	if err := fixture.tx.WithContext(fixture.ctx).Delete(&fixture.directGrant).Error; err != nil {
		t.Fatal(err)
	}
	pageGrant := menu.RoleMenu{RoleID: fixture.primaryRole.ID, MenuID: fixture.page.ID}
	if err := fixture.tx.WithContext(fixture.ctx).Create(&pageGrant).Error; err != nil {
		t.Fatal(err)
	}

	assertPermission(t, fixture, fixture.page.Code, true)
	assertPermission(t, fixture, fixture.create.Code, false)
	assertPermission(t, fixture, fixture.delete.Code, false)
}

func TestRepositoryHasPermissionUnionsMultipleRoles(t *testing.T) {
	fixture := openRepositoryFixture(t)
	secondUserRole := role.UserRole{UserID: fixture.user.ID, RoleID: fixture.secondaryRole.ID}
	if err := fixture.tx.WithContext(fixture.ctx).Create(&secondUserRole).Error; err != nil {
		t.Fatal(err)
	}
	deleteGrant := menu.RoleMenu{RoleID: fixture.secondaryRole.ID, MenuID: fixture.delete.ID}
	if err := fixture.tx.WithContext(fixture.ctx).Create(&deleteGrant).Error; err != nil {
		t.Fatal(err)
	}

	assertPermission(t, fixture, fixture.create.Code, true)
	assertPermission(t, fixture, fixture.delete.Code, true)
	assertPermission(t, fixture, fixture.page.Code, true)
}

func TestRepositoryHasPermissionFiltersInactiveRecords(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *repositoryFixture)
	}{
		{
			name: "soft-deleted role menu",
			mutate: func(t *testing.T, fixture *repositoryFixture) {
				t.Helper()
				if err := fixture.tx.WithContext(fixture.ctx).Delete(&fixture.directGrant).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "disabled role",
			mutate: func(t *testing.T, fixture *repositoryFixture) {
				t.Helper()
				if err := fixture.tx.WithContext(fixture.ctx).Model(&role.Role{}).Where("id = ?", fixture.primaryRole.ID).Update("is_enabled", yesno.No).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "disabled menu",
			mutate: func(t *testing.T, fixture *repositoryFixture) {
				t.Helper()
				if err := fixture.tx.WithContext(fixture.ctx).Model(&menu.Menu{}).Where("id = ?", fixture.create.ID).Update("is_enabled", yesno.No).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "deleted user",
			mutate: func(t *testing.T, fixture *repositoryFixture) {
				t.Helper()
				if err := fixture.tx.WithContext(fixture.ctx).Delete(&fixture.user).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "disabled user",
			mutate: func(t *testing.T, fixture *repositoryFixture) {
				t.Helper()
				if err := fixture.tx.WithContext(fixture.ctx).Model(&user.User{}).Where("id = ?", fixture.user.ID).Update("is_enabled", yesno.No).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := openRepositoryFixture(t)
			test.mutate(t, fixture)
			assertPermission(t, fixture, fixture.create.Code, false)
			assertPermission(t, fixture, fixture.page.Code, false)
		})
	}
}

func TestRepositoryHasPermissionGrantsSuperAdminOnlyExistingEnabledPermissions(t *testing.T) {
	fixture := openRepositoryFixture(t)
	if err := fixture.tx.WithContext(fixture.ctx).Delete(&fixture.userRole).Error; err != nil {
		t.Fatal(err)
	}
	roleRepository := role.NewRepository(fixture.tx)
	if err := roleRepository.EnsureSystemRoles(fixture.ctx); err != nil {
		t.Fatal(err)
	}
	superAdmin, err := roleRepository.FindByCode(fixture.ctx, role.CodeSuperAdmin)
	if err != nil {
		t.Fatal(err)
	}
	superAdminRelation := role.UserRole{UserID: fixture.user.ID, RoleID: superAdmin.ID}
	if err := fixture.tx.WithContext(fixture.ctx).Create(&superAdminRelation).Error; err != nil {
		t.Fatal(err)
	}

	assertPermission(t, fixture, fixture.page.Code, true)
	assertPermission(t, fixture, fixture.create.Code, true)
	assertPermission(t, fixture, fixture.root.Code, false)
	assertPermission(t, fixture, "missing:permission", false)
	if err := fixture.tx.WithContext(fixture.ctx).Model(&menu.Menu{}).Where("id = ?", fixture.delete.ID).Update("is_enabled", yesno.No).Error; err != nil {
		t.Fatal(err)
	}
	assertPermission(t, fixture, fixture.delete.Code, false)
}

func TestRepositoryFindSourceLoadsSortedRolesMenusAndDirectGrantIDs(t *testing.T) {
	fixture := openRepositoryFixture(t)
	secondUserRole := role.UserRole{UserID: fixture.user.ID, RoleID: fixture.secondaryRole.ID}
	if err := fixture.tx.WithContext(fixture.ctx).Create(&secondUserRole).Error; err != nil {
		t.Fatal(err)
	}
	deleteGrant := menu.RoleMenu{RoleID: fixture.secondaryRole.ID, MenuID: fixture.delete.ID}
	if err := fixture.tx.WithContext(fixture.ctx).Create(&deleteGrant).Error; err != nil {
		t.Fatal(err)
	}

	source, err := fixture.repository.FindSource(fixture.ctx, fixture.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantRoleCodes := []string{fixture.primaryRole.Code, fixture.secondaryRole.Code}
	sort.Strings(wantRoleCodes)
	if !reflect.DeepEqual(source.RoleCodes, wantRoleCodes) {
		t.Fatalf("role codes = %v, want %v", source.RoleCodes, wantRoleCodes)
	}
	if source.SuperAdmin {
		t.Fatal("custom roles were marked as super admin")
	}
	wantGrantedIDs := []int64{fixture.create.ID, fixture.delete.ID}
	sort.Slice(wantGrantedIDs, func(left, right int) bool { return wantGrantedIDs[left] < wantGrantedIDs[right] })
	gotGrantedIDs := append([]int64(nil), source.GrantedMenuIDs...)
	sort.Slice(gotGrantedIDs, func(left, right int) bool { return gotGrantedIDs[left] < gotGrantedIDs[right] })
	if !reflect.DeepEqual(gotGrantedIDs, wantGrantedIDs) {
		t.Fatalf("granted menu IDs = %v, want %v", gotGrantedIDs, wantGrantedIDs)
	}
	for _, expected := range []menu.Menu{fixture.root, fixture.page, fixture.create, fixture.delete} {
		if !containsMenuID(source.Menus, expected.ID) {
			t.Errorf("enabled menu %d missing from source", expected.ID)
		}
	}
}

func TestRepositoryFindSourceRejectsUsersWithoutAnActiveRole(t *testing.T) {
	fixture := openRepositoryFixture(t)
	if err := fixture.tx.WithContext(fixture.ctx).Delete(&fixture.userRole).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.FindSource(fixture.ctx, fixture.user.ID); err == nil {
		t.Fatal("FindSource accepted a user without an active role")
	}
}

type repositoryFixture struct {
	tx            *gorm.DB
	ctx           context.Context
	repository    *access.Repository
	user          user.User
	primaryRole   role.Role
	secondaryRole role.Role
	userRole      role.UserRole
	directGrant   menu.RoleMenu
	root          menu.Menu
	page          menu.Menu
	create        menu.Menu
	delete        menu.Menu
}

func openRepositoryFixture(t *testing.T) *repositoryFixture {
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	connection, err := database.Open(ctx, settings.PostgresDSN)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	if err := database.AutoMigrate(ctx, connection.GORM,
		&user.User{}, &role.Role{}, &role.UserRole{}, &menu.Menu{}, &menu.RoleMenu{}, &auth.Session{}); err != nil {
		t.Fatalf("AutoMigrate access schema: %v", err)
	}
	if err := auth.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("EnsureSchema auth: %v", err)
	}
	if err := menu.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("EnsureSchema menu: %v", err)
	}
	tx := connection.GORM.WithContext(ctx).Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })

	unique := time.Now().UnixNano()
	primaryRole := role.Role{Code: fmt.Sprintf("access_role_a_%d", unique), Name: "Access Role A", IsDefault: yesno.No, IsEnabled: yesno.Yes}
	secondaryRole := role.Role{Code: fmt.Sprintf("access_role_b_%d", unique), Name: "Access Role B", IsDefault: yesno.No, IsEnabled: yesno.Yes}
	if err := tx.Create(&primaryRole).Error; err != nil {
		t.Fatalf("create primary role: %v", err)
	}
	if err := tx.Create(&secondaryRole).Error; err != nil {
		t.Fatalf("create secondary role: %v", err)
	}

	createdUser := user.User{
		Username:     fmt.Sprintf("access_user_%d", unique),
		Email:        fmt.Sprintf("access_user_%d@example.com", unique),
		PasswordHash: "not-a-real-hash",
		IsEnabled:    yesno.Yes,
	}
	if err := tx.Create(&createdUser).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	userRole := role.UserRole{UserID: createdUser.ID, RoleID: primaryRole.ID}
	if err := tx.Create(&userRole).Error; err != nil {
		t.Fatalf("create user role: %v", err)
	}

	root := menu.Menu{
		MenuType: menu.TypeDirectory, Code: fmt.Sprintf("repository:%d", unique), I18nKey: "navigation.system",
		SortOrder: 10, IsEnabled: yesno.Yes,
	}
	if err := tx.Create(&root).Error; err != nil {
		t.Fatalf("create root menu: %v", err)
	}
	pagePath := fmt.Sprintf("/repository-%d/users", unique)
	pageView := "systemUsers"
	page := menu.Menu{
		ParentID: &root.ID, MenuType: menu.TypePage, Code: fmt.Sprintf("repository:%d:user:view", unique),
		I18nKey: "navigation.systemUsers", Path: &pagePath, ViewKey: &pageView, SortOrder: 10, IsEnabled: yesno.Yes,
	}
	if err := tx.Create(&page).Error; err != nil {
		t.Fatalf("create page menu: %v", err)
	}
	createAction := menu.Menu{
		ParentID: &page.ID, MenuType: menu.TypeAction, Code: fmt.Sprintf("repository:%d:user:create", unique),
		I18nKey: "permission.userCreate", SortOrder: 10, IsEnabled: yesno.Yes,
	}
	if err := tx.Create(&createAction).Error; err != nil {
		t.Fatalf("create create action: %v", err)
	}
	deleteAction := menu.Menu{
		ParentID: &page.ID, MenuType: menu.TypeAction, Code: fmt.Sprintf("repository:%d:user:delete", unique),
		I18nKey: "permission.userDelete", SortOrder: 20, IsEnabled: yesno.Yes,
	}
	if err := tx.Create(&deleteAction).Error; err != nil {
		t.Fatalf("create delete action: %v", err)
	}
	directGrant := menu.RoleMenu{RoleID: primaryRole.ID, MenuID: createAction.ID}
	if err := tx.Create(&directGrant).Error; err != nil {
		t.Fatalf("create direct role grant: %v", err)
	}

	return &repositoryFixture{
		tx: tx, ctx: ctx, repository: access.NewRepository(tx), user: createdUser,
		primaryRole: primaryRole, secondaryRole: secondaryRole, userRole: userRole, directGrant: directGrant,
		root: root, page: page, create: createAction, delete: deleteAction,
	}
}

func assertPermission(t *testing.T, fixture *repositoryFixture, code string, want bool) {
	t.Helper()
	got, err := fixture.repository.HasPermission(fixture.ctx, fixture.user.ID, code)
	if err != nil {
		t.Fatalf("HasPermission(%q) error = %v", code, err)
	}
	if got != want {
		t.Fatalf("HasPermission(%q) = %v, want %v", code, got, want)
	}
}

func containsMenuID(menus []menu.Menu, menuID int64) bool {
	for _, item := range menus {
		if item.ID == menuID {
			return true
		}
	}
	return false
}
