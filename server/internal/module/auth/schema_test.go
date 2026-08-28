package auth_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/access"
	"admin/server/internal/module/auth"
	"admin/server/internal/module/authplatform"
	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type expectedColumn struct {
	dataType string
	nullable string
	length   int64
}

func TestAuthenticationSchema(t *testing.T) {
	connection, ctx := openAuthenticationSchema(t)
	if got := (user.User{}).TableName(); got != "user_account" {
		t.Fatalf("User.TableName() = %q", got)
	}
	if got := (auth.Session{}).TableName(); got != "auth_session" {
		t.Fatalf("Session.TableName() = %q", got)
	}

	tables := map[string]map[string]expectedColumn{
		"user_account": {
			"id":            {dataType: "bigint", nullable: "NO"},
			"username":      {dataType: "character varying", nullable: "NO", length: 64},
			"email":         {dataType: "character varying", nullable: "NO", length: 254},
			"phone":         {dataType: "character varying", nullable: "YES", length: 32},
			"password_hash": {dataType: "character varying", nullable: "NO", length: 255},
			"is_enabled":    {dataType: "smallint", nullable: "NO"},
			"created_at":    {dataType: "timestamp with time zone", nullable: "NO"},
			"updated_at":    {dataType: "timestamp with time zone", nullable: "NO"},
			"deleted_at":    {dataType: "timestamp with time zone", nullable: "YES"},
		},
		"auth_session": {
			"id":                 {dataType: "bigint", nullable: "NO"},
			"user_id":            {dataType: "bigint", nullable: "NO"},
			"platform":           {dataType: "character varying", nullable: "NO", length: 49},
			"device_id":          {dataType: "character varying", nullable: "NO", length: 36},
			"refresh_token_hash": {dataType: "character", nullable: "NO", length: 64},
			"version":            {dataType: "bigint", nullable: "NO"},
			"client_ip":          {dataType: "character varying", nullable: "NO", length: 64},
			"user_agent":         {dataType: "character varying", nullable: "NO", length: 512},
			"refresh_expires_at": {dataType: "timestamp with time zone", nullable: "NO"},
			"revoked_at":         {dataType: "timestamp with time zone", nullable: "YES"},
			"created_at":         {dataType: "timestamp with time zone", nullable: "NO"},
			"updated_at":         {dataType: "timestamp with time zone", nullable: "NO"},
		},
	}

	for tableName, columns := range tables {
		for columnName, want := range columns {
			assertColumn(t, connection, ctx, tableName, columnName, want)
		}
	}
	assertRelationMissing(t, connection, ctx, "sys_user")
	assertRelationMissing(t, connection, ctx, "sys_user_session")

	checks := map[string]string{
		"ck_user_account_is_enabled": "is_enabled",
		"ck_auth_session_version":    "version",
	}
	for name, expression := range checks {
		definition := constraintDefinition(t, connection, ctx, name)
		if !strings.Contains(definition, expression) || !strings.Contains(definition, "CHECK") {
			t.Errorf("constraint %s = %q", name, definition)
		}
	}

	for _, name := range []string{"fk_auth_session_user"} {
		definition := constraintDefinition(t, connection, ctx, name)
		if !strings.Contains(definition, "FOREIGN KEY") || !strings.Contains(definition, "ON DELETE RESTRICT") {
			t.Errorf("constraint %s = %q", name, definition)
		}
	}

	indexes := map[string][]string{
		"ux_user_account_username_active":      {"CREATE UNIQUE INDEX", "lower((username)::text)", "WHERE (deleted_at IS NULL)"},
		"ux_user_account_email_active":         {"CREATE UNIQUE INDEX", "(email)", "WHERE (deleted_at IS NULL)"},
		"ux_user_account_phone_active":         {"CREATE UNIQUE INDEX", "(phone)", "phone IS NOT NULL", "deleted_at IS NULL"},
		"ux_auth_session_refresh_hash":         {"CREATE UNIQUE INDEX", "(refresh_token_hash)"},
		"ix_auth_session_user_platform_active": {"CREATE INDEX", "(user_id, platform, created_at DESC, id DESC)", "WHERE (revoked_at IS NULL)"},
	}
	for name, fragments := range indexes {
		definition := indexDefinition(t, connection, ctx, name)
		for _, fragment := range fragments {
			if !strings.Contains(definition, fragment) {
				t.Errorf("index %s = %q, missing %q", name, definition, fragment)
			}
		}
	}
	if definition := optionalIndexDefinition(t, connection, ctx, "ux_auth_session_current"); definition != "" {
		t.Fatalf("legacy current-session index still exists: %s", definition)
	}
}

func TestAuthenticationSchemaCreatesNullableUniquePhone(t *testing.T) {
	connection, ctx := openAuthenticationSchema(t)
	var column struct {
		DataType               string
		CharacterMaximumLength int
		IsNullable             string
	}
	if err := connection.GORM.WithContext(ctx).Raw(`
		SELECT data_type, character_maximum_length, is_nullable
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = 'user_account'
		  AND column_name = 'phone'`).Scan(&column).Error; err != nil {
		t.Fatal(err)
	}
	if column.DataType != "character varying" || column.CharacterMaximumLength != 32 || column.IsNullable != "YES" {
		t.Fatalf("phone column = %+v", column)
	}
	definition := indexDefinition(t, connection, ctx, "ux_user_account_phone_active")
	for _, fragment := range []string{"CREATE UNIQUE INDEX", "(phone)", "phone IS NOT NULL", "deleted_at IS NULL"} {
		if !strings.Contains(definition, fragment) {
			t.Errorf("phone index = %q, missing %q", definition, fragment)
		}
	}
}

func assertRelationMissing(t *testing.T, connection *database.Connection, ctx context.Context, name string) {
	t.Helper()
	var exists bool
	if err := connection.GORM.WithContext(ctx).Raw(
		`SELECT to_regclass(current_schema() || '.' || ?) IS NOT NULL`, name,
	).Scan(&exists).Error; err != nil {
		t.Fatalf("inspect relation %s: %v", name, err)
	}
	if exists {
		t.Fatalf("legacy relation %s still exists", name)
	}
}

func TestPrepareSessionSchemaRevokesOnlyLegacySessions(t *testing.T) {
	connection, ctx := openIsolatedAuthenticationDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &user.User{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	createdUser := user.User{Username: fmt.Sprintf("migration-%d", now.UnixNano()), Email: fmt.Sprintf("migration-%d@example.com", now.UnixNano()), PasswordHash: "hash", IsEnabled: 1, CreatedAt: now, UpdatedAt: now}
	if err := connection.GORM.WithContext(ctx).Create(&createdUser).Error; err != nil {
		t.Fatal(err)
	}
	if err := connection.GORM.WithContext(ctx).Exec(`
		CREATE TABLE auth_session (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			refresh_token_hash CHAR(64) NOT NULL,
			version BIGINT NOT NULL DEFAULT 1,
			client_ip VARCHAR(64) NOT NULL,
			user_agent VARCHAR(512) NOT NULL,
			refresh_expires_at TIMESTAMPTZ NOT NULL,
			revoked_at TIMESTAMPTZ NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX ux_auth_session_current
		ON auth_session (user_id) WHERE revoked_at IS NULL;`).Error; err != nil {
		t.Fatal(err)
	}
	var legacySessionID int64
	if err := connection.GORM.WithContext(ctx).Raw(`
		INSERT INTO auth_session
			(user_id, refresh_token_hash, version, client_ip, user_agent, refresh_expires_at, created_at, updated_at)
		VALUES (?, ?, 1, '127.0.0.1', 'legacy', ?, ?, ?)
		RETURNING id`, createdUser.ID, strings.Repeat("a", 64), now.Add(time.Hour), now, now).Scan(&legacySessionID).Error; err != nil {
		t.Fatal(err)
	}

	if err := auth.PrepareSessionSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(ctx, connection.GORM, &auth.Session{}); err != nil {
		t.Fatal(err)
	}
	if err := auth.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	var legacy auth.Session
	if err := connection.GORM.WithContext(ctx).Take(&legacy, legacySessionID).Error; err != nil {
		t.Fatal(err)
	}
	if legacy.Platform != "admin" || legacy.DeviceID != "" || legacy.RevokedAt == nil {
		t.Fatalf("migrated legacy session = %+v", legacy)
	}
	if definition := optionalIndexDefinition(t, connection, ctx, "ux_auth_session_current"); definition != "" {
		t.Fatalf("legacy current-session index still exists: %s", definition)
	}
	if definition := optionalIndexDefinition(t, connection, ctx, "ix_auth_session_user_platform_active"); definition == "" {
		t.Fatal("platform session index does not exist")
	}

	postMigration := auth.Session{
		UserID: createdUser.ID, Platform: "admin", DeviceID: "550e8400-e29b-41d4-a716-446655440000",
		RefreshTokenHash: strings.Repeat("b", 64), Version: 1, ClientIP: "127.0.0.1", UserAgent: "current",
		RefreshExpiresAt: now.Add(time.Hour), CreatedAt: now.Add(time.Minute), UpdatedAt: now.Add(time.Minute),
	}
	if err := connection.GORM.WithContext(ctx).Create(&postMigration).Error; err != nil {
		t.Fatal(err)
	}
	if err := auth.PrepareSessionSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	var stored auth.Session
	if err := connection.GORM.WithContext(ctx).Take(&stored, postMigration.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.RevokedAt != nil {
		t.Fatalf("post-migration session was revoked at %v", stored.RevokedAt)
	}
}

func TestAuthenticationSchemaSourceDoesNotOwnRoleObjects(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("schema.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"ux_rbac_role_", "ux_rbac_user_role_", "fk_rbac_user_role_"} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("auth/schema.go still owns %q", forbidden)
		}
	}
}

func openAuthenticationSchema(t *testing.T) (*database.Connection, context.Context) {
	t.Helper()
	connection, ctx := openIsolatedAuthenticationDatabase(t)
	if err := auth.PrepareSessionSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("PrepareSessionSchema: %v", err)
	}
	if err := database.AutoMigrate(ctx, connection.GORM,
		&user.User{}, &role.Role{}, &role.UserRole{}, &authplatform.Platform{}, &auth.Session{}, &access.Version{}); err != nil {
		t.Fatalf("AutoMigrate auth schema: %v", err)
	}
	if err := role.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("Ensure role schema: %v", err)
	}
	if err := authplatform.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("Ensure authentication platform schema: %v", err)
	}
	if err := auth.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	if err := access.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("Ensure access schema: %v", err)
	}
	return connection, ctx
}

func openIsolatedAuthenticationDatabase(t *testing.T) (*database.Connection, context.Context) {
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
	root, err := database.Open(ctx, settings.PostgresDSN)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	schema := fmt.Sprintf("test_auth_%d", time.Now().UnixNano())
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
	})
	return connection, ctx
}

func assertColumn(t *testing.T, connection *database.Connection, ctx context.Context, tableName, columnName string, want expectedColumn) {
	t.Helper()
	var got struct {
		DataType      string  `gorm:"column:data_type"`
		IsNullable    string  `gorm:"column:is_nullable"`
		MaximumLength *int64  `gorm:"column:character_maximum_length"`
		ColumnDefault *string `gorm:"column:column_default"`
	}
	if err := connection.GORM.WithContext(ctx).Raw(`
		SELECT data_type, is_nullable, character_maximum_length, column_default
		FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
		tableName, columnName).Scan(&got).Error; err != nil {
		t.Fatalf("inspect %s.%s: %v", tableName, columnName, err)
	}
	if got.DataType != want.dataType || got.IsNullable != want.nullable {
		t.Errorf("%s.%s = type %q nullable %q, want %q %q", tableName, columnName, got.DataType, got.IsNullable, want.dataType, want.nullable)
	}
	if want.length > 0 && (got.MaximumLength == nil || *got.MaximumLength != want.length) {
		t.Errorf("%s.%s length = %v, want %d", tableName, columnName, got.MaximumLength, want.length)
	}
	if (columnName == "created_at" || columnName == "updated_at") && (got.ColumnDefault == nil || *got.ColumnDefault == "") {
		t.Errorf("%s.%s lacks a default", tableName, columnName)
	}
}

func constraintDefinition(t *testing.T, connection *database.Connection, ctx context.Context, name string) string {
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

func indexDefinition(t *testing.T, connection *database.Connection, ctx context.Context, name string) string {
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

func optionalIndexDefinition(t *testing.T, connection *database.Connection, ctx context.Context, name string) string {
	t.Helper()
	var definition string
	if err := connection.GORM.WithContext(ctx).Raw(`
		SELECT indexdef FROM pg_indexes
		WHERE schemaname = current_schema() AND indexname = ?`, name).Scan(&definition).Error; err != nil {
		t.Fatalf("inspect index %s: %v", name, err)
	}
	return definition
}
