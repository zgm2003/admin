package menu

import (
	"context"
	"fmt"
	"time"

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
			(menu_type = 'directory' AND path IS NULL AND component_path IS NULL)
			OR (menu_type = 'page' AND path IS NOT NULL AND btrim(path) <> '' AND component_path IS NOT NULL AND btrim(component_path) <> '')
			OR (menu_type = 'action' AND path IS NULL AND component_path IS NULL AND icon IS NULL AND is_hidden = 1)
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
		name:  "ck_sys_menu_is_hidden",
		table: "sys_menu",
		ddl:   `ALTER TABLE sys_menu ADD CONSTRAINT ck_sys_menu_is_hidden CHECK (is_hidden IN (0, 1))`,
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
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := migrateMenuProtocol(tx); err != nil {
			return err
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

func migrateMenuProtocol(db *gorm.DB) error {
	componentPathExists, err := menuColumnExists(db, "component_path")
	if err != nil {
		return err
	}
	isHiddenExists, err := menuColumnExists(db, "is_hidden")
	if err != nil {
		return err
	}
	if !componentPathExists || !isHiddenExists {
		return fmt.Errorf("migrate menu protocol requires component_path and is_hidden columns")
	}

	legacyColumnExists, err := menuColumnExists(db, "view_key")
	if err != nil {
		return err
	}
	if legacyColumnExists {
		type legacyPage struct {
			ID          int64
			Code        string
			LegacyValue *string
		}
		var pages []legacyPage
		if err := db.Raw(`
			SELECT id, code, view_key AS legacy_value
			FROM sys_menu
			WHERE menu_type = 'page' AND component_path IS NULL
			ORDER BY id`).Scan(&pages).Error; err != nil {
			return fmt.Errorf("find legacy menu pages: %w", err)
		}
		for _, page := range pages {
			if page.LegacyValue == nil {
				return fmt.Errorf("migrate menu %s: view_key is null", page.Code)
			}
			componentPath, exists := legacyComponentPaths[*page.LegacyValue]
			if !exists {
				return fmt.Errorf("migrate menu %s: view_key %q has no component path mapping", page.Code, *page.LegacyValue)
			}
			result := db.Exec(`
				UPDATE sys_menu
				SET component_path = ?, updated_at = CURRENT_TIMESTAMP
				WHERE id = ? AND component_path IS NULL`, componentPath, page.ID)
			if result.Error != nil {
				return fmt.Errorf("migrate menu %s component path: %w", page.Code, result.Error)
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("migrate menu %s component path: expected one row, got %d", page.Code, result.RowsAffected)
			}
		}

		if err := retireLegacyMenuManagementNode(db); err != nil {
			return err
		}
		for _, name := range []string{"ck_sys_menu_shape", "ck_sys_menu_render_shape"} {
			if err := db.Exec(`ALTER TABLE sys_menu DROP CONSTRAINT IF EXISTS ` + name).Error; err != nil {
				return fmt.Errorf("drop legacy menu constraint %s: %w", name, err)
			}
		}
		if err := db.Exec(`ALTER TABLE sys_menu DROP COLUMN view_key`).Error; err != nil {
			return fmt.Errorf("drop legacy menu view_key: %w", err)
		}
		if err := db.Exec(`UPDATE sys_menu SET is_hidden = CASE WHEN menu_type = 'action' THEN 1 ELSE 0 END`).Error; err != nil {
			return fmt.Errorf("backfill menu hidden state: %w", err)
		}
	}
	if err := db.Exec(`ALTER TABLE sys_menu ALTER COLUMN component_path TYPE VARCHAR(255)`).Error; err != nil {
		return fmt.Errorf("set menu component path type: %w", err)
	}
	if err := db.Exec(`ALTER TABLE sys_menu ALTER COLUMN icon TYPE VARCHAR(128)`).Error; err != nil {
		return fmt.Errorf("set menu icon type: %w", err)
	}
	return nil
}

func retireLegacyMenuManagementNode(db *gorm.DB) error {
	var menuIDs []int64
	if err := db.Raw(`
		WITH RECURSIVE retired AS (
			SELECT id FROM sys_menu WHERE code = ? AND deleted_at IS NULL
			UNION ALL
			SELECT child.id
			FROM sys_menu AS child
			JOIN retired AS parent ON child.parent_id = parent.id
			WHERE child.deleted_at IS NULL
		)
		SELECT id FROM retired ORDER BY id`, PermissionList).Scan(&menuIDs).Error; err != nil {
		return fmt.Errorf("find legacy menu management subtree: %w", err)
	}
	if len(menuIDs) == 0 {
		return nil
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := db.Exec(`
		UPDATE sys_role_menu
		SET updated_at = ?, deleted_at = ?
		WHERE menu_id IN ? AND deleted_at IS NULL`, now, now, menuIDs).Error; err != nil {
		return fmt.Errorf("retire legacy menu management grants: %w", err)
	}
	if err := db.Exec(`
		UPDATE sys_menu
		SET updated_at = ?, deleted_at = ?
		WHERE id IN ? AND deleted_at IS NULL`, now, now, menuIDs).Error; err != nil {
		return fmt.Errorf("retire legacy menu management nodes: %w", err)
	}
	return nil
}

func menuColumnExists(db *gorm.DB, columnName string) (bool, error) {
	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = 'sys_menu'
			  AND column_name = ?
		)`, columnName).Scan(&exists).Error; err != nil {
		return false, fmt.Errorf("inspect menu column %s: %w", columnName, err)
	}
	return exists, nil
}

func replaceMenuConstraints(db *gorm.DB) error {
	for _, name := range []string{"ck_sys_menu_shape", "ck_sys_menu_render_shape", "ck_sys_menu_is_hidden"} {
		if err := db.Exec(`ALTER TABLE sys_menu DROP CONSTRAINT IF EXISTS ` + name).Error; err != nil {
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
