package authplatform_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/authplatform"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPlatformTableName(t *testing.T) {
	if got := (authplatform.Platform{}).TableName(); got != "sys_auth_platform" {
		t.Fatalf("TableName() = %q", got)
	}
}

func TestAuthenticationPlatformSchemaAndBuiltinAdmin(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &authplatform.Platform{}); err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("second EnsureSchema() error = %v", err)
	}

	expectedColumns := map[string]struct {
		dataType string
		nullable string
	}{
		"id":                        {dataType: "bigint", nullable: "NO"},
		"code":                      {dataType: "character varying", nullable: "NO"},
		"name":                      {dataType: "character varying", nullable: "NO"},
		"policy_version":            {dataType: "bigint", nullable: "NO"},
		"access_ttl_seconds":        {dataType: "integer", nullable: "NO"},
		"refresh_ttl_seconds":       {dataType: "integer", nullable: "NO"},
		"session_cache_ttl_seconds": {dataType: "integer", nullable: "NO"},
		"access_cache_ttl_seconds":  {dataType: "integer", nullable: "NO"},
		"bind_device":               {dataType: "smallint", nullable: "NO"},
		"bind_ip":                   {dataType: "smallint", nullable: "NO"},
		"max_sessions":              {dataType: "smallint", nullable: "NO"},
		"allow_register":            {dataType: "smallint", nullable: "NO"},
		"is_enabled":                {dataType: "smallint", nullable: "NO"},
		"is_builtin":                {dataType: "smallint", nullable: "NO"},
		"created_at":                {dataType: "timestamp with time zone", nullable: "NO"},
		"updated_at":                {dataType: "timestamp with time zone", nullable: "NO"},
		"deleted_at":                {dataType: "timestamp with time zone", nullable: "YES"},
	}
	for column, want := range expectedColumns {
		var got struct {
			DataType   string  `gorm:"column:data_type"`
			IsNullable string  `gorm:"column:is_nullable"`
			Default    *string `gorm:"column:column_default"`
		}
		if err := connection.GORM.WithContext(ctx).Raw(`
			SELECT data_type, is_nullable, column_default
			FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = 'sys_auth_platform' AND column_name = ?`, column).Scan(&got).Error; err != nil {
			t.Fatal(err)
		}
		if got.DataType != want.dataType || got.IsNullable != want.nullable {
			t.Errorf("column %s = %q/%q, want %q/%q", column, got.DataType, got.IsNullable, want.dataType, want.nullable)
		}
		if (column == "created_at" || column == "updated_at") && (got.Default == nil || *got.Default == "") {
			t.Errorf("column %s lacks a default", column)
		}
	}

	constraints := map[string][]string{
		"ck_sys_auth_platform_code":                      {"CHECK", "code"},
		"ck_sys_auth_platform_policy_version":            {"CHECK", "policy_version", ">= 1"},
		"ck_sys_auth_platform_access_ttl_seconds":        {"CHECK", "access_ttl_seconds", "2592000"},
		"ck_sys_auth_platform_refresh_ttl_seconds":       {"CHECK", "refresh_ttl_seconds", "31536000"},
		"ck_sys_auth_platform_session_cache_ttl_seconds": {"CHECK", "session_cache_ttl_seconds", "86400"},
		"ck_sys_auth_platform_access_cache_ttl_seconds":  {"CHECK", "access_cache_ttl_seconds", "86400"},
		"ck_sys_auth_platform_bind_device":               {"CHECK", "bind_device"},
		"ck_sys_auth_platform_bind_ip":                   {"CHECK", "bind_ip"},
		"ck_sys_auth_platform_max_sessions":              {"CHECK", "max_sessions", "100"},
		"ck_sys_auth_platform_allow_register":            {"CHECK", "allow_register"},
		"ck_sys_auth_platform_is_enabled":                {"CHECK", "is_enabled"},
		"ck_sys_auth_platform_is_builtin":                {"CHECK", "is_builtin"},
	}
	for name, fragments := range constraints {
		definition := platformConstraintDefinition(t, connection, ctx, name)
		for _, fragment := range fragments {
			if !strings.Contains(definition, fragment) {
				t.Errorf("constraint %s = %q, missing %q", name, definition, fragment)
			}
		}
	}
	var indexDefinition string
	if err := connection.GORM.WithContext(ctx).Raw(`
		SELECT indexdef FROM pg_indexes
		WHERE schemaname = current_schema() AND indexname = 'ux_sys_auth_platform_code_active'`).Scan(&indexDefinition).Error; err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"CREATE UNIQUE INDEX", "(code)", "WHERE (deleted_at IS NULL)"} {
		if !strings.Contains(indexDefinition, fragment) {
			t.Errorf("platform index = %q, missing %q", indexDefinition, fragment)
		}
	}

	var admin authplatform.Platform
	if err := connection.GORM.WithContext(ctx).Where("code = ?", "admin").Take(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if admin.Name != "Admin" || admin.PolicyVersion != 1 || admin.AccessTTLSeconds != 900 || admin.RefreshTTLSeconds != 1209600 || admin.SessionCacheTTLSeconds != 1800 || admin.AccessCacheTTLSeconds != 1800 || admin.BindDevice != yesno.No || admin.BindIP != yesno.No || admin.MaxSessions != 1 || admin.AllowRegister != yesno.Yes || admin.IsEnabled != yesno.Yes || admin.IsBuiltin != yesno.Yes || admin.DeletedAt.Valid {
		t.Fatalf("builtin admin = %+v", admin)
	}
}

func TestAuthenticationPlatformSchemaRejectsDamagedBuiltin(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &authplatform.Platform{}); err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	if err := connection.GORM.WithContext(ctx).Model(&authplatform.Platform{}).Where("code = ?", "admin").Update("name", "Damaged").Error; err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, connection.GORM); err == nil {
		t.Fatal("damaged builtin admin was silently accepted")
	}
}

func platformConstraintDefinition(t *testing.T, connection *database.Connection, ctx context.Context, name string) string {
	t.Helper()
	var definition string
	if err := connection.GORM.WithContext(ctx).Raw(`
		SELECT pg_get_constraintdef(oid)
		FROM pg_constraint
		WHERE conname = ? AND connamespace = current_schema()::regnamespace`, name).Scan(&definition).Error; err != nil {
		t.Fatal(err)
	}
	if definition == "" {
		t.Fatalf("constraint %s does not exist", name)
	}
	return definition
}

func openAuthenticationPlatformDatabase(t *testing.T) (*database.Connection, context.Context) {
	t.Helper()
	return openScopedPostgreSQL(t, "authplatform")
}

func openScopedPostgreSQL(t *testing.T, prefix string) (*database.Connection, context.Context) {
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
	root, err := database.Open(ctx, settings.PostgresDSN)
	if err != nil {
		cancel()
		t.Fatalf("open PostgreSQL: %v", err)
	}
	schema := fmt.Sprintf("test_%s_%d", prefix, time.Now().UnixNano())
	if err := root.GORM.WithContext(ctx).Exec("CREATE SCHEMA " + schema).Error; err != nil {
		_ = root.Close()
		cancel()
		t.Fatalf("create test schema: %v", err)
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
