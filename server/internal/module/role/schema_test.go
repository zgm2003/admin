package role_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/database/testschema"
	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestRoleSchema(t *testing.T) {
	db, ctx := openRoleSchema(t)
	if got := (role.Role{}).TableName(); got != "rbac_role" {
		t.Fatalf("Role.TableName() = %q", got)
	}
	if got := (role.UserRole{}).TableName(); got != "rbac_user_role" {
		t.Fatalf("UserRole.TableName() = %q", got)
	}

	for name, fragments := range map[string][]string{
		"ux_rbac_role_code_active":    {"CREATE UNIQUE INDEX", "(code)", "deleted_at IS NULL"},
		"ux_rbac_role_name_active":    {"CREATE UNIQUE INDEX", "(name)", "deleted_at IS NULL"},
		"ux_rbac_role_default_active": {"CREATE UNIQUE INDEX", "(is_default)", "is_default = 1", "deleted_at IS NULL"},
		"ux_rbac_user_role_active":    {"CREATE UNIQUE INDEX", "(user_id, role_id)", "deleted_at IS NULL"},
	} {
		definition := roleIndexDefinition(t, db, ctx, name)
		for _, fragment := range fragments {
			if !strings.Contains(definition, fragment) {
				t.Errorf("index %s = %q, missing %q", name, definition, fragment)
			}
		}
	}

	for name, fragments := range map[string][]string{
		"fk_rbac_user_role_user":  {"FOREIGN KEY (user_id)", "user_account(id)", "ON DELETE RESTRICT"},
		"fk_rbac_user_role_role":  {"FOREIGN KEY (role_id)", "rbac_role(id)", "ON DELETE RESTRICT"},
		"ck_rbac_role_is_default": {"CHECK", "is_default", "0", "1"},
		"ck_rbac_role_is_enabled": {"CHECK", "is_enabled", "0", "1"},
	} {
		definition := roleConstraintDefinition(t, db, ctx, name)
		for _, fragment := range fragments {
			if !strings.Contains(definition, fragment) {
				t.Errorf("constraint %s = %q, missing %q", name, definition, fragment)
			}
		}
	}
	assertRoleRelationMissing(t, db, ctx, "sys_role")
	assertRoleRelationMissing(t, db, ctx, "sys_user_role")
}

func openRoleSchema(t *testing.T) (*gorm.DB, context.Context) {
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
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_role_schema")
	if err := database.AutoMigrate(ctx, db, &user.User{}, &role.Role{}, &role.UserRole{}); err != nil {
		t.Fatalf("AutoMigrate role schema: %v", err)
	}
	if err := role.EnsureSchema(ctx, db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return db, ctx
}

func roleConstraintDefinition(t *testing.T, db *gorm.DB, ctx context.Context, name string) string {
	t.Helper()
	var definition string
	if err := db.WithContext(ctx).Raw(`
		SELECT pg_get_constraintdef(oid)
		FROM pg_constraint
		WHERE conname = ? AND connamespace = current_schema()::regnamespace`, name).Scan(&definition).Error; err != nil {
		t.Fatalf("inspect constraint %s: %v", name, err)
	}
	if definition == "" {
		t.Fatalf("constraint %s does not exist", name)
	}
	return definition
}

func roleIndexDefinition(t *testing.T, db *gorm.DB, ctx context.Context, name string) string {
	t.Helper()
	var definition string
	if err := db.WithContext(ctx).Raw(`
		SELECT indexdef FROM pg_indexes
		WHERE schemaname = current_schema() AND indexname = ?`, name).Scan(&definition).Error; err != nil {
		t.Fatalf("inspect index %s: %v", name, err)
	}
	if definition == "" {
		t.Fatalf("index %s does not exist", name)
	}
	return definition
}

func assertRoleRelationMissing(t *testing.T, db *gorm.DB, ctx context.Context, name string) {
	t.Helper()
	var exists bool
	if err := db.WithContext(ctx).Raw(
		`SELECT to_regclass(current_schema() || '.' || ?) IS NOT NULL`, name,
	).Scan(&exists).Error; err != nil {
		t.Fatalf("inspect relation %s: %v", name, err)
	}
	if exists {
		t.Fatalf("legacy relation %s still exists", name)
	}
}
