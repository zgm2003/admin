package access

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

func EnsureSchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("ensure access schema requires a database")
	}
	db = db.WithContext(ctx)
	if err := ensureAccessConstraint(db, "ck_rbac_access_version_version", `ALTER TABLE rbac_access_version ADD CONSTRAINT ck_rbac_access_version_version CHECK (version >= 1)`); err != nil {
		return err
	}
	if err := ensureAccessConstraint(db, "fk_rbac_access_version_user", `ALTER TABLE rbac_access_version ADD CONSTRAINT fk_rbac_access_version_user FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE RESTRICT`); err != nil {
		return err
	}
	if err := db.Exec(`
		INSERT INTO rbac_access_version (user_id, version, created_at, updated_at)
		SELECT id, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		FROM user_account
		ON CONFLICT (user_id) DO NOTHING`).Error; err != nil {
		return fmt.Errorf("backfill access versions: %w", err)
	}
	return nil
}

func ensureAccessConstraint(db *gorm.DB, name, ddl string) error {
	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_constraint
			WHERE conname = ? AND conrelid = to_regclass(current_schema() || '.rbac_access_version')
		)`, name).Scan(&exists).Error; err != nil {
		return fmt.Errorf("inspect access constraint %s: %w", name, err)
	}
	if exists {
		return nil
	}
	if err := db.Exec(ddl).Error; err != nil {
		return fmt.Errorf("create access constraint %s: %w", name, err)
	}
	return nil
}
