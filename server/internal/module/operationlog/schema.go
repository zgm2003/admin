package operationlog

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

func PrepareSchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("prepare operation log schema requires a database")
	}
	db = db.WithContext(ctx)
	var tableExists bool
	if err := db.Raw(`SELECT to_regclass(current_schema() || '.audit_operation_log') IS NOT NULL`).Scan(&tableExists).Error; err != nil {
		return fmt.Errorf("inspect operation log table: %w", err)
	}
	if !tableExists {
		return nil
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, statement := range []string{
			`ALTER TABLE audit_operation_log ADD COLUMN IF NOT EXISTS event_id VARCHAR(64)`,
			`UPDATE audit_operation_log SET event_id = 'legacy-' || id::text WHERE event_id IS NULL OR event_id = ''`,
			`ALTER TABLE audit_operation_log ALTER COLUMN event_id SET NOT NULL`,
		} {
			if err := tx.Exec(statement).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("migrate operation log event ID: %w", err)
	}
	return nil
}

type constraintDefinition struct {
	name string
	ddl  string
}

var operationLogConstraints = []constraintDefinition{
	{
		name: "ck_audit_operation_log_is_success",
		ddl:  "ALTER TABLE audit_operation_log ADD CONSTRAINT ck_audit_operation_log_is_success CHECK (is_success IN (0, 1))",
	},
	{
		name: "ck_audit_operation_log_latency_ms",
		ddl:  "ALTER TABLE audit_operation_log ADD CONSTRAINT ck_audit_operation_log_latency_ms CHECK (latency_ms >= 0)",
	},
}

func EnsureSchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("ensure operation log schema requires a database")
	}
	db = db.WithContext(ctx)
	for _, definition := range operationLogConstraints {
		if err := ensureConstraint(db, definition); err != nil {
			return err
		}
	}
	indexes := []string{
		"DROP INDEX IF EXISTS ux_audit_operation_log_request_id",
		"CREATE UNIQUE INDEX IF NOT EXISTS ux_audit_operation_log_event_id ON audit_operation_log (event_id)",
		"CREATE INDEX IF NOT EXISTS ix_audit_operation_log_request_id ON audit_operation_log (request_id)",
		"CREATE INDEX IF NOT EXISTS ix_audit_operation_log_created_at ON audit_operation_log (created_at DESC)",
		"CREATE INDEX IF NOT EXISTS ix_audit_operation_log_user_created ON audit_operation_log (user_id, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS ix_audit_operation_log_action_created ON audit_operation_log (action, created_at DESC)",
	}
	for _, ddl := range indexes {
		if err := db.Exec(ddl).Error; err != nil {
			return fmt.Errorf("create operation log index: %w", err)
		}
	}
	return nil
}

func ensureConstraint(db *gorm.DB, definition constraintDefinition) error {
	var exists bool
	if err := db.Raw(
		"SELECT EXISTS (SELECT 1 FROM pg_constraint "+
			"WHERE conname = ? AND conrelid = to_regclass(current_schema() || '.audit_operation_log'))",
		definition.name,
	).Scan(&exists).Error; err != nil {
		return fmt.Errorf("inspect operation log constraint %s: %w", definition.name, err)
	}
	if exists {
		return nil
	}
	if err := db.Exec(definition.ddl).Error; err != nil {
		return fmt.Errorf("create operation log constraint %s: %w", definition.name, err)
	}
	return nil
}
