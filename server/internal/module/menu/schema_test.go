package menu_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/menu"
	"admin/server/internal/module/role"
	"github.com/joho/godotenv"
)

type expectedColumn struct {
	dataType string
	nullable string
	length   int64
}

func TestMenuSchema(t *testing.T) {
	connection, ctx := openMenuSchema(t)

	tables := map[string]map[string]expectedColumn{
		"sys_menu": {
			"id":         {dataType: "bigint", nullable: "NO"},
			"parent_id":  {dataType: "bigint", nullable: "YES"},
			"menu_type":  {dataType: "character varying", nullable: "NO", length: 16},
			"code":       {dataType: "character varying", nullable: "NO", length: 128},
			"i18n_key":   {dataType: "character varying", nullable: "NO", length: 128},
			"path":       {dataType: "character varying", nullable: "YES", length: 255},
			"view_key":   {dataType: "character varying", nullable: "YES", length: 128},
			"icon":       {dataType: "character varying", nullable: "YES", length: 64},
			"sort_order": {dataType: "integer", nullable: "NO"},
			"is_enabled": {dataType: "smallint", nullable: "NO"},
			"created_at": {dataType: "timestamp with time zone", nullable: "NO"},
			"updated_at": {dataType: "timestamp with time zone", nullable: "NO"},
			"deleted_at": {dataType: "timestamp with time zone", nullable: "YES"},
		},
		"sys_role_menu": {
			"id":         {dataType: "bigint", nullable: "NO"},
			"role_id":    {dataType: "bigint", nullable: "NO"},
			"menu_id":    {dataType: "bigint", nullable: "NO"},
			"created_at": {dataType: "timestamp with time zone", nullable: "NO"},
			"updated_at": {dataType: "timestamp with time zone", nullable: "NO"},
			"deleted_at": {dataType: "timestamp with time zone", nullable: "YES"},
		},
	}

	for tableName, columns := range tables {
		for columnName, want := range columns {
			assertColumn(t, connection, ctx, tableName, columnName, want)
		}
	}

	checks := map[string][]string{
		"ck_sys_menu_type":       {"CHECK", "menu_type", "directory", "page", "action"},
		"ck_sys_menu_shape":      {"CHECK", "menu_type", "path", "view_key"},
		"ck_sys_menu_is_enabled": {"CHECK", "is_enabled", "0", "1"},
	}
	for name, fragments := range checks {
		definition := constraintDefinition(t, connection, ctx, name)
		for _, fragment := range fragments {
			if !strings.Contains(definition, fragment) {
				t.Errorf("constraint %s = %q, missing %q", name, definition, fragment)
			}
		}
	}

	for _, name := range []string{
		"fk_sys_menu_parent",
		"fk_sys_role_menu_role",
		"fk_sys_role_menu_menu",
	} {
		definition := constraintDefinition(t, connection, ctx, name)
		if !strings.Contains(definition, "FOREIGN KEY") || !strings.Contains(definition, "ON DELETE RESTRICT") {
			t.Errorf("constraint %s = %q", name, definition)
		}
	}

	indexes := map[string][]string{
		"ux_sys_menu_code_active":   {"CREATE UNIQUE INDEX", "(code)", "WHERE (deleted_at IS NULL)"},
		"ix_sys_menu_parent_active": {"CREATE INDEX", "(parent_id, sort_order, id)", "WHERE (deleted_at IS NULL)"},
		"ux_sys_role_menu_active":   {"CREATE UNIQUE INDEX", "(role_id, menu_id)", "WHERE (deleted_at IS NULL)"},
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

func openMenuSchema(t *testing.T) (*database.Connection, context.Context) {
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
		&role.Role{}, &menu.Menu{}, &menu.RoleMenu{}); err != nil {
		t.Fatalf("AutoMigrate menu schema: %v", err)
	}
	if err := menu.EnsureSchema(ctx, connection.GORM); err != nil {
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
