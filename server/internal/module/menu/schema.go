package menu

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

var menuConstraints = []constraintDefinition{
	{
		name:  "ck_sys_menu_type",
		table: "sys_menu",
		ddl:   `ALTER TABLE sys_menu ADD CONSTRAINT ck_sys_menu_type CHECK (menu_type IN ('directory', 'page', 'action'))`,
	},
	{
		name:  "ck_sys_menu_shape",
		table: "sys_menu",
		ddl: `ALTER TABLE sys_menu ADD CONSTRAINT ck_sys_menu_shape CHECK (
			(menu_type = 'directory' AND view_key IS NULL)
			OR (menu_type = 'page' AND path IS NOT NULL AND btrim(path) <> '' AND view_key IS NOT NULL AND btrim(view_key) <> '')
			OR (menu_type = 'action' AND path IS NULL AND view_key IS NULL)
		)`,
	},
	{
		name:  "ck_sys_menu_render_shape",
		table: "sys_menu",
		ddl: `ALTER TABLE sys_menu ADD CONSTRAINT ck_sys_menu_render_shape CHECK (
			(menu_type = 'directory' AND path IS NULL AND view_key IS NULL)
			OR (menu_type = 'page' AND path IS NOT NULL AND btrim(path) <> '' AND view_key IS NOT NULL AND btrim(view_key) <> '')
			OR (menu_type = 'action' AND path IS NULL AND view_key IS NULL AND icon IS NULL)
		)`,
	},
	{
		name:  "ck_sys_menu_sort_order",
		table: "sys_menu",
		ddl:   `ALTER TABLE sys_menu ADD CONSTRAINT ck_sys_menu_sort_order CHECK (sort_order >= 0)`,
	},
	{
		name:  "ck_sys_menu_is_enabled",
		table: "sys_menu",
		ddl:   `ALTER TABLE sys_menu ADD CONSTRAINT ck_sys_menu_is_enabled CHECK (is_enabled IN (0, 1))`,
	},
	{
		name:  "fk_sys_menu_parent",
		table: "sys_menu",
		ddl:   `ALTER TABLE sys_menu ADD CONSTRAINT fk_sys_menu_parent FOREIGN KEY (parent_id) REFERENCES sys_menu(id) ON DELETE RESTRICT`,
	},
	{
		name:  "fk_sys_role_menu_role",
		table: "sys_role_menu",
		ddl:   `ALTER TABLE sys_role_menu ADD CONSTRAINT fk_sys_role_menu_role FOREIGN KEY (role_id) REFERENCES sys_role(id) ON DELETE RESTRICT`,
	},
	{
		name:  "fk_sys_role_menu_menu",
		table: "sys_role_menu",
		ddl:   `ALTER TABLE sys_role_menu ADD CONSTRAINT fk_sys_role_menu_menu FOREIGN KEY (menu_id) REFERENCES sys_menu(id) ON DELETE RESTRICT`,
	},
}

var menuIndexes = []string{
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_sys_menu_code_active ON sys_menu (code) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_sys_menu_page_path_active ON sys_menu (path) WHERE deleted_at IS NULL AND menu_type = 'page'`,
	`CREATE INDEX IF NOT EXISTS ix_sys_menu_parent_active ON sys_menu (parent_id, sort_order, id) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_sys_role_menu_active ON sys_role_menu (role_id, menu_id) WHERE deleted_at IS NULL`,
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
	db = db.WithContext(ctx)
	for _, definition := range menuConstraints {
		if err := ensureConstraint(db, definition); err != nil {
			return err
		}
	}
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
