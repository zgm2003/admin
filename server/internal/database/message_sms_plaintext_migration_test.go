package database_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"admin/server/internal/database/testschema"
)

func TestMessageSMSPlaintextMigrationRemovesPhoneCiphertextAndUsesNumericRules(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_message_sms_plaintext")
	for _, statement := range []string{
		`CREATE TABLE system_config_cache_generation(namespace varchar(128) NOT NULL, scope_key varchar(128) NOT NULL, generation bigint NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(namespace,scope_key))`,
		`CREATE TABLE system_config_cache_outbox(id bigserial PRIMARY KEY, namespace varchar(128) NOT NULL, scope_key varchar(128) NOT NULL, generation bigint NOT NULL, attempts integer NOT NULL DEFAULT 0, available_at timestamptz NOT NULL DEFAULT now(), locked_until timestamptz, lock_token varchar(64), last_error varchar(512) NOT NULL DEFAULT '', published_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(namespace,scope_key,generation), FOREIGN KEY(namespace,scope_key) REFERENCES system_config_cache_generation(namespace,scope_key))`,
		`INSERT INTO system_config_cache_generation(namespace,scope_key,generation) VALUES ('message.sms','global',1)`,
		`CREATE TABLE message_sms_recipient_rule(id bigserial PRIMARY KEY, scope varchar(16) NOT NULL CHECK(scope IN ('phone','prefix')), pattern_ciphertext text NOT NULL, pattern_hint varchar(64) NOT NULL, pattern_hmac varchar(128) NOT NULL, action varchar(16) NOT NULL CHECK(action IN ('allow','deny')), name varchar(128) NOT NULL, remark varchar(512) NOT NULL DEFAULT '', is_enabled smallint NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz)`,
		`CREATE UNIQUE INDEX ux_message_sms_recipient_rule_pattern_action_active ON message_sms_recipient_rule(scope,pattern_hmac,action) WHERE deleted_at IS NULL`,
		`CREATE TABLE message_sms_log(id bigserial PRIMARY KEY, platform_id bigint NOT NULL, challenge_id varchar(128), user_id bigint, scene varchar(32) NOT NULL, template_id bigint NOT NULL, to_phone_ciphertext text NOT NULL, to_phone_hint varchar(32) NOT NULL, to_phone_hmac varchar(128) NOT NULL, status smallint NOT NULL, request_id varchar(128) NOT NULL DEFAULT '', serial_no varchar(128) NOT NULL DEFAULT '', fee integer NOT NULL DEFAULT 0, error_code varchar(128) NOT NULL DEFAULT '', error_summary varchar(512) NOT NULL DEFAULT '', latency_ms bigint NOT NULL DEFAULT 0, sent_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now())`,
		`CREATE INDEX ix_message_sms_log_to_phone_hmac_id_desc ON message_sms_log(to_phone_hmac,id DESC)`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	script := readMessageSMSPlaintextMigration(t)
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("migration: %v", err)
	}
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("rerun: %v", err)
	}
	var generation int64
	if err := db.WithContext(ctx).Raw(`SELECT generation FROM system_config_cache_generation WHERE namespace='message.sms' AND scope_key='global'`).Scan(&generation).Error; err != nil {
		t.Fatal(err)
	}
	if generation != 2 {
		t.Fatalf("generation=%d", generation)
	}
	var outbox int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM system_config_cache_outbox WHERE namespace='message.sms' AND scope_key='global'`).Scan(&outbox).Error; err != nil || outbox != 1 {
		t.Fatalf("outbox=%d err=%v", outbox, err)
	}
	var columns int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name IN ('message_sms_recipient_rule','message_sms_log') AND column_name IN ('pattern_ciphertext','pattern_hint','pattern_hmac','to_phone_ciphertext','to_phone_hint','to_phone_hmac')`).Scan(&columns).Error; err != nil || columns != 0 {
		t.Fatalf("legacy columns=%d err=%v", columns, err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO message_sms_recipient_rule(scope,pattern,action,name) VALUES (0,'+8615671628271',0,'deny')`).Error; err != nil {
		t.Fatalf("plain rule insert: %v", err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO message_sms_log(platform_id,scene,template_id,to_phone,status) VALUES (1,'login',1,'+8615671628271',1)`).Error; err != nil {
		t.Fatalf("plain log insert: %v", err)
	}
}

func TestMessageSMSPlaintextMigrationRejectsUnrecoverableCiphertext(t *testing.T) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_message_sms_plaintext_reject")
	for _, statement := range []string{
		`CREATE TABLE system_config_cache_generation(namespace varchar(128) NOT NULL, scope_key varchar(128) NOT NULL, generation bigint NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(namespace,scope_key))`,
		`CREATE TABLE system_config_cache_outbox(id bigserial PRIMARY KEY, namespace varchar(128) NOT NULL, scope_key varchar(128) NOT NULL, generation bigint NOT NULL, attempts integer NOT NULL DEFAULT 0, available_at timestamptz NOT NULL DEFAULT now(), locked_until timestamptz, lock_token varchar(64), last_error varchar(512) NOT NULL DEFAULT '', published_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(namespace,scope_key,generation), FOREIGN KEY(namespace,scope_key) REFERENCES system_config_cache_generation(namespace,scope_key))`,
		`INSERT INTO system_config_cache_generation(namespace,scope_key,generation) VALUES ('message.sms','global',1)`,
		`CREATE TABLE message_sms_recipient_rule(id bigserial PRIMARY KEY, scope varchar(16) NOT NULL, pattern_ciphertext text NOT NULL, pattern_hint varchar(64) NOT NULL, pattern_hmac varchar(128) NOT NULL, action varchar(16) NOT NULL, name varchar(128) NOT NULL, remark varchar(512) NOT NULL DEFAULT '', is_enabled smallint NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz)`,
		`CREATE TABLE message_sms_log(id bigserial PRIMARY KEY, platform_id bigint NOT NULL, scene varchar(32) NOT NULL, template_id bigint NOT NULL, to_phone_ciphertext text NOT NULL, to_phone_hint varchar(32) NOT NULL, to_phone_hmac varchar(128) NOT NULL, status smallint NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now())`,
		`INSERT INTO message_sms_recipient_rule(scope,pattern_ciphertext,pattern_hint,pattern_hmac,action,name) VALUES ('phone','sms:v1:unknown','156****8271','hmac','deny','legacy')`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	err := db.WithContext(ctx).Exec(readMessageSMSPlaintextMigration(t)).Error
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unrecoverable") {
		t.Fatalf("error=%v", err)
	}
	var typ string
	if err := db.WithContext(ctx).Raw(`SELECT data_type FROM information_schema.columns WHERE table_name='message_sms_recipient_rule' AND column_name='scope'`).Scan(&typ).Error; err != nil {
		t.Fatal(err)
	}
	if typ != "character varying" {
		t.Fatalf("failed migration changed scope=%q", typ)
	}
}

func readMessageSMSPlaintextMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-10-08-message-sms-plaintext.sql"))
	if err != nil {
		t.Fatalf("read SMS plaintext migration: %v", err)
	}
	return string(data)
}
