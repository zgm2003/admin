package auth

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

var authenticationIndexes = []string{
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_user_account_username_active ON user_account (lower(username)) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_user_account_email_active ON user_account (email) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_auth_session_refresh_hash ON auth_session (refresh_token_hash)`,
	`CREATE INDEX IF NOT EXISTS ix_auth_session_user_created ON auth_session (user_id, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS ix_auth_session_user_platform_active ON auth_session (user_id, platform, created_at DESC, id DESC) WHERE revoked_at IS NULL`,
}

var authenticationForeignKeys = []foreignKeyDefinition{
	{
		name:  "fk_auth_session_user",
		table: "auth_session",
		ddl:   `ALTER TABLE auth_session ADD CONSTRAINT fk_auth_session_user FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE RESTRICT`,
	},
}

type foreignKeyDefinition struct {
	name  string
	table string
	ddl   string
}

func PrepareSessionSchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("prepare session schema requires a database")
	}
	db = db.WithContext(ctx)
	var tableExists bool
	if err := db.Raw(`SELECT to_regclass(current_schema() || '.auth_session') IS NOT NULL`).Scan(&tableExists).Error; err != nil {
		return fmt.Errorf("inspect session table: %w", err)
	}
	if !tableExists {
		return nil
	}
	var columnCount int64
	if err := db.Raw(`
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = 'auth_session'
		  AND column_name IN ('platform', 'device_id')`).Scan(&columnCount).Error; err != nil {
		return fmt.Errorf("inspect session migration columns: %w", err)
	}
	if columnCount == 2 {
		return nil
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, statement := range []string{
			`ALTER TABLE auth_session ADD COLUMN IF NOT EXISTS platform VARCHAR(49) NOT NULL DEFAULT 'admin'`,
			`ALTER TABLE auth_session ADD COLUMN IF NOT EXISTS device_id VARCHAR(36) NOT NULL DEFAULT ''`,
			`UPDATE auth_session
			 SET revoked_at = COALESCE(revoked_at, CURRENT_TIMESTAMP),
			     updated_at = CASE WHEN revoked_at IS NULL THEN CURRENT_TIMESTAMP ELSE updated_at END
			 WHERE revoked_at IS NULL`,
			`DROP INDEX IF EXISTS ux_auth_session_current`,
			`ALTER TABLE auth_session ALTER COLUMN platform DROP DEFAULT`,
			`ALTER TABLE auth_session ALTER COLUMN device_id DROP DEFAULT`,
		} {
			if err := tx.Exec(statement).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("migrate session schema: %w", err)
	}
	return nil
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
