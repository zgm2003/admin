package authplatform

import (
	"context"
	"fmt"
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type constraintDefinition struct {
	name string
	ddl  string
}

var platformConstraints = []constraintDefinition{
	{name: "ck_auth_platform_code", ddl: `ALTER TABLE auth_platform ADD CONSTRAINT ck_auth_platform_code CHECK (code ~ '^[a-z][a-z0-9_]{1,48}$')`},
	{name: "ck_auth_platform_policy_version", ddl: `ALTER TABLE auth_platform ADD CONSTRAINT ck_auth_platform_policy_version CHECK (policy_version >= 1)`},
	{name: "ck_auth_platform_access_ttl_seconds", ddl: `ALTER TABLE auth_platform ADD CONSTRAINT ck_auth_platform_access_ttl_seconds CHECK (access_ttl_seconds BETWEEN 60 AND 2592000)`},
	{name: "ck_auth_platform_refresh_ttl_seconds", ddl: `ALTER TABLE auth_platform ADD CONSTRAINT ck_auth_platform_refresh_ttl_seconds CHECK (refresh_ttl_seconds BETWEEN 60 AND 31536000)`},
	{name: "ck_auth_platform_session_cache_ttl_seconds", ddl: `ALTER TABLE auth_platform ADD CONSTRAINT ck_auth_platform_session_cache_ttl_seconds CHECK (session_cache_ttl_seconds BETWEEN 60 AND 86400)`},
	{name: "ck_auth_platform_access_cache_ttl_seconds", ddl: `ALTER TABLE auth_platform ADD CONSTRAINT ck_auth_platform_access_cache_ttl_seconds CHECK (access_cache_ttl_seconds BETWEEN 60 AND 86400)`},
	{name: "ck_auth_platform_bind_device", ddl: `ALTER TABLE auth_platform ADD CONSTRAINT ck_auth_platform_bind_device CHECK (bind_device IN (0, 1))`},
	{name: "ck_auth_platform_bind_ip", ddl: `ALTER TABLE auth_platform ADD CONSTRAINT ck_auth_platform_bind_ip CHECK (bind_ip IN (0, 1))`},
	{name: "ck_auth_platform_max_sessions", ddl: `ALTER TABLE auth_platform ADD CONSTRAINT ck_auth_platform_max_sessions CHECK (max_sessions BETWEEN 0 AND 100)`},
	{name: "ck_auth_platform_allow_register", ddl: `ALTER TABLE auth_platform ADD CONSTRAINT ck_auth_platform_allow_register CHECK (allow_register IN (0, 1))`},
	{name: "ck_auth_platform_is_enabled", ddl: `ALTER TABLE auth_platform ADD CONSTRAINT ck_auth_platform_is_enabled CHECK (is_enabled IN (0, 1))`},
	{name: "ck_auth_platform_is_builtin", ddl: `ALTER TABLE auth_platform ADD CONSTRAINT ck_auth_platform_is_builtin CHECK (is_builtin IN (0, 1))`},
}

func EnsureSchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("ensure authentication platform schema requires a database")
	}
	db = db.WithContext(ctx)
	for _, definition := range platformConstraints {
		if err := ensureConstraint(db, definition); err != nil {
			return err
		}
	}
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS ux_auth_platform_code_active ON auth_platform (code) WHERE deleted_at IS NULL`).Error; err != nil {
		return fmt.Errorf("create authentication platform index: %w", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`LOCK TABLE auth_platform IN SHARE ROW EXCLUSIVE MODE`).Error; err != nil {
			return fmt.Errorf("lock authentication platforms: %w", err)
		}
		rows := make([]Platform, 0, 2)
		if err := tx.Unscoped().Where("code = ?", BuiltinAdminCode).Order("id ASC").Limit(2).Find(&rows).Error; err != nil {
			return fmt.Errorf("find builtin authentication platform: %w", err)
		}
		if len(rows) == 0 {
			now := time.Now().UTC().Truncate(time.Microsecond)
			value := builtinAdmin(now)
			if err := tx.Create(&value).Error; err != nil {
				return fmt.Errorf("create builtin authentication platform: %w", err)
			}
			return nil
		}
		if len(rows) != 1 || !matchesBuiltinAdmin(rows[0]) {
			return fmt.Errorf("builtin authentication platform admin is missing or damaged")
		}
		return nil
	}); err != nil {
		return fmt.Errorf("ensure builtin authentication platform: %w", err)
	}
	return nil
}

func ensureConstraint(db *gorm.DB, definition constraintDefinition) error {
	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_constraint
			WHERE conname = ? AND conrelid = to_regclass(current_schema() || '.auth_platform')
		)`, definition.name).Scan(&exists).Error; err != nil {
		return fmt.Errorf("inspect authentication platform constraint %s: %w", definition.name, err)
	}
	if exists {
		return nil
	}
	if err := db.Exec(definition.ddl).Error; err != nil {
		return fmt.Errorf("create authentication platform constraint %s: %w", definition.name, err)
	}
	return nil
}

func builtinAdmin(now time.Time) Platform {
	return Platform{
		Code: BuiltinAdminCode, Name: "Admin", PolicyVersion: 1,
		AccessTTLSeconds: 900, RefreshTTLSeconds: 1_209_600,
		SessionCacheTTLSeconds: 1_800, AccessCacheTTLSeconds: 1_800,
		BindDevice: yesno.No, BindIP: yesno.No, MaxSessions: 1,
		AllowRegister: yesno.Yes, IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes,
		CreatedAt: now, UpdatedAt: now,
	}
}

func matchesBuiltinAdmin(value Platform) bool {
	return value.Code == BuiltinAdminCode && value.Name == "Admin" && value.PolicyVersion == 1 &&
		value.AccessTTLSeconds == 900 && value.RefreshTTLSeconds == 1_209_600 &&
		value.SessionCacheTTLSeconds == 1_800 && value.AccessCacheTTLSeconds == 1_800 &&
		value.BindDevice == yesno.No && value.BindIP == yesno.No && value.MaxSessions == 1 &&
		value.AllowRegister == yesno.Yes && value.IsEnabled == yesno.Yes && value.IsBuiltin == yesno.Yes &&
		!value.DeletedAt.Valid
}
