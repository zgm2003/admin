package access_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/rbac/access"
	"admin/server/internal/module/user/account"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestAccessVersionTableName(t *testing.T) {
	if got := (access.Version{}).TableName(); got != "rbac_access_version" {
		t.Fatalf("TableName() = %q", got)
	}
}

func TestAccessVersionSchemaBackfillsUsers(t *testing.T) {
	connection, ctx := openAccessSchemaDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &account.User{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	createdUser := account.User{Username: fmt.Sprintf("version-%d", now.UnixNano()), Email: fmt.Sprintf("version-%d@example.com", now.UnixNano()), PasswordHash: "hash", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	if err := connection.GORM.WithContext(ctx).Create(&createdUser).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(ctx, connection.GORM, &access.Version{}); err != nil {
		t.Fatal(err)
	}
	if err := access.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	var legacyTableExists bool
	if err := connection.GORM.WithContext(ctx).Raw(
		`SELECT to_regclass(current_schema() || '.sys_access_version') IS NOT NULL`,
	).Scan(&legacyTableExists).Error; err != nil {
		t.Fatal(err)
	}
	if legacyTableExists {
		t.Fatal("legacy relation sys_access_version still exists")
	}
	if err := access.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("second EnsureSchema() error = %v", err)
	}

	var version access.Version
	if err := connection.GORM.WithContext(ctx).Take(&version, "user_id = ?", createdUser.ID).Error; err != nil {
		t.Fatal(err)
	}
	if version.Version != 1 || version.CreatedAt.IsZero() || version.UpdatedAt.IsZero() {
		t.Fatalf("access version = %+v", version)
	}

	for column, wantType := range map[string]string{
		"user_id": "bigint", "version": "bigint", "created_at": "timestamp with time zone", "updated_at": "timestamp with time zone",
	} {
		var got struct {
			DataType   string `gorm:"column:data_type"`
			IsNullable string `gorm:"column:is_nullable"`
		}
		if err := connection.GORM.WithContext(ctx).Raw(`
			SELECT data_type, is_nullable FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = 'rbac_access_version' AND column_name = ?`, column).Scan(&got).Error; err != nil {
			t.Fatal(err)
		}
		if got.DataType != wantType || got.IsNullable != "NO" {
			t.Errorf("column %s = %q/%q", column, got.DataType, got.IsNullable)
		}
	}
	var deletedAtCount int64
	if err := connection.GORM.WithContext(ctx).Raw(`
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'rbac_access_version' AND column_name = 'deleted_at'`).Scan(&deletedAtCount).Error; err != nil {
		t.Fatal(err)
	}
	if deletedAtCount != 0 {
		t.Fatal("rbac_access_version must not have deleted_at")
	}
	for name, fragments := range map[string][]string{
		"ck_rbac_access_version_version": {"CHECK", "version", ">= 1"},
		"fk_rbac_access_version_user":    {"FOREIGN KEY", "user_id", "ON DELETE RESTRICT"},
	} {
		var definition string
		if err := connection.GORM.WithContext(ctx).Raw(`
			SELECT pg_get_constraintdef(oid) FROM pg_constraint
			WHERE conname = ? AND connamespace = current_schema()::regnamespace`, name).Scan(&definition).Error; err != nil {
			t.Fatal(err)
		}
		for _, fragment := range fragments {
			if !strings.Contains(definition, fragment) {
				t.Errorf("constraint %s = %q, missing %q", name, definition, fragment)
			}
		}
	}
}

func openAccessSchemaDatabase(t *testing.T) (*database.Connection, context.Context) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	root, err := database.Open(ctx, settings.PostgresDSN)
	if err != nil {
		cancel()
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
		cancel()
	})
	return connection, ctx
}
