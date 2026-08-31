package role

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

var roleIndexes = []string{
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_permission_role_code_active ON permission_role (code) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_permission_role_name_active ON permission_role (name) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_permission_role_default_active ON permission_role (is_default) WHERE is_default = 1 AND deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_permission_user_role_active ON permission_user_role (user_id, role_id) WHERE deleted_at IS NULL`,
}

var roleForeignKeys = []roleForeignKeyDefinition{
	{
		name:  "fk_permission_user_role_user",
		table: "permission_user_role",
		ddl:   `ALTER TABLE permission_user_role ADD CONSTRAINT fk_permission_user_role_user FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE RESTRICT`,
	},
	{
		name:  "fk_permission_user_role_role",
		table: "permission_user_role",
		ddl:   `ALTER TABLE permission_user_role ADD CONSTRAINT fk_permission_user_role_role FOREIGN KEY (role_id) REFERENCES permission_role(id) ON DELETE RESTRICT`,
	},
}

type roleForeignKeyDefinition struct {
	name  string
	table string
	ddl   string
}

func EnsureSchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("ensure role schema requires a database")
	}
	db = db.WithContext(ctx)
	for _, statement := range roleIndexes {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("create role index: %w", err)
		}
	}
	for _, definition := range roleForeignKeys {
		if err := ensureRoleForeignKey(db, definition); err != nil {
			return err
		}
	}
	return nil
}

func ensureRoleForeignKey(db *gorm.DB, definition roleForeignKeyDefinition) error {
	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM pg_constraint
			WHERE conname = ?
			  AND conrelid = to_regclass(current_schema() || '.' || ?)
		)`, definition.name, definition.table).Scan(&exists).Error; err != nil {
		return fmt.Errorf("inspect role constraint %s: %w", definition.name, err)
	}
	if exists {
		return nil
	}
	if err := db.Exec(definition.ddl).Error; err != nil {
		return fmt.Errorf("create role constraint %s: %w", definition.name, err)
	}
	return nil
}
