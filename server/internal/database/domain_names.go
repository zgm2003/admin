package database

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type domainRename struct {
	Old string
	New string
}

type domainConstraintRename struct {
	Table    string
	Old      string
	New      string
	Optional bool
}

type domainObjectRename struct {
	Old      string
	New      string
	Optional bool
}

type domainSchemaState uint8

const (
	domainSchemaEmpty domainSchemaState = iota
	domainSchemaLegacy
	domainSchemaCurrent
	domainSchemaMixed
)

var domainTableRenames = []domainRename{
	{Old: "sys_user", New: "user_account"},
	{Old: "sys_user_session", New: "auth_session"},
	{Old: "sys_menu", New: "rbac_menu"},
	{Old: "sys_role", New: "rbac_role"},
	{Old: "sys_user_role", New: "rbac_user_role"},
	{Old: "sys_role_menu", New: "rbac_role_menu"},
	{Old: "sys_access_version", New: "rbac_access_version"},
	{Old: "sys_auth_platform", New: "auth_platform"},
	{Old: "sys_operation_log", New: "audit_operation_log"},
}

var domainSequenceRenames = []domainObjectRename{
	{Old: "sys_user_id_seq", New: "user_account_id_seq"},
	{Old: "sys_user_session_id_seq", New: "auth_session_id_seq"},
	{Old: "sys_menu_id_seq", New: "rbac_menu_id_seq"},
	{Old: "sys_role_id_seq", New: "rbac_role_id_seq"},
	{Old: "sys_user_role_id_seq", New: "rbac_user_role_id_seq"},
	{Old: "sys_role_menu_id_seq", New: "rbac_role_menu_id_seq"},
	{Old: "sys_access_version_user_id_seq", New: "rbac_access_version_user_id_seq"},
	{Old: "sys_auth_platform_id_seq", New: "auth_platform_id_seq"},
	{Old: "sys_operation_log_id_seq", New: "audit_operation_log_id_seq"},
}

var domainIndexRenames = []domainObjectRename{
	{Old: "ux_sys_user_username_active", New: "ux_user_account_username_active"},
	{Old: "ux_sys_user_email_active", New: "ux_user_account_email_active"},
	{Old: "ux_sys_user_session_refresh_hash", New: "ux_auth_session_refresh_hash"},
	{Old: "ix_sys_user_session_user_created", New: "ix_auth_session_user_created"},
	{Old: "ix_sys_user_session_user_platform_active", New: "ix_auth_session_user_platform_active"},
	{Old: "ux_sys_user_session_current", New: "ux_auth_session_current", Optional: true},
	{Old: "ux_sys_menu_code_active", New: "ux_rbac_menu_code_active"},
	{Old: "ux_sys_menu_page_path_active", New: "ux_rbac_menu_page_path_active"},
	{Old: "ix_sys_menu_parent_active", New: "ix_rbac_menu_parent_active"},
	{Old: "ux_sys_role_code_active", New: "ux_rbac_role_code_active"},
	{Old: "ux_sys_role_name_active", New: "ux_rbac_role_name_active"},
	{Old: "ux_sys_role_default_active", New: "ux_rbac_role_default_active"},
	{Old: "ux_sys_user_role_active", New: "ux_rbac_user_role_active"},
	{Old: "ux_sys_role_menu_active", New: "ux_rbac_role_menu_active"},
	{Old: "ux_sys_auth_platform_code_active", New: "ux_auth_platform_code_active"},
	{Old: "ux_sys_operation_log_event_id", New: "ux_audit_operation_log_event_id"},
	{Old: "ux_sys_operation_log_request_id", New: "ux_audit_operation_log_request_id", Optional: true},
	{Old: "ix_sys_operation_log_request_id", New: "ix_audit_operation_log_request_id"},
	{Old: "ix_sys_operation_log_created_at", New: "ix_audit_operation_log_created_at"},
	{Old: "ix_sys_operation_log_user_created", New: "ix_audit_operation_log_user_created"},
	{Old: "ix_sys_operation_log_action_created", New: "ix_audit_operation_log_action_created"},
}

var domainConstraintRenames = []domainConstraintRename{
	{Table: "sys_user", Old: "sys_user_pkey", New: "user_account_pkey"},
	{Table: "sys_user_session", Old: "sys_user_session_pkey", New: "auth_session_pkey"},
	{Table: "sys_menu", Old: "sys_menu_pkey", New: "rbac_menu_pkey"},
	{Table: "sys_role", Old: "sys_role_pkey", New: "rbac_role_pkey"},
	{Table: "sys_user_role", Old: "sys_user_role_pkey", New: "rbac_user_role_pkey"},
	{Table: "sys_role_menu", Old: "sys_role_menu_pkey", New: "rbac_role_menu_pkey"},
	{Table: "sys_access_version", Old: "sys_access_version_pkey", New: "rbac_access_version_pkey"},
	{Table: "sys_auth_platform", Old: "sys_auth_platform_pkey", New: "auth_platform_pkey"},
	{Table: "sys_operation_log", Old: "sys_operation_log_pkey", New: "audit_operation_log_pkey"},

	{Table: "sys_user", Old: "ck_sys_user_is_enabled", New: "ck_user_account_is_enabled"},
	{Table: "sys_user_session", Old: "ck_sys_user_session_version", New: "ck_auth_session_version"},
	{Table: "sys_user_session", Old: "fk_sys_user_session_user", New: "fk_auth_session_user"},
	{Table: "sys_menu", Old: "ck_sys_menu_type", New: "ck_rbac_menu_type"},
	{Table: "sys_menu", Old: "ck_sys_menu_shape", New: "ck_rbac_menu_shape"},
	{Table: "sys_menu", Old: "ck_sys_menu_render_shape", New: "ck_rbac_menu_render_shape", Optional: true},
	{Table: "sys_menu", Old: "ck_sys_menu_sort_order", New: "ck_rbac_menu_sort_order"},
	{Table: "sys_menu", Old: "ck_sys_menu_is_enabled", New: "ck_rbac_menu_is_enabled"},
	{Table: "sys_menu", Old: "ck_sys_menu_is_hidden", New: "ck_rbac_menu_is_hidden"},
	{Table: "sys_menu", Old: "fk_sys_menu_parent", New: "fk_rbac_menu_parent", Optional: true},
	{Table: "sys_role", Old: "ck_sys_role_is_default", New: "ck_rbac_role_is_default"},
	{Table: "sys_role", Old: "ck_sys_role_is_enabled", New: "ck_rbac_role_is_enabled"},
	{Table: "sys_user_role", Old: "fk_sys_user_role_user", New: "fk_rbac_user_role_user"},
	{Table: "sys_user_role", Old: "fk_sys_user_role_role", New: "fk_rbac_user_role_role"},
	{Table: "sys_role_menu", Old: "fk_sys_role_menu_role", New: "fk_rbac_role_menu_role"},
	{Table: "sys_role_menu", Old: "fk_sys_role_menu_menu", New: "fk_rbac_role_menu_menu"},
	{Table: "sys_access_version", Old: "ck_sys_access_version_version", New: "ck_rbac_access_version_version"},
	{Table: "sys_access_version", Old: "fk_sys_access_version_user", New: "fk_rbac_access_version_user"},
	{Table: "sys_auth_platform", Old: "ck_sys_auth_platform_code", New: "ck_auth_platform_code"},
	{Table: "sys_auth_platform", Old: "ck_sys_auth_platform_policy_version", New: "ck_auth_platform_policy_version"},
	{Table: "sys_auth_platform", Old: "ck_sys_auth_platform_access_ttl_seconds", New: "ck_auth_platform_access_ttl_seconds"},
	{Table: "sys_auth_platform", Old: "ck_sys_auth_platform_refresh_ttl_seconds", New: "ck_auth_platform_refresh_ttl_seconds"},
	{Table: "sys_auth_platform", Old: "ck_sys_auth_platform_session_cache_ttl_seconds", New: "ck_auth_platform_session_cache_ttl_seconds"},
	{Table: "sys_auth_platform", Old: "ck_sys_auth_platform_access_cache_ttl_seconds", New: "ck_auth_platform_access_cache_ttl_seconds"},
	{Table: "sys_auth_platform", Old: "ck_sys_auth_platform_bind_device", New: "ck_auth_platform_bind_device"},
	{Table: "sys_auth_platform", Old: "ck_sys_auth_platform_bind_ip", New: "ck_auth_platform_bind_ip"},
	{Table: "sys_auth_platform", Old: "ck_sys_auth_platform_max_sessions", New: "ck_auth_platform_max_sessions"},
	{Table: "sys_auth_platform", Old: "ck_sys_auth_platform_allow_register", New: "ck_auth_platform_allow_register"},
	{Table: "sys_auth_platform", Old: "ck_sys_auth_platform_is_enabled", New: "ck_auth_platform_is_enabled"},
	{Table: "sys_auth_platform", Old: "ck_sys_auth_platform_is_builtin", New: "ck_auth_platform_is_builtin"},
	{Table: "sys_operation_log", Old: "ck_sys_operation_log_is_success", New: "ck_audit_operation_log_is_success"},
	{Table: "sys_operation_log", Old: "ck_sys_operation_log_latency_ms", New: "ck_audit_operation_log_latency_ms"},

	{Table: "sys_user", Old: "sys_user_id_not_null", New: "user_account_id_not_null"},
	{Table: "sys_user", Old: "sys_user_username_not_null", New: "user_account_username_not_null"},
	{Table: "sys_user", Old: "sys_user_email_not_null", New: "user_account_email_not_null"},
	{Table: "sys_user", Old: "sys_user_password_hash_not_null", New: "user_account_password_hash_not_null"},
	{Table: "sys_user", Old: "sys_user_is_enabled_not_null", New: "user_account_is_enabled_not_null"},
	{Table: "sys_user", Old: "sys_user_created_at_not_null", New: "user_account_created_at_not_null"},
	{Table: "sys_user", Old: "sys_user_updated_at_not_null", New: "user_account_updated_at_not_null"},

	{Table: "sys_user_session", Old: "sys_user_session_id_not_null", New: "auth_session_id_not_null"},
	{Table: "sys_user_session", Old: "sys_user_session_user_id_not_null", New: "auth_session_user_id_not_null"},
	{Table: "sys_user_session", Old: "sys_user_session_platform_not_null", New: "auth_session_platform_not_null"},
	{Table: "sys_user_session", Old: "sys_user_session_device_id_not_null", New: "auth_session_device_id_not_null"},
	{Table: "sys_user_session", Old: "sys_user_session_refresh_token_hash_not_null", New: "auth_session_refresh_token_hash_not_null"},
	{Table: "sys_user_session", Old: "sys_user_session_version_not_null", New: "auth_session_version_not_null"},
	{Table: "sys_user_session", Old: "sys_user_session_client_ip_not_null", New: "auth_session_client_ip_not_null"},
	{Table: "sys_user_session", Old: "sys_user_session_user_agent_not_null", New: "auth_session_user_agent_not_null"},
	{Table: "sys_user_session", Old: "sys_user_session_refresh_expires_at_not_null", New: "auth_session_refresh_expires_at_not_null"},
	{Table: "sys_user_session", Old: "sys_user_session_created_at_not_null", New: "auth_session_created_at_not_null"},
	{Table: "sys_user_session", Old: "sys_user_session_updated_at_not_null", New: "auth_session_updated_at_not_null"},

	{Table: "sys_menu", Old: "sys_menu_id_not_null", New: "rbac_menu_id_not_null"},
	{Table: "sys_menu", Old: "sys_menu_menu_type_not_null", New: "rbac_menu_menu_type_not_null"},
	{Table: "sys_menu", Old: "sys_menu_code_not_null", New: "rbac_menu_code_not_null"},
	// The menu protocol makes i18n_key nullable for action nodes, so the
	// legacy NOT NULL constraint is intentionally removed by menu schema setup.
	{Table: "sys_menu", Old: "sys_menu_i18n_key_not_null", New: "rbac_menu_i18n_key_not_null", Optional: true},
	{Table: "sys_menu", Old: "sys_menu_sort_order_not_null", New: "rbac_menu_sort_order_not_null"},
	{Table: "sys_menu", Old: "sys_menu_is_enabled_not_null", New: "rbac_menu_is_enabled_not_null"},
	{Table: "sys_menu", Old: "sys_menu_is_hidden_not_null", New: "rbac_menu_is_hidden_not_null"},
	{Table: "sys_menu", Old: "sys_menu_created_at_not_null", New: "rbac_menu_created_at_not_null"},
	{Table: "sys_menu", Old: "sys_menu_updated_at_not_null", New: "rbac_menu_updated_at_not_null"},

	{Table: "sys_role", Old: "sys_role_id_not_null", New: "rbac_role_id_not_null"},
	{Table: "sys_role", Old: "sys_role_code_not_null", New: "rbac_role_code_not_null"},
	{Table: "sys_role", Old: "sys_role_name_not_null", New: "rbac_role_name_not_null"},
	{Table: "sys_role", Old: "sys_role_is_default_not_null", New: "rbac_role_is_default_not_null"},
	{Table: "sys_role", Old: "sys_role_is_enabled_not_null", New: "rbac_role_is_enabled_not_null"},
	{Table: "sys_role", Old: "sys_role_created_at_not_null", New: "rbac_role_created_at_not_null"},
	{Table: "sys_role", Old: "sys_role_updated_at_not_null", New: "rbac_role_updated_at_not_null"},

	{Table: "sys_user_role", Old: "sys_user_role_id_not_null", New: "rbac_user_role_id_not_null"},
	{Table: "sys_user_role", Old: "sys_user_role_user_id_not_null", New: "rbac_user_role_user_id_not_null"},
	{Table: "sys_user_role", Old: "sys_user_role_role_id_not_null", New: "rbac_user_role_role_id_not_null"},
	{Table: "sys_user_role", Old: "sys_user_role_created_at_not_null", New: "rbac_user_role_created_at_not_null"},
	{Table: "sys_user_role", Old: "sys_user_role_updated_at_not_null", New: "rbac_user_role_updated_at_not_null"},

	{Table: "sys_role_menu", Old: "sys_role_menu_id_not_null", New: "rbac_role_menu_id_not_null"},
	{Table: "sys_role_menu", Old: "sys_role_menu_role_id_not_null", New: "rbac_role_menu_role_id_not_null"},
	{Table: "sys_role_menu", Old: "sys_role_menu_menu_id_not_null", New: "rbac_role_menu_menu_id_not_null"},
	{Table: "sys_role_menu", Old: "sys_role_menu_created_at_not_null", New: "rbac_role_menu_created_at_not_null"},
	{Table: "sys_role_menu", Old: "sys_role_menu_updated_at_not_null", New: "rbac_role_menu_updated_at_not_null"},

	{Table: "sys_access_version", Old: "sys_access_version_user_id_not_null", New: "rbac_access_version_user_id_not_null"},
	{Table: "sys_access_version", Old: "sys_access_version_version_not_null", New: "rbac_access_version_version_not_null"},
	{Table: "sys_access_version", Old: "sys_access_version_created_at_not_null", New: "rbac_access_version_created_at_not_null"},
	{Table: "sys_access_version", Old: "sys_access_version_updated_at_not_null", New: "rbac_access_version_updated_at_not_null"},

	{Table: "sys_auth_platform", Old: "sys_auth_platform_id_not_null", New: "auth_platform_id_not_null"},
	{Table: "sys_auth_platform", Old: "sys_auth_platform_code_not_null", New: "auth_platform_code_not_null"},
	{Table: "sys_auth_platform", Old: "sys_auth_platform_name_not_null", New: "auth_platform_name_not_null"},
	{Table: "sys_auth_platform", Old: "sys_auth_platform_policy_version_not_null", New: "auth_platform_policy_version_not_null"},
	{Table: "sys_auth_platform", Old: "sys_auth_platform_access_ttl_seconds_not_null", New: "auth_platform_access_ttl_seconds_not_null"},
	{Table: "sys_auth_platform", Old: "sys_auth_platform_refresh_ttl_seconds_not_null", New: "auth_platform_refresh_ttl_seconds_not_null"},
	{Table: "sys_auth_platform", Old: "sys_auth_platform_session_cache_ttl_seconds_not_null", New: "auth_platform_session_cache_ttl_seconds_not_null"},
	{Table: "sys_auth_platform", Old: "sys_auth_platform_access_cache_ttl_seconds_not_null", New: "auth_platform_access_cache_ttl_seconds_not_null"},
	{Table: "sys_auth_platform", Old: "sys_auth_platform_bind_device_not_null", New: "auth_platform_bind_device_not_null"},
	{Table: "sys_auth_platform", Old: "sys_auth_platform_bind_ip_not_null", New: "auth_platform_bind_ip_not_null"},
	{Table: "sys_auth_platform", Old: "sys_auth_platform_max_sessions_not_null", New: "auth_platform_max_sessions_not_null"},
	{Table: "sys_auth_platform", Old: "sys_auth_platform_allow_register_not_null", New: "auth_platform_allow_register_not_null"},
	{Table: "sys_auth_platform", Old: "sys_auth_platform_is_enabled_not_null", New: "auth_platform_is_enabled_not_null"},
	{Table: "sys_auth_platform", Old: "sys_auth_platform_is_builtin_not_null", New: "auth_platform_is_builtin_not_null"},
	{Table: "sys_auth_platform", Old: "sys_auth_platform_created_at_not_null", New: "auth_platform_created_at_not_null"},
	{Table: "sys_auth_platform", Old: "sys_auth_platform_updated_at_not_null", New: "auth_platform_updated_at_not_null"},

	{Table: "sys_operation_log", Old: "sys_operation_log_id_not_null", New: "audit_operation_log_id_not_null"},
	{Table: "sys_operation_log", Old: "sys_operation_log_event_id_not_null", New: "audit_operation_log_event_id_not_null"},
	{Table: "sys_operation_log", Old: "sys_operation_log_request_id_not_null", New: "audit_operation_log_request_id_not_null"},
	{Table: "sys_operation_log", Old: "sys_operation_log_method_not_null", New: "audit_operation_log_method_not_null"},
	{Table: "sys_operation_log", Old: "sys_operation_log_route_not_null", New: "audit_operation_log_route_not_null"},
	{Table: "sys_operation_log", Old: "sys_operation_log_module_not_null", New: "audit_operation_log_module_not_null"},
	{Table: "sys_operation_log", Old: "sys_operation_log_action_not_null", New: "audit_operation_log_action_not_null"},
	{Table: "sys_operation_log", Old: "sys_operation_log_client_ip_not_null", New: "audit_operation_log_client_ip_not_null"},
	{Table: "sys_operation_log", Old: "sys_operation_log_user_agent_not_null", New: "audit_operation_log_user_agent_not_null"},
	{Table: "sys_operation_log", Old: "sys_operation_log_status_code_not_null", New: "audit_operation_log_status_code_not_null"},
	{Table: "sys_operation_log", Old: "sys_operation_log_is_success_not_null", New: "audit_operation_log_is_success_not_null"},
	{Table: "sys_operation_log", Old: "sys_operation_log_latency_ms_not_null", New: "audit_operation_log_latency_ms_not_null"},
	{Table: "sys_operation_log", Old: "sys_operation_log_created_at_not_null", New: "audit_operation_log_created_at_not_null"},
	{Table: "sys_operation_log", Old: "sys_operation_log_updated_at_not_null", New: "audit_operation_log_updated_at_not_null"},
}

func PrepareDomainNames(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("prepare domain names requires a database")
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		state, err := inspectDomainRenameState(tx)
		if err != nil {
			return err
		}
		switch state {
		case domainSchemaEmpty:
			return nil
		case domainSchemaCurrent:
			return verifyCurrentDomainObjects(tx)
		case domainSchemaLegacy:
			return renameLegacyDomainObjects(tx)
		case domainSchemaMixed:
			return fmt.Errorf("domain schema mixes legacy and current names")
		default:
			return fmt.Errorf("unknown domain schema state %d", state)
		}
	})
}

func inspectDomainRenameState(db *gorm.DB) (domainSchemaState, error) {
	legacyCount := 0
	currentCount := 0
	missingCount := 0
	for _, rename := range domainTableRenames {
		oldExists, err := domainRelationExists(db, rename.Old)
		if err != nil {
			return domainSchemaMixed, fmt.Errorf("inspect legacy table %s: %w", rename.Old, err)
		}
		newExists, err := domainRelationExists(db, rename.New)
		if err != nil {
			return domainSchemaMixed, fmt.Errorf("inspect current table %s: %w", rename.New, err)
		}
		if oldExists && newExists {
			return domainSchemaMixed, fmt.Errorf("domain table pair conflicts: %s and %s both exist", rename.Old, rename.New)
		}
		switch {
		case oldExists:
			legacyCount++
		case newExists:
			currentCount++
		default:
			missingCount++
		}
	}

	switch {
	case missingCount == len(domainTableRenames):
		return domainSchemaEmpty, nil
	case legacyCount == len(domainTableRenames):
		return domainSchemaLegacy, nil
	case currentCount == len(domainTableRenames):
		return domainSchemaCurrent, nil
	default:
		return domainSchemaMixed, nil
	}
}

func renameLegacyDomainObjects(db *gorm.DB) error {
	for _, rename := range domainConstraintRenames {
		if err := renameDomainConstraint(db, rename); err != nil {
			return err
		}
	}
	for _, rename := range domainIndexRenames {
		if err := renameDomainRelation(db, "index", rename, `ALTER INDEX "%s" RENAME TO "%s"`); err != nil {
			return err
		}
	}
	for _, rename := range domainSequenceRenames {
		if err := renameDomainRelation(db, "sequence", rename, `ALTER SEQUENCE "%s" RENAME TO "%s"`); err != nil {
			return err
		}
	}
	for _, rename := range domainTableRenames {
		object := domainObjectRename{Old: rename.Old, New: rename.New}
		if err := renameDomainRelation(db, "table", object, `ALTER TABLE "%s" RENAME TO "%s"`); err != nil {
			return err
		}
	}
	return verifyCurrentDomainObjects(db)
}

func renameDomainConstraint(db *gorm.DB, rename domainConstraintRename) error {
	oldExists, err := domainConstraintExists(db, rename.Table, rename.Old)
	if err != nil {
		return fmt.Errorf("inspect constraint %s: %w", rename.Old, err)
	}
	newExists, err := domainConstraintExists(db, rename.Table, rename.New)
	if err != nil {
		return fmt.Errorf("inspect constraint %s: %w", rename.New, err)
	}
	if oldExists && newExists {
		return fmt.Errorf("constraint names conflict on %s: %s and %s both exist", rename.Table, rename.Old, rename.New)
	}
	if !oldExists {
		if rename.Optional && !newExists {
			return nil
		}
		if newExists {
			return fmt.Errorf("constraint %s was already renamed while table %s is legacy", rename.New, rename.Table)
		}
		return fmt.Errorf("required legacy constraint %s is missing from %s", rename.Old, rename.Table)
	}
	if err := db.Exec(fmt.Sprintf(`ALTER TABLE "%s" RENAME CONSTRAINT "%s" TO "%s"`, rename.Table, rename.Old, rename.New)).Error; err != nil {
		return fmt.Errorf("rename constraint %s to %s: %w", rename.Old, rename.New, err)
	}
	return nil
}

func renameDomainRelation(db *gorm.DB, kind string, rename domainObjectRename, ddl string) error {
	oldExists, err := domainRelationExists(db, rename.Old)
	if err != nil {
		return fmt.Errorf("inspect %s %s: %w", kind, rename.Old, err)
	}
	newExists, err := domainRelationExists(db, rename.New)
	if err != nil {
		return fmt.Errorf("inspect %s %s: %w", kind, rename.New, err)
	}
	if oldExists && newExists {
		return fmt.Errorf("%s names conflict: %s and %s both exist", kind, rename.Old, rename.New)
	}
	if !oldExists {
		if rename.Optional && !newExists {
			return nil
		}
		if newExists {
			return fmt.Errorf("%s %s was already renamed while domain tables are legacy", kind, rename.New)
		}
		return fmt.Errorf("required legacy %s %s is missing", kind, rename.Old)
	}
	if err := db.Exec(fmt.Sprintf(ddl, rename.Old, rename.New)).Error; err != nil {
		return fmt.Errorf("rename %s %s to %s: %w", kind, rename.Old, rename.New, err)
	}
	return nil
}

func verifyCurrentDomainObjects(db *gorm.DB) error {
	for _, rename := range domainTableRenames {
		if err := verifyCurrentDomainRelation(db, "table", domainObjectRename{Old: rename.Old, New: rename.New}); err != nil {
			return err
		}
	}
	for _, rename := range domainConstraintRenames {
		currentTable := currentDomainTableName(rename.Table)
		oldExists, err := domainConstraintExists(db, currentTable, rename.Old)
		if err != nil {
			return fmt.Errorf("inspect legacy constraint %s: %w", rename.Old, err)
		}
		newExists, err := domainConstraintExists(db, currentTable, rename.New)
		if err != nil {
			return fmt.Errorf("inspect current constraint %s: %w", rename.New, err)
		}
		if oldExists {
			return fmt.Errorf("legacy constraint %s remains on %s", rename.Old, currentTable)
		}
		if !newExists && !rename.Optional {
			return fmt.Errorf("required current constraint %s is missing from %s", rename.New, currentTable)
		}
	}
	for _, rename := range domainIndexRenames {
		if err := verifyCurrentDomainRelation(db, "index", rename); err != nil {
			return err
		}
	}
	for _, rename := range domainSequenceRenames {
		if err := verifyCurrentDomainRelation(db, "sequence", rename); err != nil {
			return err
		}
	}
	return nil
}

func verifyCurrentDomainRelation(db *gorm.DB, kind string, rename domainObjectRename) error {
	oldExists, err := domainRelationExists(db, rename.Old)
	if err != nil {
		return fmt.Errorf("inspect legacy %s %s: %w", kind, rename.Old, err)
	}
	newExists, err := domainRelationExists(db, rename.New)
	if err != nil {
		return fmt.Errorf("inspect current %s %s: %w", kind, rename.New, err)
	}
	if oldExists {
		return fmt.Errorf("legacy %s %s still exists", kind, rename.Old)
	}
	if !newExists && !rename.Optional {
		return fmt.Errorf("required current %s %s is missing", kind, rename.New)
	}
	return nil
}

func currentDomainTableName(legacyName string) string {
	for _, rename := range domainTableRenames {
		if rename.Old == legacyName {
			return rename.New
		}
	}
	return legacyName
}

func domainRelationExists(db *gorm.DB, name string) (bool, error) {
	var exists bool
	err := db.Raw(`SELECT to_regclass(current_schema() || '.' || ?) IS NOT NULL`, name).Scan(&exists).Error
	return exists, err
}

func domainConstraintExists(db *gorm.DB, tableName, constraintName string) (bool, error) {
	var exists bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_constraint
			WHERE conrelid = to_regclass(current_schema() || '.' || ?)
			  AND conname = ?
		)`, tableName, constraintName).Scan(&exists).Error
	return exists, err
}
