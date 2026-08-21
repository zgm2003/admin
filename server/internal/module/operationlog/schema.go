package operationlog

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type constraintDefinition struct {
	name string
	ddl  string
}

var operationLogConstraints = []constraintDefinition{
	{
		name: "ck_sys_operation_log_is_success",
		ddl:  "ALTER TABLE sys_operation_log ADD CONSTRAINT ck_sys_operation_log_is_success CHECK (is_success IN (0, 1))",
	},
	{
		name: "ck_sys_operation_log_latency_ms",
		ddl:  "ALTER TABLE sys_operation_log ADD CONSTRAINT ck_sys_operation_log_latency_ms CHECK (latency_ms >= 0)",
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
		"CREATE UNIQUE INDEX IF NOT EXISTS ux_sys_operation_log_request_id ON sys_operation_log (request_id)",
		"CREATE INDEX IF NOT EXISTS ix_sys_operation_log_created_at ON sys_operation_log (created_at DESC)",
		"CREATE INDEX IF NOT EXISTS ix_sys_operation_log_user_created ON sys_operation_log (user_id, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS ix_sys_operation_log_action_created ON sys_operation_log (action, created_at DESC)",
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
			"WHERE conname = ? AND conrelid = to_regclass(current_schema() || '.sys_operation_log'))",
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
