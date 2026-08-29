package menu

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/authplatform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openMenuTransaction(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	db, ctx := openMenuDatabase(t)
	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	return tx, ctx
}

func openMenuDatabase(t *testing.T) (*gorm.DB, context.Context) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	root, err := database.Open(ctx, settings.PostgresDSN)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	schema := fmt.Sprintf("test_menu_%d", time.Now().UnixNano())
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
		t.Fatalf("AutoMigrate authentication platform test schema: %v", err)
	}
	if err := authplatform.EnsureSchema(ctx, db); err != nil {
		t.Fatalf("Ensure authentication platform schema: %v", err)
	}
	if err := database.AutoMigrate(ctx, db, &testRole{}, &testUser{}, &testAccessVersion{}, &Menu{}, &RoleMenu{}); err != nil {
		t.Fatalf("AutoMigrate menu test schema: %v", err)
	}
	if err := EnsureSchema(ctx, db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return db, ctx
}

func testAdminPlatformID(t *testing.T, db *gorm.DB, ctx context.Context) int64 {
	t.Helper()
	var platform authplatform.Platform
	if err := db.WithContext(ctx).Where("code = ?", authplatform.BuiltinAdminCode).Take(&platform).Error; err != nil {
		t.Fatalf("find builtin Admin platform: %v", err)
	}
	if platform.ID < 1 {
		t.Fatalf("builtin Admin platform id = %d", platform.ID)
	}
	return platform.ID
}

func stringPointer(value string) *string {
	return &value
}

func value(pointer *string) string {
	if pointer == nil {
		return ""
	}
	return *pointer
}
