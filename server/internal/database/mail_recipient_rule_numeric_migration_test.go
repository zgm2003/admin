package database_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"admin/server/internal/database/testschema"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

func mailRuleNumericFixture(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_mail_rule_numeric")
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE message_mail_recipient_rule (
 id bigserial PRIMARY KEY, scope varchar(16) NOT NULL, pattern varchar(254) NOT NULL,
 action varchar(16) NOT NULL, name varchar(128) NOT NULL, remark varchar(512) NOT NULL DEFAULT '',
 is_enabled smallint NOT NULL DEFAULT 1 CHECK(is_enabled IN (0,1)),
 created_at timestamptz NOT NULL DEFAULT '2026-01-01T00:00:00Z',
 updated_at timestamptz NOT NULL DEFAULT '2026-01-02T00:00:00Z', deleted_at timestamptz,
 CONSTRAINT message_mail_recipient_rule_scope_check CHECK(scope IN ('email','domain')),
 CONSTRAINT message_mail_recipient_rule_action_check CHECK(action IN ('allow','deny')));
CREATE UNIQUE INDEX ux_message_mail_rule_scope_pattern_action_active
 ON message_mail_recipient_rule(scope,pattern,action) WHERE deleted_at IS NULL;
CREATE TABLE system_config_cache_generation (
 namespace varchar(128) NOT NULL, scope_key varchar(128) NOT NULL, generation bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(namespace,scope_key));
CREATE TABLE system_config_cache_outbox (
 id bigserial PRIMARY KEY, namespace varchar(128) NOT NULL, scope_key varchar(128) NOT NULL,
 generation bigint NOT NULL, attempts integer NOT NULL DEFAULT 0,
 available_at timestamptz NOT NULL, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 published_at timestamptz, UNIQUE(namespace,scope_key,generation));
INSERT INTO system_config_cache_generation(namespace,scope_key,generation)
 VALUES ('message.mail','global',3),('message.sms','global',7),('system.setting','global',17);
INSERT INTO message_mail_recipient_rule(scope,pattern,action,name,remark,is_enabled,deleted_at) VALUES
 ('email','user@example.com','allow','email allow','keep details',1,NULL),
 ('email','user@example.com','deny','email deny','',0,NULL),
 ('domain','example.com','allow','domain allow','',1,NULL),
 ('domain','example.com','deny','domain deny','',1,NULL),
 ('email','user@example.com','deny','deleted duplicate','preserve deletion',1,'2026-01-03T00:00:00Z');
`).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func mailRuleNumericSQL(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-10-08-mail-recipient-rule-numeric.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func mailRuleNumericSnapshot(t *testing.T, db *gorm.DB, ctx context.Context) string {
	t.Helper()
	var result string
	if err := db.WithContext(ctx).Raw(`SELECT jsonb_build_object(
 'rules',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM message_mail_recipient_rule r),
 'generation',(SELECT jsonb_agg(to_jsonb(g) ORDER BY namespace,scope_key) FROM system_config_cache_generation g),
 'outbox',(SELECT jsonb_agg(to_jsonb(o) ORDER BY id) FROM system_config_cache_outbox o),
 'types',(SELECT jsonb_agg(format_type(atttypid,atttypmod) ORDER BY attnum) FROM pg_attribute WHERE attrelid='message_mail_recipient_rule'::regclass AND attnum>0 AND NOT attisdropped),
 'checks',(SELECT jsonb_agg(pg_get_constraintdef(oid) ORDER BY conname) FROM pg_constraint WHERE conrelid='message_mail_recipient_rule'::regclass))::text`).Scan(&result).Error; err != nil {
		t.Fatal(err)
	}
	return result
}

func TestMailRuleNumericMigrationPreservesRulesAndAdvancesOnlyMailOnce(t *testing.T) {
	db, ctx := mailRuleNumericFixture(t)
	var facts string
	const rowFacts = `SELECT jsonb_agg(to_jsonb(r)-'scope'-'action' ORDER BY id)::text FROM message_mail_recipient_rule r`
	if err := db.WithContext(ctx).Raw(rowFacts).Scan(&facts).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(mailRuleNumericSQL(t)).Error; err != nil {
		t.Fatal(err)
	}
	var valid bool
	if err := db.WithContext(ctx).Raw(`SELECT
 (SELECT count(*)=2 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='message_mail_recipient_rule' AND column_name IN ('scope','action') AND data_type='smallint' AND is_nullable='NO' AND column_default IS NULL) AND
 (SELECT jsonb_agg(jsonb_build_array(scope,action) ORDER BY id)='[[0,1],[0,0],[1,1],[1,0],[0,0]]'::jsonb FROM message_mail_recipient_rule) AND
 (SELECT generation=4 FROM system_config_cache_generation WHERE namespace='message.mail') AND
 (SELECT generation=7 FROM system_config_cache_generation WHERE namespace='message.sms') AND
 (SELECT generation=17 FROM system_config_cache_generation WHERE namespace='system.setting') AND
 (SELECT count(*)=1 FROM system_config_cache_outbox) AND
 (SELECT count(*)=1 FROM system_config_cache_outbox WHERE namespace='message.mail' AND scope_key='global' AND generation=4)
`).Scan(&valid).Error; err != nil || !valid {
		t.Fatalf("invalid numeric migration valid=%v error=%v", valid, err)
	}
	var afterFacts string
	if err := db.WithContext(ctx).Raw(rowFacts).Scan(&afterFacts).Error; err != nil || afterFacts != facts {
		t.Fatalf("non-enum rule facts changed: %v", err)
	}
	before := mailRuleNumericSnapshot(t, db, ctx)
	if err := db.WithContext(ctx).Exec(mailRuleNumericSQL(t)).Error; err != nil {
		t.Fatal(err)
	}
	if mailRuleNumericSnapshot(t, db, ctx) != before {
		t.Fatal("repeat migration changed facts")
	}

	for _, query := range []string{
		`INSERT INTO message_mail_recipient_rule(scope,pattern,action,name) VALUES(2,'bad@example.com',0,'bad scope')`,
		`INSERT INTO message_mail_recipient_rule(scope,pattern,action,name) VALUES(0,'bad@example.com',2,'bad action')`,
		`INSERT INTO message_mail_recipient_rule(pattern,action,name) VALUES('bad@example.com',0,'missing scope')`,
		`INSERT INTO message_mail_recipient_rule(scope,pattern,name) VALUES(0,'bad@example.com','missing action')`,
	} {
		if err := db.WithContext(ctx).Exec(query).Error; err == nil {
			t.Fatalf("invalid enum accepted: %s", query)
		}
	}
	err := db.WithContext(ctx).Exec(`INSERT INTO message_mail_recipient_rule(scope,pattern,action,name) VALUES(0,'user@example.com',0,'duplicate')`).Error
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("active uniqueness lost: %v", err)
	}
}

func TestMailRuleNumericMigrationRejectsSourceDriftAtomically(t *testing.T) {
	for _, test := range []struct{ name, setup string }{
		{"unknown scope", `ALTER TABLE message_mail_recipient_rule DROP CONSTRAINT message_mail_recipient_rule_scope_check; UPDATE message_mail_recipient_rule SET scope='unknown' WHERE id=1;`},
		{"unknown action", `ALTER TABLE message_mail_recipient_rule DROP CONSTRAINT message_mail_recipient_rule_action_check; UPDATE message_mail_recipient_rule SET action='unknown' WHERE id=5;`},
		{"missing generation", `DELETE FROM system_config_cache_generation WHERE namespace='message.mail';`},
		{"missing unique index", `DROP INDEX ux_message_mail_rule_scope_pattern_action_active;`},
		{"mixed types", `ALTER TABLE message_mail_recipient_rule DROP CONSTRAINT message_mail_recipient_rule_scope_check; ALTER TABLE message_mail_recipient_rule ALTER COLUMN scope TYPE smallint USING CASE scope WHEN 'email' THEN 0 ELSE 1 END;`},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, ctx := mailRuleNumericFixture(t)
			if err := db.WithContext(ctx).Exec(test.setup).Error; err != nil {
				t.Fatal(err)
			}
			before := mailRuleNumericSnapshot(t, db, ctx)
			if err := db.WithContext(ctx).Exec(mailRuleNumericSQL(t)).Error; err == nil {
				t.Fatal("source drift accepted")
			}
			if mailRuleNumericSnapshot(t, db, ctx) != before {
				t.Fatal("failed migration did not roll back all facts")
			}
		})
	}
}
