package menu_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/database/testschema"
	"admin/server/internal/module/authplatform"
	"admin/server/internal/module/rbac/menu"
	"admin/server/internal/module/rbac/role"
	"github.com/joho/godotenv"
)

type expectedColumn struct {
	dataType string
	nullable string
	length   int64
}

func TestMenuSchema(t *testing.T) {
	connection, ctx := openMenuSchema(t)
	if got := (menu.Menu{}).TableName(); got != "rbac_menu" {
		t.Fatalf("Menu.TableName() = %q", got)
	}
	if got := (menu.RoleMenu{}).TableName(); got != "rbac_role_menu" {
		t.Fatalf("RoleMenu.TableName() = %q", got)
	}

	tables := map[string]map[string]expectedColumn{
		"rbac_menu": {
			"id":             {dataType: "bigint", nullable: "NO"},
			"platform_id":    {dataType: "bigint", nullable: "NO"},
			"parent_id":      {dataType: "bigint", nullable: "YES"},
			"menu_type":      {dataType: "character varying", nullable: "NO", length: 16},
			"name":           {dataType: "character varying", nullable: "NO", length: 128},
			"code":           {dataType: "character varying", nullable: "NO", length: 128},
			"i18n_key":       {dataType: "character varying", nullable: "YES", length: 128},
			"path":           {dataType: "character varying", nullable: "YES", length: 255},
			"component_path": {dataType: "character varying", nullable: "YES", length: 255},
			"icon":           {dataType: "character varying", nullable: "YES", length: 128},
			"sort_order":     {dataType: "integer", nullable: "NO"},
			"is_enabled":     {dataType: "smallint", nullable: "NO"},
			"is_hidden":      {dataType: "smallint", nullable: "NO"},
			"created_at":     {dataType: "timestamp with time zone", nullable: "NO"},
			"updated_at":     {dataType: "timestamp with time zone", nullable: "NO"},
			"deleted_at":     {dataType: "timestamp with time zone", nullable: "YES"},
		},
		"rbac_role_menu": {
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
	assertColumnMissing(t, connection, ctx, "rbac_menu", "view_key")
	assertMenuRelationMissing(t, connection, ctx, "sys_menu")
	assertMenuRelationMissing(t, connection, ctx, "sys_role_menu")

	checks := map[string][]string{
		"ck_rbac_menu_type":       {"CHECK", "menu_type", "directory", "page", "action"},
		"ck_rbac_menu_shape":      {"CHECK", "name", "i18n_key", "component_path", "is_hidden", "action", "icon"},
		"ck_rbac_menu_sort_order": {"CHECK", "sort_order", "0"},
		"ck_rbac_menu_is_enabled": {"CHECK", "is_enabled", "0", "1"},
		"ck_rbac_menu_is_hidden":  {"CHECK", "is_hidden", "0", "1"},
	}
	for name, fragments := range checks {
		definition := constraintDefinition(t, connection, ctx, name)
		for _, fragment := range fragments {
			if !strings.Contains(definition, fragment) {
				t.Errorf("constraint %s = %q, missing %q", name, definition, fragment)
			}
		}
	}

	constraints := map[string][]string{
		"fk_rbac_menu_platform":        {"FOREIGN KEY (platform_id)", "auth_platform(id)", "ON DELETE RESTRICT"},
		"uq_rbac_menu_id_platform":     {"UNIQUE (id, platform_id)"},
		"fk_rbac_menu_parent_platform": {"FOREIGN KEY (parent_id, platform_id)", "rbac_menu(id, platform_id)", "ON DELETE RESTRICT"},
		"fk_rbac_role_menu_role":       {"FOREIGN KEY", "ON DELETE RESTRICT"},
		"fk_rbac_role_menu_menu":       {"FOREIGN KEY", "ON DELETE RESTRICT"},
	}
	for name, fragments := range constraints {
		definition := constraintDefinition(t, connection, ctx, name)
		for _, fragment := range fragments {
			if !strings.Contains(definition, fragment) {
				t.Errorf("constraint %s = %q, missing %q", name, definition, fragment)
			}
		}
	}
	assertConstraintMissing(t, connection, ctx, "fk_rbac_menu_parent")

	indexes := map[string][]string{
		"ux_rbac_menu_code_active":      {"CREATE UNIQUE INDEX", "(platform_id, code)", "WHERE (deleted_at IS NULL)"},
		"ux_rbac_menu_page_path_active": {"CREATE UNIQUE INDEX", "(platform_id, path)", "menu_type", "page", "deleted_at IS NULL"},
		"ix_rbac_menu_parent_active":    {"CREATE INDEX", "(platform_id, parent_id, sort_order, id)", "WHERE (deleted_at IS NULL)"},
		"ux_rbac_role_menu_active":      {"CREATE UNIQUE INDEX", "(role_id, menu_id)", "WHERE (deleted_at IS NULL)"},
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

func assertMenuRelationMissing(t *testing.T, connection *database.Connection, ctx context.Context, name string) {
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

func assertColumnMissing(t *testing.T, connection *database.Connection, ctx context.Context, tableName, columnName string) {
	t.Helper()
	var count int64
	if err := connection.GORM.WithContext(ctx).Raw(`
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
		tableName, columnName).Scan(&count).Error; err != nil {
		t.Fatalf("inspect missing %s.%s: %v", tableName, columnName, err)
	}
	if count != 0 {
		t.Fatalf("column %s.%s still exists", tableName, columnName)
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
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_menu_schema")
	connection := &database.Connection{GORM: db}
	if err := database.AutoMigrate(ctx, db, &authplatform.Platform{}); err != nil {
		t.Fatalf("AutoMigrate authentication platform schema: %v", err)
	}
	if err := authplatform.EnsureSchema(ctx, db); err != nil {
		t.Fatalf("Ensure authentication platform schema: %v", err)
	}
	if err := database.AutoMigrate(ctx, db, &role.Role{}, &menu.Menu{}, &menu.RoleMenu{}); err != nil {
		t.Fatalf("AutoMigrate menu schema: %v", err)
	}
	if err := menu.EnsureSchema(ctx, db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return connection, ctx
}

func assertConstraintMissing(t *testing.T, connection *database.Connection, ctx context.Context, name string) {
	t.Helper()
	var exists bool
	if err := connection.GORM.WithContext(ctx).Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_constraint
			WHERE conname = ? AND connamespace = current_schema()::regnamespace
		)`, name).Scan(&exists).Error; err != nil {
		t.Fatalf("inspect missing constraint %s: %v", name, err)
	}
	if exists {
		t.Fatalf("obsolete constraint %s still exists", name)
	}
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
