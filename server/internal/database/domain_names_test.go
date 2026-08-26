package database_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/database/testschema"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

type objectRename struct {
	old string
	new string
}

var expectedTableRenames = []objectRename{
	{old: "sys_user", new: "user_account"},
	{old: "sys_user_session", new: "auth_session"},
	{old: "sys_menu", new: "rbac_menu"},
	{old: "sys_role", new: "rbac_role"},
	{old: "sys_user_role", new: "rbac_user_role"},
	{old: "sys_role_menu", new: "rbac_role_menu"},
	{old: "sys_access_version", new: "rbac_access_version"},
	{old: "sys_auth_platform", new: "auth_platform"},
	{old: "sys_operation_log", new: "audit_operation_log"},
}

var expectedSequenceRenames = []objectRename{
	{old: "sys_user_id_seq", new: "user_account_id_seq"},
	{old: "sys_user_session_id_seq", new: "auth_session_id_seq"},
	{old: "sys_menu_id_seq", new: "rbac_menu_id_seq"},
	{old: "sys_role_id_seq", new: "rbac_role_id_seq"},
	{old: "sys_user_role_id_seq", new: "rbac_user_role_id_seq"},
	{old: "sys_role_menu_id_seq", new: "rbac_role_menu_id_seq"},
	{old: "sys_access_version_user_id_seq", new: "rbac_access_version_user_id_seq"},
	{old: "sys_auth_platform_id_seq", new: "auth_platform_id_seq"},
	{old: "sys_operation_log_id_seq", new: "audit_operation_log_id_seq"},
}

var expectedIndexRenames = []objectRename{
	{old: "ux_sys_user_username_active", new: "ux_user_account_username_active"},
	{old: "ux_sys_user_email_active", new: "ux_user_account_email_active"},
	{old: "ux_sys_user_session_refresh_hash", new: "ux_auth_session_refresh_hash"},
	{old: "ix_sys_user_session_user_created", new: "ix_auth_session_user_created"},
	{old: "ix_sys_user_session_user_platform_active", new: "ix_auth_session_user_platform_active"},
	{old: "ux_sys_user_session_current", new: "ux_auth_session_current"},
	{old: "ux_sys_menu_code_active", new: "ux_rbac_menu_code_active"},
	{old: "ux_sys_menu_page_path_active", new: "ux_rbac_menu_page_path_active"},
	{old: "ix_sys_menu_parent_active", new: "ix_rbac_menu_parent_active"},
	{old: "ux_sys_role_code_active", new: "ux_rbac_role_code_active"},
	{old: "ux_sys_role_name_active", new: "ux_rbac_role_name_active"},
	{old: "ux_sys_role_default_active", new: "ux_rbac_role_default_active"},
	{old: "ux_sys_user_role_active", new: "ux_rbac_user_role_active"},
	{old: "ux_sys_role_menu_active", new: "ux_rbac_role_menu_active"},
	{old: "ux_sys_auth_platform_code_active", new: "ux_auth_platform_code_active"},
	{old: "ux_sys_operation_log_event_id", new: "ux_audit_operation_log_event_id"},
	{old: "ux_sys_operation_log_request_id", new: "ux_audit_operation_log_request_id"},
	{old: "ix_sys_operation_log_request_id", new: "ix_audit_operation_log_request_id"},
	{old: "ix_sys_operation_log_created_at", new: "ix_audit_operation_log_created_at"},
	{old: "ix_sys_operation_log_user_created", new: "ix_audit_operation_log_user_created"},
	{old: "ix_sys_operation_log_action_created", new: "ix_audit_operation_log_action_created"},
}

func TestPrepareDomainNamesRenamesTablesAndOwnedObjects(t *testing.T) {
	db, ctx := openDomainNamesDatabase(t, "complete")
	createLegacyDomainSchema(t, db)

	if err := database.PrepareDomainNames(ctx, db); err != nil {
		t.Fatal(err)
	}

	assertCurrentDomainSchema(t, db)
	assertNoOwnedObjectPrefix(t, db, "sys_")
	assertRowPreserved(t, db, "user_account", 101)
	assertRowPreserved(t, db, "rbac_menu", 301)
	assertRowPreserved(t, db, "audit_operation_log", 901)
}

func TestPrepareDomainNamesIsIdempotentForCompleteNewSchema(t *testing.T) {
	db, ctx := openDomainNamesDatabase(t, "idempotent")
	createLegacyDomainSchema(t, db)
	if err := database.PrepareDomainNames(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := database.PrepareDomainNames(ctx, db); err != nil {
		t.Fatalf("second PrepareDomainNames() error = %v", err)
	}
	assertCurrentDomainSchema(t, db)
}

func TestPrepareDomainNamesAllowsProtocolRemovedMenuI18nConstraint(t *testing.T) {
	db, ctx := openDomainNamesDatabase(t, "nullable_menu_i18n")
	createLegacyDomainSchema(t, db)
	if err := database.PrepareDomainNames(ctx, db); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `ALTER TABLE rbac_menu DROP CONSTRAINT rbac_menu_i18n_key_not_null`)
	if err := database.PrepareDomainNames(ctx, db); err != nil {
		t.Fatalf("PrepareDomainNames() after protocol schema update = %v", err)
	}
}

func TestPrepareDomainNamesAllowsCompletelyEmptySchema(t *testing.T) {
	db, ctx := openDomainNamesDatabase(t, "empty")
	if err := database.PrepareDomainNames(ctx, db); err != nil {
		t.Fatalf("PrepareDomainNames() error = %v", err)
	}
}

func TestPrepareDomainNamesRejectsOldAndNewTableTogether(t *testing.T) {
	db, ctx := openDomainNamesDatabase(t, "pair_conflict")
	createLegacyDomainSchema(t, db)
	mustExec(t, db, `CREATE TABLE user_account (id BIGINT PRIMARY KEY)`)

	err := database.PrepareDomainNames(ctx, db)
	if err == nil || !strings.Contains(err.Error(), "sys_user") || !strings.Contains(err.Error(), "user_account") {
		t.Fatalf("PrepareDomainNames() error = %v, want conflicting pair", err)
	}
	for _, rename := range expectedTableRenames {
		assertRelationExists(t, db, rename.old)
	}
}

func TestPrepareDomainNamesRejectsMixedGeneration(t *testing.T) {
	db, ctx := openDomainNamesDatabase(t, "mixed")
	createLegacyDomainSchema(t, db)
	mustExec(t, db, `ALTER TABLE sys_operation_log RENAME TO audit_operation_log`)

	err := database.PrepareDomainNames(ctx, db)
	if err == nil || !strings.Contains(err.Error(), "mixes legacy and current") {
		t.Fatalf("PrepareDomainNames() error = %v, want mixed generation", err)
	}
	assertRelationExists(t, db, "sys_user")
	assertRelationExists(t, db, "audit_operation_log")
}

func TestPrepareDomainNamesRollsBackAllRenames(t *testing.T) {
	db, ctx := openDomainNamesDatabase(t, "rollback")
	createLegacyDomainSchema(t, db)
	mustExec(t, db, `CREATE SEQUENCE rbac_menu_id_seq`)

	err := database.PrepareDomainNames(ctx, db)
	if err == nil || !strings.Contains(err.Error(), "rbac_menu_id_seq") {
		t.Fatalf("PrepareDomainNames() error = %v, want sequence conflict", err)
	}
	for _, rename := range expectedTableRenames {
		assertRelationExists(t, db, rename.old)
		assertRelationMissing(t, db, rename.new)
	}
	assertConstraintExists(t, db, "sys_user_pkey")
	assertRelationExists(t, db, "ux_sys_user_username_active")
}

func openDomainNamesDatabase(t *testing.T, suffix string) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration test")
	}
	if err := godotenv.Load("../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("load server .env: %v", err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatalf("load worker config: %v", err)
	}
	return testschema.Open(t, settings.PostgresDSN, "test_domain_names_"+suffix)
}

func createLegacyDomainSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, rename := range expectedSequenceRenames {
		mustExec(t, db, `CREATE SEQUENCE `+rename.old)
	}

	statements := []string{
		`CREATE TABLE sys_user (
			id BIGINT CONSTRAINT sys_user_id_not_null NOT NULL DEFAULT nextval('sys_user_id_seq'),
			username VARCHAR(64) CONSTRAINT sys_user_username_not_null NOT NULL,
			email VARCHAR(254) CONSTRAINT sys_user_email_not_null NOT NULL,
			password_hash VARCHAR(255) CONSTRAINT sys_user_password_hash_not_null NOT NULL,
			is_enabled SMALLINT CONSTRAINT sys_user_is_enabled_not_null NOT NULL,
			created_at TIMESTAMPTZ CONSTRAINT sys_user_created_at_not_null NOT NULL,
			updated_at TIMESTAMPTZ CONSTRAINT sys_user_updated_at_not_null NOT NULL,
			deleted_at TIMESTAMPTZ,
			CONSTRAINT sys_user_pkey PRIMARY KEY (id),
			CONSTRAINT ck_sys_user_is_enabled CHECK (is_enabled IN (0, 1))
		)`,
		`CREATE TABLE sys_user_session (
			id BIGINT CONSTRAINT sys_user_session_id_not_null NOT NULL DEFAULT nextval('sys_user_session_id_seq'),
			user_id BIGINT CONSTRAINT sys_user_session_user_id_not_null NOT NULL,
			platform VARCHAR(64) CONSTRAINT sys_user_session_platform_not_null NOT NULL,
			device_id VARCHAR(64) CONSTRAINT sys_user_session_device_id_not_null NOT NULL,
			refresh_token_hash VARCHAR(64) CONSTRAINT sys_user_session_refresh_token_hash_not_null NOT NULL,
			version BIGINT CONSTRAINT sys_user_session_version_not_null NOT NULL,
			client_ip VARCHAR(64) CONSTRAINT sys_user_session_client_ip_not_null NOT NULL,
			user_agent TEXT CONSTRAINT sys_user_session_user_agent_not_null NOT NULL,
			refresh_expires_at TIMESTAMPTZ CONSTRAINT sys_user_session_refresh_expires_at_not_null NOT NULL,
			created_at TIMESTAMPTZ CONSTRAINT sys_user_session_created_at_not_null NOT NULL,
			updated_at TIMESTAMPTZ CONSTRAINT sys_user_session_updated_at_not_null NOT NULL,
			revoked_at TIMESTAMPTZ,
			deleted_at TIMESTAMPTZ,
			CONSTRAINT sys_user_session_pkey PRIMARY KEY (id),
			CONSTRAINT ck_sys_user_session_version CHECK (version >= 1),
			CONSTRAINT fk_sys_user_session_user FOREIGN KEY (user_id) REFERENCES sys_user(id) ON DELETE RESTRICT
		)`,
		`CREATE TABLE sys_menu (
			id BIGINT CONSTRAINT sys_menu_id_not_null NOT NULL DEFAULT nextval('sys_menu_id_seq'),
			parent_id BIGINT,
			menu_type VARCHAR(16) CONSTRAINT sys_menu_menu_type_not_null NOT NULL,
			code VARCHAR(128) CONSTRAINT sys_menu_code_not_null NOT NULL,
			i18n_key VARCHAR(128) CONSTRAINT sys_menu_i18n_key_not_null NOT NULL,
			path VARCHAR(255),
			component_path VARCHAR(255),
			sort_order INTEGER CONSTRAINT sys_menu_sort_order_not_null NOT NULL,
			is_enabled SMALLINT CONSTRAINT sys_menu_is_enabled_not_null NOT NULL,
			is_hidden SMALLINT CONSTRAINT sys_menu_is_hidden_not_null NOT NULL,
			created_at TIMESTAMPTZ CONSTRAINT sys_menu_created_at_not_null NOT NULL,
			updated_at TIMESTAMPTZ CONSTRAINT sys_menu_updated_at_not_null NOT NULL,
			deleted_at TIMESTAMPTZ,
			CONSTRAINT sys_menu_pkey PRIMARY KEY (id),
			CONSTRAINT ck_sys_menu_type CHECK (menu_type IN ('directory', 'page', 'button')),
			CONSTRAINT ck_sys_menu_shape CHECK (code <> ''),
			CONSTRAINT ck_sys_menu_render_shape CHECK (menu_type <> 'invalid'),
			CONSTRAINT ck_sys_menu_sort_order CHECK (sort_order >= 0),
			CONSTRAINT ck_sys_menu_is_enabled CHECK (is_enabled IN (0, 1)),
			CONSTRAINT ck_sys_menu_is_hidden CHECK (is_hidden IN (0, 1)),
			CONSTRAINT fk_sys_menu_parent FOREIGN KEY (parent_id) REFERENCES sys_menu(id) ON DELETE RESTRICT
		)`,
		`CREATE TABLE sys_role (
			id BIGINT CONSTRAINT sys_role_id_not_null NOT NULL DEFAULT nextval('sys_role_id_seq'),
			code VARCHAR(64) CONSTRAINT sys_role_code_not_null NOT NULL,
			name VARCHAR(64) CONSTRAINT sys_role_name_not_null NOT NULL,
			is_default SMALLINT CONSTRAINT sys_role_is_default_not_null NOT NULL,
			is_enabled SMALLINT CONSTRAINT sys_role_is_enabled_not_null NOT NULL,
			created_at TIMESTAMPTZ CONSTRAINT sys_role_created_at_not_null NOT NULL,
			updated_at TIMESTAMPTZ CONSTRAINT sys_role_updated_at_not_null NOT NULL,
			deleted_at TIMESTAMPTZ,
			CONSTRAINT sys_role_pkey PRIMARY KEY (id),
			CONSTRAINT ck_sys_role_is_default CHECK (is_default IN (0, 1)),
			CONSTRAINT ck_sys_role_is_enabled CHECK (is_enabled IN (0, 1))
		)`,
		`CREATE TABLE sys_user_role (
			id BIGINT CONSTRAINT sys_user_role_id_not_null NOT NULL DEFAULT nextval('sys_user_role_id_seq'),
			user_id BIGINT CONSTRAINT sys_user_role_user_id_not_null NOT NULL,
			role_id BIGINT CONSTRAINT sys_user_role_role_id_not_null NOT NULL,
			created_at TIMESTAMPTZ CONSTRAINT sys_user_role_created_at_not_null NOT NULL,
			updated_at TIMESTAMPTZ CONSTRAINT sys_user_role_updated_at_not_null NOT NULL,
			deleted_at TIMESTAMPTZ,
			CONSTRAINT sys_user_role_pkey PRIMARY KEY (id),
			CONSTRAINT fk_sys_user_role_user FOREIGN KEY (user_id) REFERENCES sys_user(id) ON DELETE RESTRICT,
			CONSTRAINT fk_sys_user_role_role FOREIGN KEY (role_id) REFERENCES sys_role(id) ON DELETE RESTRICT
		)`,
		`CREATE TABLE sys_role_menu (
			id BIGINT CONSTRAINT sys_role_menu_id_not_null NOT NULL DEFAULT nextval('sys_role_menu_id_seq'),
			role_id BIGINT CONSTRAINT sys_role_menu_role_id_not_null NOT NULL,
			menu_id BIGINT CONSTRAINT sys_role_menu_menu_id_not_null NOT NULL,
			created_at TIMESTAMPTZ CONSTRAINT sys_role_menu_created_at_not_null NOT NULL,
			updated_at TIMESTAMPTZ CONSTRAINT sys_role_menu_updated_at_not_null NOT NULL,
			deleted_at TIMESTAMPTZ,
			CONSTRAINT sys_role_menu_pkey PRIMARY KEY (id),
			CONSTRAINT fk_sys_role_menu_role FOREIGN KEY (role_id) REFERENCES sys_role(id) ON DELETE RESTRICT,
			CONSTRAINT fk_sys_role_menu_menu FOREIGN KEY (menu_id) REFERENCES sys_menu(id) ON DELETE RESTRICT
		)`,
		`CREATE TABLE sys_access_version (
			user_id BIGINT CONSTRAINT sys_access_version_user_id_not_null NOT NULL DEFAULT nextval('sys_access_version_user_id_seq'),
			version BIGINT CONSTRAINT sys_access_version_version_not_null NOT NULL,
			created_at TIMESTAMPTZ CONSTRAINT sys_access_version_created_at_not_null NOT NULL,
			updated_at TIMESTAMPTZ CONSTRAINT sys_access_version_updated_at_not_null NOT NULL,
			CONSTRAINT sys_access_version_pkey PRIMARY KEY (user_id),
			CONSTRAINT ck_sys_access_version_version CHECK (version >= 1),
			CONSTRAINT fk_sys_access_version_user FOREIGN KEY (user_id) REFERENCES sys_user(id) ON DELETE RESTRICT
		)`,
		`CREATE TABLE sys_auth_platform (
			id BIGINT CONSTRAINT sys_auth_platform_id_not_null NOT NULL DEFAULT nextval('sys_auth_platform_id_seq'),
			code VARCHAR(64) CONSTRAINT sys_auth_platform_code_not_null NOT NULL,
			name VARCHAR(64) CONSTRAINT sys_auth_platform_name_not_null NOT NULL,
			policy_version BIGINT CONSTRAINT sys_auth_platform_policy_version_not_null NOT NULL,
			access_ttl_seconds INTEGER CONSTRAINT sys_auth_platform_access_ttl_seconds_not_null NOT NULL,
			refresh_ttl_seconds INTEGER CONSTRAINT sys_auth_platform_refresh_ttl_seconds_not_null NOT NULL,
			session_cache_ttl_seconds INTEGER CONSTRAINT sys_auth_platform_session_cache_ttl_seconds_not_null NOT NULL,
			access_cache_ttl_seconds INTEGER CONSTRAINT sys_auth_platform_access_cache_ttl_seconds_not_null NOT NULL,
			bind_device SMALLINT CONSTRAINT sys_auth_platform_bind_device_not_null NOT NULL,
			bind_ip SMALLINT CONSTRAINT sys_auth_platform_bind_ip_not_null NOT NULL,
			max_sessions INTEGER CONSTRAINT sys_auth_platform_max_sessions_not_null NOT NULL,
			allow_register SMALLINT CONSTRAINT sys_auth_platform_allow_register_not_null NOT NULL,
			is_enabled SMALLINT CONSTRAINT sys_auth_platform_is_enabled_not_null NOT NULL,
			is_builtin SMALLINT CONSTRAINT sys_auth_platform_is_builtin_not_null NOT NULL,
			created_at TIMESTAMPTZ CONSTRAINT sys_auth_platform_created_at_not_null NOT NULL,
			updated_at TIMESTAMPTZ CONSTRAINT sys_auth_platform_updated_at_not_null NOT NULL,
			deleted_at TIMESTAMPTZ,
			CONSTRAINT sys_auth_platform_pkey PRIMARY KEY (id),
			CONSTRAINT ck_sys_auth_platform_code CHECK (code <> ''),
			CONSTRAINT ck_sys_auth_platform_policy_version CHECK (policy_version >= 1),
			CONSTRAINT ck_sys_auth_platform_access_ttl_seconds CHECK (access_ttl_seconds > 0),
			CONSTRAINT ck_sys_auth_platform_refresh_ttl_seconds CHECK (refresh_ttl_seconds > 0),
			CONSTRAINT ck_sys_auth_platform_session_cache_ttl_seconds CHECK (session_cache_ttl_seconds > 0),
			CONSTRAINT ck_sys_auth_platform_access_cache_ttl_seconds CHECK (access_cache_ttl_seconds > 0),
			CONSTRAINT ck_sys_auth_platform_bind_device CHECK (bind_device IN (0, 1)),
			CONSTRAINT ck_sys_auth_platform_bind_ip CHECK (bind_ip IN (0, 1)),
			CONSTRAINT ck_sys_auth_platform_max_sessions CHECK (max_sessions > 0),
			CONSTRAINT ck_sys_auth_platform_allow_register CHECK (allow_register IN (0, 1)),
			CONSTRAINT ck_sys_auth_platform_is_enabled CHECK (is_enabled IN (0, 1)),
			CONSTRAINT ck_sys_auth_platform_is_builtin CHECK (is_builtin IN (0, 1))
		)`,
		`CREATE TABLE sys_operation_log (
			id BIGINT CONSTRAINT sys_operation_log_id_not_null NOT NULL DEFAULT nextval('sys_operation_log_id_seq'),
			event_id VARCHAR(64) CONSTRAINT sys_operation_log_event_id_not_null NOT NULL,
			request_id VARCHAR(64) CONSTRAINT sys_operation_log_request_id_not_null NOT NULL,
			user_id BIGINT,
			method VARCHAR(16) CONSTRAINT sys_operation_log_method_not_null NOT NULL,
			route VARCHAR(255) CONSTRAINT sys_operation_log_route_not_null NOT NULL,
			module VARCHAR(64) CONSTRAINT sys_operation_log_module_not_null NOT NULL,
			action VARCHAR(64) CONSTRAINT sys_operation_log_action_not_null NOT NULL,
			client_ip VARCHAR(64) CONSTRAINT sys_operation_log_client_ip_not_null NOT NULL,
			user_agent TEXT CONSTRAINT sys_operation_log_user_agent_not_null NOT NULL,
			status_code INTEGER CONSTRAINT sys_operation_log_status_code_not_null NOT NULL,
			is_success SMALLINT CONSTRAINT sys_operation_log_is_success_not_null NOT NULL,
			latency_ms BIGINT CONSTRAINT sys_operation_log_latency_ms_not_null NOT NULL,
			created_at TIMESTAMPTZ CONSTRAINT sys_operation_log_created_at_not_null NOT NULL,
			updated_at TIMESTAMPTZ CONSTRAINT sys_operation_log_updated_at_not_null NOT NULL,
			CONSTRAINT sys_operation_log_pkey PRIMARY KEY (id),
			CONSTRAINT ck_sys_operation_log_is_success CHECK (is_success IN (0, 1)),
			CONSTRAINT ck_sys_operation_log_latency_ms CHECK (latency_ms >= 0)
		)`,
	}
	for _, statement := range statements {
		mustExec(t, db, statement)
	}

	for _, statement := range []string{
		`ALTER SEQUENCE sys_user_id_seq OWNED BY sys_user.id`,
		`ALTER SEQUENCE sys_user_session_id_seq OWNED BY sys_user_session.id`,
		`ALTER SEQUENCE sys_menu_id_seq OWNED BY sys_menu.id`,
		`ALTER SEQUENCE sys_role_id_seq OWNED BY sys_role.id`,
		`ALTER SEQUENCE sys_user_role_id_seq OWNED BY sys_user_role.id`,
		`ALTER SEQUENCE sys_role_menu_id_seq OWNED BY sys_role_menu.id`,
		`ALTER SEQUENCE sys_access_version_user_id_seq OWNED BY sys_access_version.user_id`,
		`ALTER SEQUENCE sys_auth_platform_id_seq OWNED BY sys_auth_platform.id`,
		`ALTER SEQUENCE sys_operation_log_id_seq OWNED BY sys_operation_log.id`,
		`CREATE UNIQUE INDEX ux_sys_user_username_active ON sys_user (username) WHERE deleted_at IS NULL`,
		`CREATE UNIQUE INDEX ux_sys_user_email_active ON sys_user (email) WHERE deleted_at IS NULL`,
		`CREATE UNIQUE INDEX ux_sys_user_session_refresh_hash ON sys_user_session (refresh_token_hash)`,
		`CREATE INDEX ix_sys_user_session_user_created ON sys_user_session (user_id, created_at)`,
		`CREATE INDEX ix_sys_user_session_user_platform_active ON sys_user_session (user_id, platform) WHERE revoked_at IS NULL AND deleted_at IS NULL`,
		`CREATE UNIQUE INDEX ux_sys_user_session_current ON sys_user_session (user_id, platform, device_id) WHERE revoked_at IS NULL AND deleted_at IS NULL`,
		`CREATE UNIQUE INDEX ux_sys_menu_code_active ON sys_menu (code) WHERE deleted_at IS NULL`,
		`CREATE UNIQUE INDEX ux_sys_menu_page_path_active ON sys_menu (path) WHERE menu_type = 'page' AND deleted_at IS NULL`,
		`CREATE INDEX ix_sys_menu_parent_active ON sys_menu (parent_id) WHERE deleted_at IS NULL`,
		`CREATE UNIQUE INDEX ux_sys_role_code_active ON sys_role (code) WHERE deleted_at IS NULL`,
		`CREATE UNIQUE INDEX ux_sys_role_name_active ON sys_role (name) WHERE deleted_at IS NULL`,
		`CREATE UNIQUE INDEX ux_sys_role_default_active ON sys_role (is_default) WHERE is_default = 1 AND deleted_at IS NULL`,
		`CREATE UNIQUE INDEX ux_sys_user_role_active ON sys_user_role (user_id, role_id) WHERE deleted_at IS NULL`,
		`CREATE UNIQUE INDEX ux_sys_role_menu_active ON sys_role_menu (role_id, menu_id) WHERE deleted_at IS NULL`,
		`CREATE UNIQUE INDEX ux_sys_auth_platform_code_active ON sys_auth_platform (code) WHERE deleted_at IS NULL`,
		`CREATE UNIQUE INDEX ux_sys_operation_log_event_id ON sys_operation_log (event_id)`,
		`CREATE UNIQUE INDEX ux_sys_operation_log_request_id ON sys_operation_log (request_id)`,
		`CREATE INDEX ix_sys_operation_log_request_id ON sys_operation_log (request_id)`,
		`CREATE INDEX ix_sys_operation_log_created_at ON sys_operation_log (created_at)`,
		`CREATE INDEX ix_sys_operation_log_user_created ON sys_operation_log (user_id, created_at)`,
		`CREATE INDEX ix_sys_operation_log_action_created ON sys_operation_log (action, created_at)`,
		`INSERT INTO sys_user (id, username, email, password_hash, is_enabled, created_at, updated_at) VALUES (101, 'admin', 'admin@example.com', 'hash', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		`INSERT INTO sys_menu (id, menu_type, code, i18n_key, sort_order, is_enabled, is_hidden, created_at, updated_at) VALUES (301, 'page', 'system:user:list', 'menu.user', 1, 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		`INSERT INTO sys_operation_log (id, event_id, request_id, method, route, module, action, client_ip, user_agent, status_code, is_success, latency_ms, created_at, updated_at) VALUES (901, 'event-901', 'request-901', 'GET', '/api/v1/users', 'user', 'list', '127.0.0.1', 'test', 200, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
	} {
		mustExec(t, db, statement)
	}
}

func expectedConstraintRenames() []objectRename {
	renames := []objectRename{
		{old: "sys_user_pkey", new: "user_account_pkey"},
		{old: "sys_user_session_pkey", new: "auth_session_pkey"},
		{old: "sys_menu_pkey", new: "rbac_menu_pkey"},
		{old: "sys_role_pkey", new: "rbac_role_pkey"},
		{old: "sys_user_role_pkey", new: "rbac_user_role_pkey"},
		{old: "sys_role_menu_pkey", new: "rbac_role_menu_pkey"},
		{old: "sys_access_version_pkey", new: "rbac_access_version_pkey"},
		{old: "sys_auth_platform_pkey", new: "auth_platform_pkey"},
		{old: "sys_operation_log_pkey", new: "audit_operation_log_pkey"},
	}
	for _, rename := range []objectRename{
		{old: "ck_sys_user_is_enabled", new: "ck_user_account_is_enabled"},
		{old: "ck_sys_user_session_version", new: "ck_auth_session_version"},
		{old: "fk_sys_user_session_user", new: "fk_auth_session_user"},
		{old: "ck_sys_menu_type", new: "ck_rbac_menu_type"},
		{old: "ck_sys_menu_shape", new: "ck_rbac_menu_shape"},
		{old: "ck_sys_menu_render_shape", new: "ck_rbac_menu_render_shape"},
		{old: "ck_sys_menu_sort_order", new: "ck_rbac_menu_sort_order"},
		{old: "ck_sys_menu_is_enabled", new: "ck_rbac_menu_is_enabled"},
		{old: "ck_sys_menu_is_hidden", new: "ck_rbac_menu_is_hidden"},
		{old: "fk_sys_menu_parent", new: "fk_rbac_menu_parent"},
		{old: "ck_sys_role_is_default", new: "ck_rbac_role_is_default"},
		{old: "ck_sys_role_is_enabled", new: "ck_rbac_role_is_enabled"},
		{old: "fk_sys_user_role_user", new: "fk_rbac_user_role_user"},
		{old: "fk_sys_user_role_role", new: "fk_rbac_user_role_role"},
		{old: "fk_sys_role_menu_role", new: "fk_rbac_role_menu_role"},
		{old: "fk_sys_role_menu_menu", new: "fk_rbac_role_menu_menu"},
		{old: "ck_sys_access_version_version", new: "ck_rbac_access_version_version"},
		{old: "fk_sys_access_version_user", new: "fk_rbac_access_version_user"},
		{old: "ck_sys_auth_platform_code", new: "ck_auth_platform_code"},
		{old: "ck_sys_auth_platform_policy_version", new: "ck_auth_platform_policy_version"},
		{old: "ck_sys_auth_platform_access_ttl_seconds", new: "ck_auth_platform_access_ttl_seconds"},
		{old: "ck_sys_auth_platform_refresh_ttl_seconds", new: "ck_auth_platform_refresh_ttl_seconds"},
		{old: "ck_sys_auth_platform_session_cache_ttl_seconds", new: "ck_auth_platform_session_cache_ttl_seconds"},
		{old: "ck_sys_auth_platform_access_cache_ttl_seconds", new: "ck_auth_platform_access_cache_ttl_seconds"},
		{old: "ck_sys_auth_platform_bind_device", new: "ck_auth_platform_bind_device"},
		{old: "ck_sys_auth_platform_bind_ip", new: "ck_auth_platform_bind_ip"},
		{old: "ck_sys_auth_platform_max_sessions", new: "ck_auth_platform_max_sessions"},
		{old: "ck_sys_auth_platform_allow_register", new: "ck_auth_platform_allow_register"},
		{old: "ck_sys_auth_platform_is_enabled", new: "ck_auth_platform_is_enabled"},
		{old: "ck_sys_auth_platform_is_builtin", new: "ck_auth_platform_is_builtin"},
		{old: "ck_sys_operation_log_is_success", new: "ck_audit_operation_log_is_success"},
		{old: "ck_sys_operation_log_latency_ms", new: "ck_audit_operation_log_latency_ms"},
	} {
		renames = append(renames, rename)
	}
	columns := map[string][]string{
		"sys_user":           {"id", "username", "email", "password_hash", "is_enabled", "created_at", "updated_at"},
		"sys_user_session":   {"id", "user_id", "platform", "device_id", "refresh_token_hash", "version", "client_ip", "user_agent", "refresh_expires_at", "created_at", "updated_at"},
		"sys_menu":           {"id", "menu_type", "code", "i18n_key", "sort_order", "is_enabled", "is_hidden", "created_at", "updated_at"},
		"sys_role":           {"id", "code", "name", "is_default", "is_enabled", "created_at", "updated_at"},
		"sys_user_role":      {"id", "user_id", "role_id", "created_at", "updated_at"},
		"sys_role_menu":      {"id", "role_id", "menu_id", "created_at", "updated_at"},
		"sys_access_version": {"user_id", "version", "created_at", "updated_at"},
		"sys_auth_platform":  {"id", "code", "name", "policy_version", "access_ttl_seconds", "refresh_ttl_seconds", "session_cache_ttl_seconds", "access_cache_ttl_seconds", "bind_device", "bind_ip", "max_sessions", "allow_register", "is_enabled", "is_builtin", "created_at", "updated_at"},
		"sys_operation_log":  {"id", "event_id", "request_id", "method", "route", "module", "action", "client_ip", "user_agent", "status_code", "is_success", "latency_ms", "created_at", "updated_at"},
	}
	newPrefixes := map[string]string{
		"sys_user": "user_account", "sys_user_session": "auth_session", "sys_menu": "rbac_menu",
		"sys_role": "rbac_role", "sys_user_role": "rbac_user_role", "sys_role_menu": "rbac_role_menu",
		"sys_access_version": "rbac_access_version", "sys_auth_platform": "auth_platform",
		"sys_operation_log": "audit_operation_log",
	}
	for oldPrefix, tableColumns := range columns {
		for _, column := range tableColumns {
			renames = append(renames, objectRename{
				old: fmt.Sprintf("%s_%s_not_null", oldPrefix, column),
				new: fmt.Sprintf("%s_%s_not_null", newPrefixes[oldPrefix], column),
			})
		}
	}
	return renames
}

func assertCurrentDomainSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, rename := range expectedTableRenames {
		assertRelationMissing(t, db, rename.old)
		assertRelationExists(t, db, rename.new)
	}
	for _, rename := range expectedSequenceRenames {
		assertRelationMissing(t, db, rename.old)
		assertRelationExists(t, db, rename.new)
	}
	for _, rename := range expectedIndexRenames {
		assertRelationMissing(t, db, rename.old)
		assertRelationExists(t, db, rename.new)
	}
	for _, rename := range expectedConstraintRenames() {
		assertConstraintMissing(t, db, rename.old)
		assertConstraintExists(t, db, rename.new)
	}
}

func assertNoOwnedObjectPrefix(t *testing.T, db *gorm.DB, prefix string) {
	t.Helper()
	var names []string
	if err := db.Raw(`
		SELECT relname AS name FROM pg_class
		WHERE relnamespace = current_schema()::regnamespace AND relname LIKE ?
		UNION ALL
		SELECT conname AS name FROM pg_constraint
		WHERE connamespace = current_schema()::regnamespace AND conname LIKE ?
		ORDER BY name`, prefix+"%", prefix+"%").Scan(&names).Error; err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Fatalf("legacy owned objects remain: %v", names)
	}
}

func assertRelationExists(t *testing.T, db *gorm.DB, name string) {
	t.Helper()
	if !relationExists(t, db, name) {
		t.Fatalf("relation %s does not exist", name)
	}
}

func assertRelationMissing(t *testing.T, db *gorm.DB, name string) {
	t.Helper()
	if relationExists(t, db, name) {
		t.Fatalf("relation %s still exists", name)
	}
}

func relationExists(t *testing.T, db *gorm.DB, name string) bool {
	t.Helper()
	var exists bool
	if err := db.Raw(`SELECT to_regclass(current_schema() || '.' || ?) IS NOT NULL`, name).Scan(&exists).Error; err != nil {
		t.Fatalf("inspect relation %s: %v", name, err)
	}
	return exists
}

func assertConstraintExists(t *testing.T, db *gorm.DB, name string) {
	t.Helper()
	if !constraintExists(t, db, name) {
		t.Fatalf("constraint %s does not exist", name)
	}
}

func assertConstraintMissing(t *testing.T, db *gorm.DB, name string) {
	t.Helper()
	if constraintExists(t, db, name) {
		t.Fatalf("constraint %s still exists", name)
	}
}

func constraintExists(t *testing.T, db *gorm.DB, name string) bool {
	t.Helper()
	var exists bool
	if err := db.Raw(`SELECT EXISTS (
		SELECT 1 FROM pg_constraint
		WHERE connamespace = current_schema()::regnamespace AND conname = ?
	)`, name).Scan(&exists).Error; err != nil {
		t.Fatalf("inspect constraint %s: %v", name, err)
	}
	return exists
}

func assertRowPreserved(t *testing.T, db *gorm.DB, tableName string, id int64) {
	t.Helper()
	var count int64
	if err := db.Table(tableName).Where("id = ?", id).Count(&count).Error; err != nil {
		t.Fatalf("inspect row %s.%d: %v", tableName, id, err)
	}
	if count != 1 {
		t.Fatalf("row %s.%d count = %d, want 1", tableName, id, count)
	}
}

func mustExec(t *testing.T, db *gorm.DB, statement string) {
	t.Helper()
	if err := db.Exec(statement).Error; err != nil {
		t.Fatalf("execute %q: %v", statement, err)
	}
}
