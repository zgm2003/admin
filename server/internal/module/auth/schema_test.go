package auth_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/auth"
	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	"github.com/joho/godotenv"
)

type expectedColumn struct {
	dataType string
	nullable string
	length   int64
}

func TestAuthenticationSchema(t *testing.T) {
	connection, ctx := openAuthenticationSchema(t)

	tables := map[string]map[string]expectedColumn{
		"sys_user": {
			"id":            {dataType: "bigint", nullable: "NO"},
			"username":      {dataType: "character varying", nullable: "NO", length: 64},
			"email":         {dataType: "character varying", nullable: "NO", length: 254},
			"password_hash": {dataType: "character varying", nullable: "NO", length: 255},
			"is_enabled":    {dataType: "smallint", nullable: "NO"},
			"created_at":    {dataType: "timestamp with time zone", nullable: "NO"},
			"updated_at":    {dataType: "timestamp with time zone", nullable: "NO"},
			"deleted_at":    {dataType: "timestamp with time zone", nullable: "YES"},
		},
		"sys_role": {
			"id":         {dataType: "bigint", nullable: "NO"},
			"code":       {dataType: "character varying", nullable: "NO", length: 64},
			"name":       {dataType: "character varying", nullable: "NO", length: 64},
			"is_default": {dataType: "smallint", nullable: "NO"},
			"is_enabled": {dataType: "smallint", nullable: "NO"},
			"created_at": {dataType: "timestamp with time zone", nullable: "NO"},
			"updated_at": {dataType: "timestamp with time zone", nullable: "NO"},
			"deleted_at": {dataType: "timestamp with time zone", nullable: "YES"},
		},
		"sys_user_role": {
			"id":         {dataType: "bigint", nullable: "NO"},
			"user_id":    {dataType: "bigint", nullable: "NO"},
			"role_id":    {dataType: "bigint", nullable: "NO"},
			"created_at": {dataType: "timestamp with time zone", nullable: "NO"},
			"updated_at": {dataType: "timestamp with time zone", nullable: "NO"},
			"deleted_at": {dataType: "timestamp with time zone", nullable: "YES"},
		},
		"sys_user_session": {
			"id":                 {dataType: "bigint", nullable: "NO"},
			"user_id":            {dataType: "bigint", nullable: "NO"},
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

	checks := map[string]string{
		"ck_sys_user_is_enabled":      "is_enabled",
		"ck_sys_role_is_default":      "is_default",
		"ck_sys_role_is_enabled":      "is_enabled",
		"ck_sys_user_session_version": "version",
	}
	for name, expression := range checks {
		definition := constraintDefinition(t, connection, ctx, name)
		if !strings.Contains(definition, expression) || !strings.Contains(definition, "CHECK") {
			t.Errorf("constraint %s = %q", name, definition)
		}
	}

	for _, name := range []string{
		"fk_sys_user_role_user",
		"fk_sys_user_role_role",
		"fk_sys_user_session_user",
	} {
		definition := constraintDefinition(t, connection, ctx, name)
		if !strings.Contains(definition, "FOREIGN KEY") || !strings.Contains(definition, "ON DELETE RESTRICT") {
			t.Errorf("constraint %s = %q", name, definition)
		}
	}

	indexes := map[string][]string{
		"ux_sys_user_username_active":      {"CREATE UNIQUE INDEX", "lower((username)::text)", "WHERE (deleted_at IS NULL)"},
		"ux_sys_user_email_active":         {"CREATE UNIQUE INDEX", "(email)", "WHERE (deleted_at IS NULL)"},
		"ux_sys_role_code_active":          {"CREATE UNIQUE INDEX", "(code)", "WHERE (deleted_at IS NULL)"},
		"ux_sys_role_default_active":       {"CREATE UNIQUE INDEX", "(is_default)", "is_default = 1", "deleted_at IS NULL"},
		"ux_sys_user_role_active":          {"CREATE UNIQUE INDEX", "(user_id, role_id)", "WHERE (deleted_at IS NULL)"},
		"ux_sys_user_session_refresh_hash": {"CREATE UNIQUE INDEX", "(refresh_token_hash)"},
		"ux_sys_user_session_current":      {"CREATE UNIQUE INDEX", "(user_id)", "WHERE (revoked_at IS NULL)"},
		"ix_sys_user_session_user_created": {"CREATE INDEX", "(user_id, created_at DESC)"},
	}
	for name, fragments := range indexes {
		definition := indexDefinition(t, connection, ctx, name)
		for _, fragment := range fragments {
			if !strings.Contains(definition, fragment) {
				t.Errorf("index %s = %q, missing %q", name, definition, fragment)
			}
		}
	}
}

func openAuthenticationSchema(t *testing.T) (*database.Connection, context.Context) {
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
	if err := database.AutoMigrate(ctx, connection.GORM,
		&user.User{}, &role.Role{}, &role.UserRole{}, &auth.Session{}); err != nil {
		t.Fatalf("AutoMigrate auth schema: %v", err)
	}
	if err := auth.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
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
