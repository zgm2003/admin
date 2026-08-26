package menu

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

var menuConstraints = []constraintDefinition{
	{
		name:  "ck_rbac_menu_type",
		table: "rbac_menu",
		ddl:   `ALTER TABLE rbac_menu ADD CONSTRAINT ck_rbac_menu_type CHECK (menu_type IN ('directory', 'page', 'action'))`,
	},
	{
		name:  "ck_rbac_menu_shape",
		table: "rbac_menu",
		ddl: `ALTER TABLE rbac_menu ADD CONSTRAINT ck_rbac_menu_shape CHECK (
			btrim(name) <> '' AND (
				(menu_type = 'directory' AND i18n_key IS NOT NULL AND path IS NULL AND component_path IS NULL)
				OR (menu_type = 'page' AND i18n_key IS NOT NULL AND path IS NOT NULL AND btrim(path) <> '' AND component_path IS NOT NULL AND btrim(component_path) <> '')
				OR (menu_type = 'action' AND i18n_key IS NULL AND path IS NULL AND component_path IS NULL AND icon IS NULL AND is_hidden = 1)
			)
		)`,
	},
	{
		name:  "ck_rbac_menu_sort_order",
		table: "rbac_menu",
		ddl:   `ALTER TABLE rbac_menu ADD CONSTRAINT ck_rbac_menu_sort_order CHECK (sort_order >= 0)`,
	},
	{
		name:  "ck_rbac_menu_is_enabled",
		table: "rbac_menu",
		ddl:   `ALTER TABLE rbac_menu ADD CONSTRAINT ck_rbac_menu_is_enabled CHECK (is_enabled IN (0, 1))`,
	},
	{
		name:  "ck_rbac_menu_is_hidden",
		table: "rbac_menu",
		ddl:   `ALTER TABLE rbac_menu ADD CONSTRAINT ck_rbac_menu_is_hidden CHECK (is_hidden IN (0, 1))`,
	},
	{
		name:  "fk_rbac_menu_parent",
		table: "rbac_menu",
		ddl:   `ALTER TABLE rbac_menu ADD CONSTRAINT fk_rbac_menu_parent FOREIGN KEY (parent_id) REFERENCES rbac_menu(id) ON DELETE RESTRICT`,
	},
	{
		name:  "fk_rbac_role_menu_role",
		table: "rbac_role_menu",
		ddl:   `ALTER TABLE rbac_role_menu ADD CONSTRAINT fk_rbac_role_menu_role FOREIGN KEY (role_id) REFERENCES rbac_role(id) ON DELETE RESTRICT`,
	},
	{
		name:  "fk_rbac_role_menu_menu",
		table: "rbac_role_menu",
		ddl:   `ALTER TABLE rbac_role_menu ADD CONSTRAINT fk_rbac_role_menu_menu FOREIGN KEY (menu_id) REFERENCES rbac_menu(id) ON DELETE RESTRICT`,
	},
}

var menuIndexes = []string{
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_rbac_menu_code_active ON rbac_menu (code) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_rbac_menu_page_path_active ON rbac_menu (path) WHERE deleted_at IS NULL AND menu_type = 'page'`,
	`CREATE INDEX IF NOT EXISTS ix_rbac_menu_parent_active ON rbac_menu (parent_id, sort_order, id) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_rbac_role_menu_active ON rbac_role_menu (role_id, menu_id) WHERE deleted_at IS NULL`,
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
	for _, name := range []string{"ck_rbac_menu_shape", "ck_rbac_menu_render_shape", "ck_rbac_menu_is_hidden"} {
		if err := db.Exec(`ALTER TABLE rbac_menu DROP CONSTRAINT IF EXISTS ` + name).Error; err != nil {
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
	for _, statement := range menuIndexes {
		if err := db.Exec(statement).Error; err != nil {
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
