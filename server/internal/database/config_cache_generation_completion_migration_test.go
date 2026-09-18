package database_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"admin/server/internal/database/testschema"
	"gorm.io/gorm"
)

var completionScopes = []string{"system.dictionary", "message.mail", "message.sms"}

func TestConfigCacheGenerationCompletionMigrationIsAtomicAndIdempotent(t *testing.T) {
	db, ctx := openConfigCacheGenerationCompletionSchema(t, "success")
	beforePolicies := readCompletionPolicyFacts(t, db, ctx)
	beforeSecurity := readCompletionSecurityFacts(t, db, ctx)
	script := readConfigCacheGenerationCompletionMigration(t)

	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("execute completion migration: %v", err)
	}
	assertCompletionResult(t, db, ctx, beforePolicies, beforeSecurity)
	beforeGeneration := readCompletionGenerationFacts(t, db, ctx)

	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("re-execute completion migration: %v", err)
	}
	assertCompletionResult(t, db, ctx, beforePolicies, beforeSecurity)
	if after := readCompletionGenerationFacts(t, db, ctx); after != beforeGeneration {
		t.Fatalf("generation facts changed on re-run:\nbefore=%s\nafter=%s", beforeGeneration, after)
	}
}

func TestConfigCacheGenerationCompletionMigrationRejectsIncompletePolicyCatalogs(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{name: "mail", query: `DELETE FROM message_mail_rate_limit_policy WHERE platform_id=2 AND policy_key='business_email_10m'`},
		{name: "sms", query: `DELETE FROM message_sms_rate_limit_policy WHERE platform_id=2 AND policy_key='business_phone_10m'`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, ctx := openConfigCacheGenerationCompletionSchema(t, "missing_"+test.name)
			if err := db.WithContext(ctx).Exec(test.query).Error; err != nil {
				t.Fatal(err)
			}
			assertCompletionMigrationRejected(t, db, ctx, "must contain exactly")
		})
	}
}

func TestConfigCacheGenerationCompletionMigrationRejectsInvalidOrDuplicatePolicyKeys(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{
			name: "mail_duplicate",
			query: `ALTER TABLE message_mail_rate_limit_policy DROP CONSTRAINT pk_message_mail_rate_limit_policy;
INSERT INTO message_mail_rate_limit_policy SELECT platform_id, policy_key, mode, dimension, limit_count, window_seconds, revision, created_at, updated_at
FROM message_mail_rate_limit_policy WHERE platform_id=1 AND policy_key='business_email_minute'`,
		},
		{
			name: "sms_duplicate",
			query: `ALTER TABLE message_sms_rate_limit_policy DROP CONSTRAINT pk_message_sms_rate_limit_policy;
INSERT INTO message_sms_rate_limit_policy SELECT platform_id, policy_key, mode, dimension, limit_count, window_seconds, revision, created_at, updated_at
FROM message_sms_rate_limit_policy WHERE platform_id=1 AND policy_key='business_phone_minute'`,
		},
		{name: "mail_invalid", query: `UPDATE message_mail_rate_limit_policy SET policy_key='custom' WHERE platform_id=1 AND policy_key='business_email_10m'`},
		{name: "sms_invalid", query: `UPDATE message_sms_rate_limit_policy SET policy_key='custom' WHERE platform_id=1 AND policy_key='business_phone_10m'`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, ctx := openConfigCacheGenerationCompletionSchema(t, test.name)
			if err := db.WithContext(ctx).Exec(test.query).Error; err != nil {
				t.Fatal(err)
			}
			assertCompletionMigrationRejected(t, db, ctx, "must contain exactly")
		})
	}
}

func TestConfigCacheGenerationCompletionMigrationRejectsInvalidPolicyValues(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{name: "mail", query: `UPDATE message_mail_rate_limit_policy SET limit_count=0 WHERE platform_id=1 AND policy_key='business_email_minute'`},
		{name: "sms", query: `UPDATE message_sms_rate_limit_policy SET window_seconds=0 WHERE platform_id=1 AND policy_key='business_phone_minute'`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, ctx := openConfigCacheGenerationCompletionSchema(t, "value_"+test.name)
			if err := db.WithContext(ctx).Exec(test.query).Error; err != nil {
				t.Fatal(err)
			}
			assertCompletionMigrationRejected(t, db, ctx, "invalid values")
		})
	}
}

func TestConfigCacheGenerationCompletionMigrationRequiresGenerationTables(t *testing.T) {
	db, ctx := openConfigCacheGenerationCompletionSchema(t, "missing_generation")
	beforePolicies := readCompletionPolicyFacts(t, db, ctx)
	beforeSecurity := readCompletionSecurityFacts(t, db, ctx)
	if err := db.WithContext(ctx).Exec(`DROP TABLE system_config_cache_outbox; DROP TABLE system_config_cache_generation`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(readConfigCacheGenerationCompletionMigration(t)).Error; err == nil {
		t.Fatal("completion migration succeeded without generation tables")
	}
	assertCompletionRevisionColumnsPresent(t, db, ctx)
	if after := readCompletionPolicyFacts(t, db, ctx); after != beforePolicies {
		t.Fatalf("policy facts changed after rejected migration:\nbefore=%s\nafter=%s", beforePolicies, after)
	}
	if after := readCompletionSecurityFacts(t, db, ctx); after != beforeSecurity {
		t.Fatalf("security facts changed after rejected migration:\nbefore=%s\nafter=%s", beforeSecurity, after)
	}
}

func assertCompletionMigrationRejected(t *testing.T, db *gorm.DB, ctx context.Context, message string) {
	t.Helper()
	beforePolicies := readCompletionPolicyFacts(t, db, ctx)
	beforeSecurity := readCompletionSecurityFacts(t, db, ctx)
	beforeGeneration := readCompletionGenerationFacts(t, db, ctx)
	err := db.WithContext(ctx).Exec(readConfigCacheGenerationCompletionMigration(t)).Error
	if err == nil {
		t.Fatal("completion migration accepted invalid policy facts")
	}
	if !strings.Contains(err.Error(), message) {
		t.Fatalf("migration error=%v want contains %q", err, message)
	}
	assertCompletionRevisionColumnsPresent(t, db, ctx)
	if after := readCompletionPolicyFacts(t, db, ctx); after != beforePolicies {
		t.Fatalf("policy facts changed after rejected migration:\nbefore=%s\nafter=%s", beforePolicies, after)
	}
	if after := readCompletionSecurityFacts(t, db, ctx); after != beforeSecurity {
		t.Fatalf("security facts changed after rejected migration:\nbefore=%s\nafter=%s", beforeSecurity, after)
	}
	if after := readCompletionGenerationFacts(t, db, ctx); after != beforeGeneration {
		t.Fatalf("generation facts changed after rejected migration:\nbefore=%s\nafter=%s", beforeGeneration, after)
	}
}

func assertCompletionResult(t *testing.T, db *gorm.DB, ctx context.Context, beforePolicies, beforeSecurity string) {
	t.Helper()
	for _, table := range []string{"message_mail_rate_limit_policy", "message_sms_rate_limit_policy"} {
		assertCompletionColumnMissing(t, db, ctx, table, "revision")
	}
	for _, index := range []string{"ix_message_mail_rate_limit_policy_platform_revision", "ix_message_sms_rate_limit_policy_platform_revision"} {
		var exists bool
		if err := db.WithContext(ctx).Raw(`SELECT to_regclass(?) IS NOT NULL`, index).Scan(&exists).Error; err != nil {
			t.Fatal(err)
		}
		if exists {
			t.Fatalf("revision index %s still exists", index)
		}
	}
	for _, namespace := range completionScopes {
		var row struct {
			Generation int64
			Outbox     int64
		}
		if err := db.WithContext(ctx).Raw(`
SELECT generation.generation,
       (SELECT count(*) FROM system_config_cache_outbox AS outbox
         WHERE outbox.namespace=generation.namespace AND outbox.scope_key=generation.scope_key
           AND outbox.generation=generation.generation) AS outbox
FROM system_config_cache_generation AS generation
WHERE generation.namespace=? AND generation.scope_key='global'`, namespace).Scan(&row).Error; err != nil {
			t.Fatal(err)
		}
		if row.Generation != 1 || row.Outbox != 1 {
			t.Fatalf("scope %s/global generation=%d outbox=%d want 1/1", namespace, row.Generation, row.Outbox)
		}
	}
	if after := readCompletionPolicyFacts(t, db, ctx); after != beforePolicies {
		t.Fatalf("policy facts changed:\nbefore=%s\nafter=%s", beforePolicies, after)
	}
	if after := readCompletionSecurityFacts(t, db, ctx); after != beforeSecurity {
		t.Fatalf("security facts changed:\nbefore=%s\nafter=%s", beforeSecurity, after)
	}
}

func assertCompletionRevisionColumnsPresent(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	for _, table := range []string{"message_mail_rate_limit_policy", "message_sms_rate_limit_policy"} {
		var exists bool
		if err := db.WithContext(ctx).Raw(`SELECT EXISTS (
SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=? AND column_name='revision')`, table).Scan(&exists).Error; err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("revision column was partially removed from %s", table)
		}
	}
}

func assertCompletionColumnMissing(t *testing.T, db *gorm.DB, ctx context.Context, table, column string) {
	t.Helper()
	var exists bool
	if err := db.WithContext(ctx).Raw(`SELECT EXISTS (
SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=? AND column_name=?)`, table, column).Scan(&exists).Error; err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatalf("column %s.%s still exists", table, column)
	}
}

func readCompletionPolicyFacts(t *testing.T, db *gorm.DB, ctx context.Context) string {
	t.Helper()
	var facts string
	if err := db.WithContext(ctx).Raw(`
SELECT concat(
  (SELECT string_agg(concat_ws('|',platform_id,policy_key,mode,dimension,limit_count,window_seconds,created_at,updated_at),';' ORDER BY platform_id,policy_key,ctid)
     FROM message_mail_rate_limit_policy), '#',
  (SELECT string_agg(concat_ws('|',platform_id,policy_key,mode,dimension,limit_count,window_seconds,created_at,updated_at),';' ORDER BY platform_id,policy_key,ctid)
     FROM message_sms_rate_limit_policy))`).Scan(&facts).Error; err != nil {
		t.Fatal(err)
	}
	return facts
}

func readCompletionSecurityFacts(t *testing.T, db *gorm.DB, ctx context.Context) string {
	t.Helper()
	var facts string
	if err := db.WithContext(ctx).Raw(`
SELECT concat(
  (SELECT string_agg(id || ':' || menu_version || ':' || policy_version, ',' ORDER BY id) FROM permission_auth_platform), '#',
  (SELECT string_agg(user_id || ':' || version, ',' ORDER BY user_id) FROM permission_access_version), '#',
  (SELECT string_agg(id || ':' || version, ',' ORDER BY id) FROM user_session))`).Scan(&facts).Error; err != nil {
		t.Fatal(err)
	}
	return facts
}

func readCompletionGenerationFacts(t *testing.T, db *gorm.DB, ctx context.Context) string {
	t.Helper()
	var facts string
	if err := db.WithContext(ctx).Raw(`
SELECT concat(
  (SELECT string_agg(namespace || '/' || scope_key || ':' || generation, ',' ORDER BY namespace,scope_key) FROM system_config_cache_generation), '#',
  (SELECT string_agg(namespace || '/' || scope_key || ':' || generation || ':' || (published_at IS NOT NULL)::int, ',' ORDER BY id) FROM system_config_cache_outbox))`).Scan(&facts).Error; err != nil {
		t.Fatal(err)
	}
	return facts
}

func readConfigCacheGenerationCompletionMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-09-17-config-cache-generation-completion.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func openConfigCacheGenerationCompletionSchema(t *testing.T, suffix string) (*gorm.DB, context.Context) {
	t.Helper()
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_config_cache_generation_completion_"+suffix)
	statements := []string{
		`CREATE TABLE permission_auth_platform (
 id bigint PRIMARY KEY, code varchar(49) NOT NULL, is_enabled smallint NOT NULL, menu_version bigint NOT NULL,
 policy_version bigint NOT NULL, deleted_at timestamptz)`,
		`CREATE TABLE permission_access_version (user_id bigint PRIMARY KEY, version bigint NOT NULL, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL)`,
		`CREATE TABLE user_session (id bigint PRIMARY KEY, user_id bigint NOT NULL, version bigint NOT NULL, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL)`,
		`CREATE TABLE system_dictionary (id bigint PRIMARY KEY, code varchar(128) NOT NULL, updated_at timestamptz NOT NULL)`,
		`CREATE TABLE message_mail_rate_limit_policy (
 platform_id bigint NOT NULL, policy_key varchar(64) NOT NULL, mode varchar(16) NOT NULL, dimension varchar(64) NOT NULL,
 limit_count integer NOT NULL, window_seconds integer NOT NULL, revision bigint NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 CONSTRAINT pk_message_mail_rate_limit_policy PRIMARY KEY(platform_id,policy_key),
 CONSTRAINT ck_message_mail_rate_limit_policy_revision CHECK(revision >= 1))`,
		`CREATE INDEX ix_message_mail_rate_limit_policy_platform_revision ON message_mail_rate_limit_policy(platform_id,revision)`,
		`CREATE TABLE message_sms_rate_limit_policy (
 platform_id bigint NOT NULL, policy_key varchar(64) NOT NULL, mode varchar(16) NOT NULL, dimension varchar(64) NOT NULL,
 limit_count integer NOT NULL, window_seconds integer NOT NULL, revision bigint NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 CONSTRAINT pk_message_sms_rate_limit_policy PRIMARY KEY(platform_id,policy_key),
 CONSTRAINT ck_message_sms_rate_limit_policy_revision CHECK(revision >= 1))`,
		`CREATE INDEX ix_message_sms_rate_limit_policy_platform_revision ON message_sms_rate_limit_policy(platform_id,revision)`,
		`CREATE TABLE system_config_cache_generation (
 namespace varchar(128) NOT NULL, scope_key varchar(128) NOT NULL, generation bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
 CONSTRAINT pk_system_config_cache_generation PRIMARY KEY(namespace,scope_key))`,
		`CREATE TABLE system_config_cache_outbox (
 id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY, namespace varchar(128) NOT NULL, scope_key varchar(128) NOT NULL,
 generation bigint NOT NULL, attempts integer NOT NULL DEFAULT 0, available_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
 locked_until timestamptz, lock_token varchar(64), last_error varchar(512) NOT NULL DEFAULT '', published_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
 CONSTRAINT uq_system_config_cache_outbox_generation UNIQUE(namespace,scope_key,generation),
 CONSTRAINT fk_system_config_cache_outbox_generation FOREIGN KEY(namespace,scope_key) REFERENCES system_config_cache_generation(namespace,scope_key))`,
		`INSERT INTO permission_auth_platform(id,code,is_enabled,menu_version,policy_version) VALUES (1,'admin',1,7,11),(2,'canvas',1,3,5)`,
		`INSERT INTO permission_access_version(user_id,version,created_at,updated_at) VALUES (10,13,'2026-09-01 00:00:00+00','2026-09-02 00:00:00+00')`,
		`INSERT INTO user_session(id,user_id,version,created_at,updated_at) VALUES (20,10,17,'2026-09-01 00:00:00+00','2026-09-03 00:00:00+00')`,
		`INSERT INTO system_dictionary(id,code,updated_at) VALUES (1,'user.gender','2026-09-04 00:00:00+00')`,
		`INSERT INTO message_mail_rate_limit_policy VALUES
 (1,'business_email_minute','business','platform_email',1,60,2,'2026-09-01 01:00:00+00','2026-09-02 01:00:00+00'),
 (1,'business_email_10m','business','platform_email',5,600,3,'2026-09-01 02:00:00+00','2026-09-02 02:00:00+00'),
 (2,'business_email_minute','business','platform_email',2,60,4,'2026-09-01 03:00:00+00','2026-09-02 03:00:00+00'),
 (2,'business_email_10m','business','platform_email',8,600,5,'2026-09-01 04:00:00+00','2026-09-02 04:00:00+00')`,
		`INSERT INTO message_sms_rate_limit_policy VALUES
 (1,'business_phone_minute','business','platform_phone',1,60,2,'2026-09-05 01:00:00+00','2026-09-06 01:00:00+00'),
 (1,'business_phone_10m','business','platform_phone',5,600,3,'2026-09-05 02:00:00+00','2026-09-06 02:00:00+00'),
 (2,'business_phone_minute','business','platform_phone',2,60,4,'2026-09-05 03:00:00+00','2026-09-06 03:00:00+00'),
 (2,'business_phone_10m','business','platform_phone',8,600,5,'2026-09-05 04:00:00+00','2026-09-06 04:00:00+00')`,
		`INSERT INTO system_config_cache_generation(namespace,scope_key,generation) VALUES ('system.setting','global',4)`,
		`INSERT INTO system_config_cache_outbox(namespace,scope_key,generation,published_at) VALUES ('system.setting','global',2,'2026-09-07 00:00:00+00')`,
	}
	for _, statement := range statements {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("prepare completion fixture: %v", err)
		}
	}
	return db, ctx
}
