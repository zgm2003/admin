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
	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestEnsureSystemRolesCreatesAndValidatesRoles(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	repository := role.NewRepository(tx)

	if err := repository.EnsureSystemRoles(ctx); err != nil {
		t.Fatalf("EnsureSystemRoles() error = %v", err)
	}
	if err := repository.EnsureSystemRoles(ctx); err != nil {
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
	if err := repository.EnsureSystemRoles(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Model(&role.Role{}).Where("code = ?", role.CodeRegisteredUser).Update("name", "错误名称").Error; err != nil {
		t.Fatal(err)
	}

	if err := repository.EnsureSystemRoles(ctx); err == nil {
		t.Fatal("mutated system role was silently accepted")
	}
}

func TestFindDefaultRequiresExactlyOneEnabledRole(t *testing.T) {
	t.Run("finds the configured default", func(t *testing.T) {
		tx, ctx := openRoleTransaction(t)
		repository := role.NewRepository(tx)
		if err := repository.EnsureSystemRoles(ctx); err != nil {
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
		if err := repository.EnsureSystemRoles(ctx); err != nil {
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
		if err := repository.EnsureSystemRoles(ctx); err != nil {
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
		if err := repository.EnsureSystemRoles(ctx); err != nil {
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
		if err := repository.EnsureSystemRoles(ctx); err != nil {
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
	if err := repository.EnsureSystemRoles(ctx); err != nil {
		t.Fatal(err)
	}
	adminRole, err := repository.FindByCode(ctx, role.CodeSuperAdmin)
	if err != nil {
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
	relation := role.UserRole{UserID: createdUser.ID, RoleID: adminRole.ID}
	if err := tx.WithContext(ctx).Create(&relation).Error; err != nil {
		t.Fatal(err)
	}

	found, err := repository.HasActiveUserWithRole(ctx, adminRole.ID)
	if err != nil || !found {
		t.Fatalf("HasActiveUserWithRole() = %v,%v", found, err)
	}
	if err := tx.WithContext(ctx).Delete(&relation).Error; err != nil {
		t.Fatal(err)
	}
	found, err = repository.HasActiveUserWithRole(ctx, adminRole.ID)
	if err != nil || found {
		t.Fatalf("after relation deletion = %v,%v", found, err)
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
	if err := database.AutoMigrate(ctx, connection.GORM, &user.User{}, &role.Role{}, &role.UserRole{}, &auth.Session{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := auth.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	tx := connection.GORM.WithContext(ctx).Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	return tx, ctx
}
