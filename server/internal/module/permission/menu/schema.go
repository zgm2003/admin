package menu

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

var menuConstraints = []constraintDefinition{
	{
		name:  "fk_permission_menu_platform",
		table: "permission_menu",
		ddl:   `ALTER TABLE permission_menu ADD CONSTRAINT fk_permission_menu_platform FOREIGN KEY (platform_id) REFERENCES auth_platform(id) ON DELETE RESTRICT`,
	},
	{
		name:  "uq_permission_menu_id_platform",
		table: "permission_menu",
		ddl:   `ALTER TABLE permission_menu ADD CONSTRAINT uq_permission_menu_id_platform UNIQUE (id, platform_id)`,
	},
	{
		name:  "ck_permission_menu_type",
		table: "permission_menu",
		ddl:   `ALTER TABLE permission_menu ADD CONSTRAINT ck_permission_menu_type CHECK (menu_type IN ('directory', 'page', 'action'))`,
	},
	{
		name:  "ck_permission_menu_shape",
		table: "permission_menu",
		ddl: `ALTER TABLE permission_menu ADD CONSTRAINT ck_permission_menu_shape CHECK (
			btrim(name) <> '' AND (
				(menu_type = 'directory' AND i18n_key IS NOT NULL AND path IS NULL AND component_path IS NULL)
				OR (menu_type = 'page' AND i18n_key IS NOT NULL AND path IS NOT NULL AND btrim(path) <> '' AND component_path IS NOT NULL AND btrim(component_path) <> '')
				OR (menu_type = 'action' AND i18n_key IS NULL AND path IS NULL AND component_path IS NULL AND icon IS NULL AND is_hidden = 1)
			)
		)`,
	},
	{
		name:  "ck_permission_menu_sort_order",
		table: "permission_menu",
		ddl:   `ALTER TABLE permission_menu ADD CONSTRAINT ck_permission_menu_sort_order CHECK (sort_order >= 0)`,
	},
	{
		name:  "ck_permission_menu_is_enabled",
		table: "permission_menu",
		ddl:   `ALTER TABLE permission_menu ADD CONSTRAINT ck_permission_menu_is_enabled CHECK (is_enabled IN (0, 1))`,
	},
	{
		name:  "ck_permission_menu_is_hidden",
		table: "permission_menu",
		ddl:   `ALTER TABLE permission_menu ADD CONSTRAINT ck_permission_menu_is_hidden CHECK (is_hidden IN (0, 1))`,
	},
	{
		name:  "fk_permission_menu_parent_platform",
		table: "permission_menu",
		ddl:   `ALTER TABLE permission_menu ADD CONSTRAINT fk_permission_menu_parent_platform FOREIGN KEY (parent_id, platform_id) REFERENCES permission_menu(id, platform_id) ON DELETE RESTRICT`,
	},
	{
		name:  "fk_permission_role_menu_role",
		table: "permission_role_menu",
		ddl:   `ALTER TABLE permission_role_menu ADD CONSTRAINT fk_permission_role_menu_role FOREIGN KEY (role_id) REFERENCES permission_role(id) ON DELETE RESTRICT`,
	},
	{
		name:  "fk_permission_role_menu_menu",
		table: "permission_role_menu",
		ddl:   `ALTER TABLE permission_role_menu ADD CONSTRAINT fk_permission_role_menu_menu FOREIGN KEY (menu_id) REFERENCES permission_menu(id) ON DELETE RESTRICT`,
	},
}

var menuIndexes = []indexDefinition{
	{name: "ux_permission_menu_code_active", ddl: `CREATE UNIQUE INDEX ux_permission_menu_code_active ON permission_menu (platform_id, code) WHERE deleted_at IS NULL`, fragments: []string{"(platform_id, code)", "WHERE (deleted_at IS NULL)"}},
	{name: "ux_permission_menu_page_path_active", ddl: `CREATE UNIQUE INDEX ux_permission_menu_page_path_active ON permission_menu (platform_id, path) WHERE deleted_at IS NULL AND menu_type = 'page'`, fragments: []string{"(platform_id, path)", "menu_type", "page", "deleted_at IS NULL"}},
	{name: "ix_permission_menu_parent_active", ddl: `CREATE INDEX ix_permission_menu_parent_active ON permission_menu (platform_id, parent_id, sort_order, id) WHERE deleted_at IS NULL`, fragments: []string{"(platform_id, parent_id, sort_order, id)", "WHERE (deleted_at IS NULL)"}},
	{name: "ux_permission_role_menu_active", ddl: `CREATE UNIQUE INDEX ux_permission_role_menu_active ON permission_role_menu (role_id, menu_id) WHERE deleted_at IS NULL`, fragments: []string{"(role_id, menu_id)", "WHERE (deleted_at IS NULL)"}},
}

type indexDefinition struct {
	name      string
	ddl       string
	fragments []string
}

type constraintDefinition struct {
	name  string
	table string
	ddl   string
}

func EnsureSchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("ensure menu schema requires a database")
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`ALTER TABLE permission_menu ADD COLUMN IF NOT EXISTS remark VARCHAR(512)`).Error; err != nil {
			return fmt.Errorf("ensure menu remark column: %w", err)
		}
		if err := replaceMenuConstraints(tx); err != nil {
			return err
		}
		return ensureMenuIndexes(tx)
	})
}

var legacyComponentPaths = map[string]string{
	"system-menus":          "system/menus",
	"system-roles":          "system/roles",
	"system-users":          "system/users",
	"system-auth-platforms": "system/auth-platforms",
	"system-sessions":       "system/sessions",
	"system-operation-logs": "system/operation-logs",
}

func replaceMenuConstraints(db *gorm.DB) error {
	for _, name := range []string{"ck_permission_menu_shape", "ck_permission_menu_render_shape", "ck_permission_menu_is_hidden", "fk_permission_menu_parent"} {
		if err := db.Exec(`ALTER TABLE permission_menu DROP CONSTRAINT IF EXISTS ` + name).Error; err != nil {
			return fmt.Errorf("drop menu constraint %s: %w", name, err)
		}
	}
	for _, definition := range menuConstraints {
		if err := ensureConstraint(db, definition); err != nil {
			return err
		}
	}
	return nil
}

func ensureMenuIndexes(db *gorm.DB) error {
	for _, definition := range menuIndexes {
		var current string
		if err := db.Raw(`SELECT indexdef FROM pg_indexes WHERE schemaname = current_schema() AND indexname = ?`, definition.name).Scan(&current).Error; err != nil {
			return fmt.Errorf("inspect menu index %s: %w", definition.name, err)
		}
		matches := current != ""
		for _, fragment := range definition.fragments {
			matches = matches && strings.Contains(current, fragment)
		}
		if matches {
			continue
		}
		if current != "" {
			if err := db.Exec(`DROP INDEX ` + definition.name).Error; err != nil {
				return fmt.Errorf("replace menu index %s: %w", definition.name, err)
			}
		}
		if err := db.Exec(definition.ddl).Error; err != nil {
			return fmt.Errorf("create menu index: %w", err)
		}
	}
	return nil
}

func ensureConstraint(db *gorm.DB, definition constraintDefinition) error {
	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM pg_constraint
			WHERE conname = ?
			  AND conrelid = to_regclass(current_schema() || '.' || ?)
		)`, definition.name, definition.table).Scan(&exists).Error; err != nil {
		return fmt.Errorf("inspect menu constraint %s: %w", definition.name, err)
	}
	if exists {
		return nil
	}
	if err := db.Exec(definition.ddl).Error; err != nil {
		return fmt.Errorf("create menu constraint %s: %w", definition.name, err)
	}
	return nil
}
