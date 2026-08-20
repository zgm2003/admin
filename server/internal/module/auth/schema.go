package auth

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

var authenticationIndexes = []string{
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_sys_user_username_active ON sys_user (lower(username)) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_sys_user_email_active ON sys_user (email) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_sys_user_session_refresh_hash ON sys_user_session (refresh_token_hash)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_sys_user_session_current ON sys_user_session (user_id) WHERE revoked_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS ix_sys_user_session_user_created ON sys_user_session (user_id, created_at DESC)`,
}

var authenticationForeignKeys = []foreignKeyDefinition{
	{
		name:  "fk_sys_user_session_user",
		table: "sys_user_session",
		ddl:   `ALTER TABLE sys_user_session ADD CONSTRAINT fk_sys_user_session_user FOREIGN KEY (user_id) REFERENCES sys_user(id) ON DELETE RESTRICT`,
	},
}

type foreignKeyDefinition struct {
	name  string
	table string
	ddl   string
}

func EnsureSchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("ensure authentication schema requires a database")
	}
	db = db.WithContext(ctx)
	for _, statement := range authenticationIndexes {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("create authentication index: %w", err)
		}
	}
	for _, definition := range authenticationForeignKeys {
		if err := ensureForeignKey(db, definition); err != nil {
			return err
		}
	}
	return nil
}

func ensureForeignKey(db *gorm.DB, definition foreignKeyDefinition) error {
	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM pg_constraint
			WHERE conname = ?
			  AND conrelid = to_regclass(current_schema() || '.' || ?)
		)`, definition.name, definition.table).Scan(&exists).Error; err != nil {
		return fmt.Errorf("inspect authentication constraint %s: %w", definition.name, err)
	}
	if exists {
		return nil
	}
	if err := db.Exec(definition.ddl).Error; err != nil {
		return fmt.Errorf("create authentication constraint %s: %w", definition.name, err)
	}
	return nil
}
