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
	"admin/server/internal/module/auth/platform"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPlatformTableName(t *testing.T) {
	if got := (authplatform.Platform{}).TableName(); got != "auth_platform" {
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
	var legacyTableExists bool
	if err := connection.GORM.WithContext(ctx).Raw(
		`SELECT to_regclass(current_schema() || '.sys_auth_platform') IS NOT NULL`,
	).Scan(&legacyTableExists).Error; err != nil {
		t.Fatal(err)
	}
	if legacyTableExists {
		t.Fatal("legacy relation sys_auth_platform still exists")
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
			WHERE table_schema = current_schema() AND table_name = 'auth_platform' AND column_name = ?`, column).Scan(&got).Error; err != nil {
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
		"ck_auth_platform_code":                      {"CHECK", "code"},
		"ck_auth_platform_policy_version":            {"CHECK", "policy_version", ">= 1"},
		"ck_auth_platform_access_ttl_seconds":        {"CHECK", "access_ttl_seconds", "2592000"},
		"ck_auth_platform_refresh_ttl_seconds":       {"CHECK", "refresh_ttl_seconds", "31536000"},
		"ck_auth_platform_session_cache_ttl_seconds": {"CHECK", "session_cache_ttl_seconds", "86400"},
		"ck_auth_platform_access_cache_ttl_seconds":  {"CHECK", "access_cache_ttl_seconds", "86400"},
		"ck_auth_platform_bind_device":               {"CHECK", "bind_device"},
		"ck_auth_platform_bind_ip":                   {"CHECK", "bind_ip"},
		"ck_auth_platform_max_sessions":              {"CHECK", "max_sessions", "100"},
		"ck_auth_platform_allow_register":            {"CHECK", "allow_register"},
		"ck_auth_platform_is_enabled":                {"CHECK", "is_enabled"},
		"ck_auth_platform_is_builtin":                {"CHECK", "is_builtin"},
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
		WHERE schemaname = current_schema() AND indexname = 'ux_auth_platform_code_active'`).Scan(&indexDefinition).Error; err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"CREATE UNIQUE INDEX", "(code)", "WHERE (deleted_at IS NULL)"} {
		if !strings.Contains(indexDefinition, fragment) {
			t.Errorf("platform index = %q, missing %q", indexDefinition, fragment)
		}
	}

	admin := readAdmin(t, connection.GORM)
	if admin.Name != "Admin" || admin.PolicyVersion != 1 || admin.AccessTTLSeconds != 900 || admin.RefreshTTLSeconds != 1209600 || admin.SessionCacheTTLSeconds != 1800 || admin.AccessCacheTTLSeconds != 1800 || admin.BindDevice != yesno.No || admin.BindIP != yesno.No || admin.MaxSessions != 1 || admin.AllowRegister != yesno.No || admin.IsEnabled != yesno.Yes || admin.IsBuiltin != yesno.Yes || admin.DeletedAt.Valid {
		t.Fatalf("builtin admin = %+v", admin)
	}
}

func TestEnsureCanvasPresetCreatesBuiltinRegistrationPlatformAndIsIdempotent(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &authplatform.Platform{}); err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureCanvasPreset(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}

	var first authplatform.Platform
	if err := connection.GORM.WithContext(ctx).Where("code = ?", "canvas").Take(&first).Error; err != nil {
		t.Fatal(err)
	}
	if first.Name != "Canvas" || first.PolicyVersion != 1 || first.AccessTTLSeconds != 900 ||
		first.RefreshTTLSeconds != 1_209_600 || first.SessionCacheTTLSeconds != 1_800 ||
		first.AccessCacheTTLSeconds != 1_800 || first.BindDevice != yesno.No || first.BindIP != yesno.No ||
		first.MaxSessions != 1 || first.AllowRegister != yesno.Yes || first.IsEnabled != yesno.Yes ||
		first.IsBuiltin != yesno.Yes || first.DeletedAt.Valid {
		t.Fatalf("Canvas preset = %+v", first)
	}

	if err := authplatform.EnsureCanvasPreset(ctx, connection.GORM); err != nil {
		t.Fatalf("second EnsureCanvasPreset() error = %v", err)
	}
	var second authplatform.Platform
	if err := connection.GORM.WithContext(ctx).Where("code = ?", "canvas").Take(&second).Error; err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.PolicyVersion != first.PolicyVersion || !second.UpdatedAt.Equal(first.UpdatedAt) {
		t.Fatalf("Canvas preset was not idempotent: first=%+v second=%+v", first, second)
	}
}

func TestEnsureCanvasPresetPromotesExistingPlatformWithoutResettingPolicy(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &authplatform.Platform{}); err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	existing := validHistoricalAdmin(now)
	existing.ID = 0
	existing.Code = "canvas"
	existing.Name = "Canvas Local"
	existing.PolicyVersion = 7
	existing.AccessTTLSeconds = 1_200
	existing.RefreshTTLSeconds = 1_300_000
	existing.SessionCacheTTLSeconds = 1_900
	existing.AccessCacheTTLSeconds = 2_000
	existing.BindDevice = yesno.Yes
	existing.BindIP = yesno.Yes
	existing.MaxSessions = 3
	existing.AllowRegister = yesno.No
	existing.IsEnabled = yesno.No
	existing.IsBuiltin = yesno.No
	if err := connection.GORM.WithContext(ctx).Create(&existing).Error; err != nil {
		t.Fatal(err)
	}

	if err := authplatform.EnsureCanvasPreset(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	var promoted authplatform.Platform
	if err := connection.GORM.WithContext(ctx).Where("code = ?", "canvas").Take(&promoted).Error; err != nil {
		t.Fatal(err)
	}
	if promoted.ID != existing.ID || promoted.IsBuiltin != yesno.Yes || promoted.PolicyVersion != 8 ||
		promoted.Name != existing.Name || promoted.AccessTTLSeconds != existing.AccessTTLSeconds ||
		promoted.RefreshTTLSeconds != existing.RefreshTTLSeconds || promoted.SessionCacheTTLSeconds != existing.SessionCacheTTLSeconds ||
		promoted.AccessCacheTTLSeconds != existing.AccessCacheTTLSeconds || promoted.BindDevice != existing.BindDevice ||
		promoted.BindIP != existing.BindIP || promoted.MaxSessions != existing.MaxSessions ||
		promoted.AllowRegister != existing.AllowRegister || promoted.IsEnabled != existing.IsEnabled ||
		!promoted.CreatedAt.Equal(existing.CreatedAt) || !promoted.UpdatedAt.After(existing.UpdatedAt) {
		t.Fatalf("promoted Canvas platform = %+v, existing = %+v", promoted, existing)
	}
}

func TestEnsureSchemaMigratesBuiltinAdminRegistrationOnce(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &authplatform.Platform{}); err != nil {
		t.Fatal(err)
	}
	db := connection.GORM.WithContext(ctx)
	now := time.Now().UTC().Truncate(time.Microsecond)
	historical := validHistoricalAdmin(now)
	historical.PolicyVersion = 7
	historical.Name = "Operations Admin"
	historical.AccessTTLSeconds = 1200
	historical.RefreshTTLSeconds = 1_300_000
	historical.SessionCacheTTLSeconds = 1_900
	historical.AccessCacheTTLSeconds = 2_000
	historical.BindDevice = yesno.Yes
	historical.BindIP = yesno.Yes
	historical.MaxSessions = 2
	historical.IsEnabled = yesno.No
	if err := db.Create(&historical).Error; err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	first := readAdmin(t, db)
	if first.AllowRegister != yesno.No || first.PolicyVersion != 8 {
		t.Fatalf("first migration = %+v", first)
	}
	if first.Name != historical.Name || first.AccessTTLSeconds != historical.AccessTTLSeconds || first.RefreshTTLSeconds != historical.RefreshTTLSeconds || first.SessionCacheTTLSeconds != historical.SessionCacheTTLSeconds || first.AccessCacheTTLSeconds != historical.AccessCacheTTLSeconds || first.BindDevice != historical.BindDevice || first.BindIP != historical.BindIP || first.MaxSessions != historical.MaxSessions || first.IsEnabled != historical.IsEnabled || !first.CreatedAt.Equal(historical.CreatedAt) {
		t.Fatalf("editable policy values were reset: historical=%+v migrated=%+v", historical, first)
	}
	if err := authplatform.EnsureSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	second := readAdmin(t, db)
	if second.PolicyVersion != first.PolicyVersion || !second.UpdatedAt.Equal(first.UpdatedAt) {
		t.Fatalf("migration was not idempotent: first=%+v second=%+v", first, second)
	}
}

func TestEnsureSchemaPreservesValidBuiltinAdminDisplayName(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &authplatform.Platform{}); err != nil {
		t.Fatal(err)
	}
	db := connection.GORM.WithContext(ctx)
	historical := validHistoricalAdmin(time.Now().UTC().Truncate(time.Microsecond))
	historical.Name = "Control Center"
	historical.AllowRegister = yesno.No
	if err := db.Create(&historical).Error; err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	admin := readAdmin(t, db)
	if admin.Name != historical.Name || admin.PolicyVersion != historical.PolicyVersion || !admin.UpdatedAt.Equal(historical.UpdatedAt) {
		t.Fatalf("display name was reset: historical=%+v current=%+v", historical, admin)
	}
}

func TestEnsureSchemaRejectsDamagedBuiltinHistory(t *testing.T) {
	tests := []struct {
		name  string
		alter func(*authplatform.Platform)
		rows  int
	}{
		{name: "duplicate rows", rows: 2},
		{name: "soft-deleted history", rows: 1, alter: func(value *authplatform.Platform) {
			value.DeletedAt = gorm.DeletedAt{Time: value.UpdatedAt, Valid: true}
		}},
		{name: "not builtin", rows: 1, alter: func(value *authplatform.Platform) { value.IsBuiltin = yesno.No }},
		{name: "invalid runtime values", rows: 1, alter: func(value *authplatform.Platform) { value.Name = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			connection, ctx := openAuthenticationPlatformDatabase(t)
			if err := database.AutoMigrate(ctx, connection.GORM, &authplatform.Platform{}); err != nil {
				t.Fatal(err)
			}
			db := connection.GORM.WithContext(ctx)
			for index := 0; index < test.rows; index++ {
				value := validHistoricalAdmin(time.Now().UTC().Add(time.Duration(index) * time.Microsecond).Truncate(time.Microsecond))
				if test.alter != nil {
					test.alter(&value)
				}
				if err := db.Create(&value).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := authplatform.EnsureSchema(ctx, db); err == nil {
				t.Fatal("damaged builtin admin history was silently accepted")
			}
		})
	}
}

func TestEnsureSchemaRollsBackBuiltinAdminMigrationFailure(t *testing.T) {
	connection, ctx := openAuthenticationPlatformDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &authplatform.Platform{}); err != nil {
		t.Fatal(err)
	}
	db := connection.GORM.WithContext(ctx)
	historical := validHistoricalAdmin(time.Now().UTC().Truncate(time.Microsecond))
	historical.PolicyVersion = 4
	if err := db.Create(&historical).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`ALTER TABLE auth_platform ADD CONSTRAINT ck_test_admin_registration_migration CHECK (allow_register = 1)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := authplatform.EnsureSchema(ctx, db); err == nil {
		t.Fatal("migration unexpectedly succeeded")
	}
	admin := readAdmin(t, db)
	if admin.AllowRegister != yesno.Yes || admin.PolicyVersion != historical.PolicyVersion || !admin.UpdatedAt.Equal(historical.UpdatedAt) {
		t.Fatalf("failed migration was not rolled back: historical=%+v current=%+v", historical, admin)
	}
}

func validHistoricalAdmin(now time.Time) authplatform.Platform {
	return authplatform.Platform{
		Code: authplatform.BuiltinAdminCode, Name: "Admin", PolicyVersion: 1,
		AccessTTLSeconds: 900, RefreshTTLSeconds: 1_209_600,
		SessionCacheTTLSeconds: 1_800, AccessCacheTTLSeconds: 1_800,
		BindDevice: yesno.No, BindIP: yesno.No, MaxSessions: 1,
		AllowRegister: yesno.Yes, IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes,
		CreatedAt: now, UpdatedAt: now,
	}
}

func readAdmin(t *testing.T, db *gorm.DB) authplatform.Platform {
	t.Helper()
	var value authplatform.Platform
	if err := db.Unscoped().Where("code = ?", authplatform.BuiltinAdminCode).Take(&value).Error; err != nil {
		t.Fatal(err)
	}
	return value
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
	if err := godotenv.Load("../../../../.env"); err != nil && !os.IsNotExist(err) {
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
