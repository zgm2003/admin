package database_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/database/testschema"
	authplatform "admin/server/internal/module/auth/platform"
	"admin/server/internal/module/rbac/menu"
	"admin/server/internal/module/rbac/role"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestCOSObjectStorageMigration(t *testing.T) {
	db, ctx := openCOSMigrationSchema(t)
	admin := ensureCOSAdmin(t, db, ctx)
	script := readCOSMigration(t)
	delta := readCOSRuleUnificationMigration(t)
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("execute COS migration: %v", err)
	}
	if err := db.WithContext(ctx).Exec(delta).Error; err != nil {
		t.Fatalf("execute COS rule unification migration: %v", err)
	}
	for _, table := range []string{"storage_cos_config", "storage_upload_rule"} {
		assertRelation(t, db, ctx, table, true)
	}
	for _, name := range []string{"fk_storage_upload_rule_platform", "fk_storage_upload_rule_cos_config", "ck_storage_cos_config_is_enabled", "ck_storage_upload_rule_is_enabled", "ck_storage_upload_rule_max_file_size", "ck_storage_upload_rule_access_mode"} {
		assertNamedConstraint(t, db, ctx, name)
	}
	for _, name := range []string{"ux_storage_cos_config_name_active", "ix_storage_cos_config_enabled_created_at", "ux_storage_upload_rule_platform_code_active", "ix_storage_upload_rule_config_enabled_created_at"} {
		assertNamedIndex(t, db, ctx, name)
	}
	var configID int64
	if err := db.WithContext(ctx).Raw(`INSERT INTO storage_cos_config (name, app_id, secret_id_ciphertext, secret_key_ciphertext, bucket, region) VALUES ('main', 'app', 'sid', 'skey', 'bucket', 'region') RETURNING id`).Scan(&configID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO storage_upload_rule (platform_id, code, name, cos_config_id, max_file_size_bytes, allowed_extensions, allowed_mime_types, access_mode) VALUES (?, 'avatar', 'Avatar', ?, 100, ARRAY['png']::text[], ARRAY['image/png']::text[], 'private')`, admin.ID, configID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO storage_upload_rule (platform_id, code, name, cos_config_id, max_file_size_bytes, allowed_extensions, allowed_mime_types, access_mode, is_enabled) VALUES (?, 'avatar-disabled', 'Disabled', ?, 100, ARRAY['png']::text[], ARRAY['image/png']::text[], 'private', 0)`, admin.ID, configID).Error; err != nil {
		t.Fatal(err)
	}
}

func TestCOSObjectStorageMigrationAllowsMultipleEnabledRules(t *testing.T) {
	db, ctx := openCOSMigrationSchema(t)
	admin := ensureCOSAdmin(t, db, ctx)
	if err := db.WithContext(ctx).Exec(readCOSMigration(t)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(readCOSRuleUnificationMigration(t)).Error; err != nil {
		t.Fatal(err)
	}
	var configID int64
	if err := db.WithContext(ctx).Raw(`INSERT INTO storage_cos_config (name, app_id, secret_id_ciphertext, secret_key_ciphertext, bucket, region) VALUES ('main', 'app', 'sid', 'skey', 'bucket', 'region') RETURNING id`).Scan(&configID).Error; err != nil {
		t.Fatal(err)
	}
	insert := `INSERT INTO storage_upload_rule (platform_id, code, name, cos_config_id, max_file_size_bytes, allowed_extensions, allowed_mime_types, access_mode) VALUES (?, ?, ?, ?, 100, ARRAY['png']::text[], ARRAY['image/png']::text[], ?)`
	if err := db.WithContext(ctx).Exec(insert, admin.ID, "one", "One", configID, "private").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(insert, admin.ID, "two", "Two", configID, "private").Error; err != nil {
		t.Fatalf("second enabled rule rejected: %v", err)
	}
	disabledInsert := `INSERT INTO storage_upload_rule (platform_id, code, name, cos_config_id, max_file_size_bytes, allowed_extensions, allowed_mime_types, access_mode, is_enabled) VALUES (?, ?, ?, ?, 100, ARRAY['png']::text[], ARRAY['image/png']::text[], ?, 0)`
	if err := db.WithContext(ctx).Exec(disabledInsert, admin.ID, "disabled", "Disabled", configID, "private").Error; err != nil {
		t.Fatal(err)
	}
}

func openCOSMigrationSchema(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	if err := godotenv.Load(filepath.Join(cosRepoRoot(t), "server", ".env")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_cos_storage_migration")
	if err := database.AutoMigrate(ctx, db, &authplatform.Platform{}, &role.Role{}, &role.UserRole{}, &menu.Menu{}, &menu.RoleMenu{}); err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := menu.EnsureSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func ensureCOSAdmin(t *testing.T, db *gorm.DB, ctx context.Context) authplatform.Platform {
	t.Helper()
	var admin authplatform.Platform
	if err := db.WithContext(ctx).Where("code = ?", authplatform.BuiltinAdminCode).Take(&admin).Error; err != nil {
		t.Fatal(err)
	}
	return admin
}

func readCOSMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(cosRepoRoot(t), "docs", "database", "2026-08-29-cos-object-storage.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func readCOSRuleUnificationMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(cosRepoRoot(t), "docs", "database", "2026-08-30-cos-upload-rule-code-unification.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func cosRepoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
}

func assertNamedConstraint(t *testing.T, db *gorm.DB, ctx context.Context, name string) {
	t.Helper()
	var exists bool
	if err := db.WithContext(ctx).Raw(`SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = ? AND connamespace = current_schema()::regnamespace)`, name).Scan(&exists).Error; err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatalf("constraint %s missing", name)
	}
}

func assertNamedIndex(t *testing.T, db *gorm.DB, ctx context.Context, name string) {
	t.Helper()
	var exists bool
	if err := db.WithContext(ctx).Raw(`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = current_schema() AND indexname = ?)`, name).Scan(&exists).Error; err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatalf("index %s missing", name)
	}
}

var _ = errors.Is
var _ = yesno.Yes
