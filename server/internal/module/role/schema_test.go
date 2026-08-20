package role_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	"github.com/joho/godotenv"
)

func TestRoleSchema(t *testing.T) {
	connection, ctx := openRoleSchema(t)

	for name, fragments := range map[string][]string{
		"ux_sys_role_code_active":    {"CREATE UNIQUE INDEX", "(code)", "deleted_at IS NULL"},
		"ux_sys_role_name_active":    {"CREATE UNIQUE INDEX", "(name)", "deleted_at IS NULL"},
		"ux_sys_role_default_active": {"CREATE UNIQUE INDEX", "(is_default)", "is_default = 1", "deleted_at IS NULL"},
		"ux_sys_user_role_active":    {"CREATE UNIQUE INDEX", "(user_id, role_id)", "deleted_at IS NULL"},
	} {
		definition := roleIndexDefinition(t, connection, ctx, name)
		for _, fragment := range fragments {
			if !strings.Contains(definition, fragment) {
				t.Errorf("index %s = %q, missing %q", name, definition, fragment)
			}
		}
	}

	for name, fragments := range map[string][]string{
		"fk_sys_user_role_user":  {"FOREIGN KEY (user_id)", "sys_user(id)", "ON DELETE RESTRICT"},
		"fk_sys_user_role_role":  {"FOREIGN KEY (role_id)", "sys_role(id)", "ON DELETE RESTRICT"},
		"ck_sys_role_is_default": {"CHECK", "is_default", "0", "1"},
		"ck_sys_role_is_enabled": {"CHECK", "is_enabled", "0", "1"},
	} {
		definition := roleConstraintDefinition(t, connection, ctx, name)
		for _, fragment := range fragments {
			if !strings.Contains(definition, fragment) {
				t.Errorf("constraint %s = %q, missing %q", name, definition, fragment)
			}
		}
	}
}

func openRoleSchema(t *testing.T) (*database.Connection, context.Context) {
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
	if err := database.AutoMigrate(ctx, connection.GORM, &user.User{}, &role.Role{}, &role.UserRole{}); err != nil {
		t.Fatalf("AutoMigrate role schema: %v", err)
	}
	if err := role.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return connection, ctx
}

func roleConstraintDefinition(t *testing.T, connection *database.Connection, ctx context.Context, name string) string {
	t.Helper()
	var definition string
	if err := connection.GORM.WithContext(ctx).Raw(`
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

func roleIndexDefinition(t *testing.T, connection *database.Connection, ctx context.Context, name string) string {
	t.Helper()
	var definition string
	if err := connection.GORM.WithContext(ctx).Raw(`
		SELECT indexdef FROM pg_indexes
		WHERE schemaname = current_schema() AND indexname = ?`, name).Scan(&definition).Error; err != nil {
		t.Fatalf("inspect index %s: %v", name, err)
	}
	if definition == "" {
		t.Fatalf("index %s does not exist", name)
	}
	return definition
}
