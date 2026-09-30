package database_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"admin/server/internal/database/testschema"
	"gorm.io/gorm"
)

func csvMigrationFixture(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_mail_rule_csv_migration")
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE permission_auth_platform (id bigint PRIMARY KEY, code varchar(49) NOT NULL, menu_version bigint NOT NULL, is_enabled smallint NOT NULL DEFAULT 1, deleted_at timestamptz, updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE permission_menu (id bigserial PRIMARY KEY, platform_id bigint NOT NULL, parent_id bigint, menu_type varchar(16) NOT NULL, name varchar(128) NOT NULL, code varchar(128) NOT NULL, i18n_key varchar(128), path varchar(255), component_path varchar(255), icon varchar(128), remark varchar(512), sort_order integer NOT NULL DEFAULT 0, is_enabled smallint NOT NULL DEFAULT 1, is_hidden smallint NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz);
CREATE UNIQUE INDEX menu_code_active ON permission_menu(platform_id,code) WHERE deleted_at IS NULL;
CREATE TABLE system_setting (id bigserial PRIMARY KEY, setting_key varchar(128) NOT NULL, value text NOT NULL, value_type smallint NOT NULL, description varchar(512) NOT NULL DEFAULT '', is_enabled smallint NOT NULL, is_builtin smallint NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz);
CREATE UNIQUE INDEX setting_key_active ON system_setting(setting_key) WHERE deleted_at IS NULL;
CREATE TABLE system_config_cache_generation (namespace varchar(128) NOT NULL, scope_key varchar(128) NOT NULL, generation bigint NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(namespace,scope_key));
CREATE TABLE system_config_cache_outbox (id bigserial PRIMARY KEY, namespace varchar(128) NOT NULL, scope_key varchar(128) NOT NULL, generation bigint NOT NULL, attempts integer NOT NULL DEFAULT 0, available_at timestamptz NOT NULL, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL, published_at timestamptz, UNIQUE(namespace,scope_key,generation));
INSERT INTO permission_auth_platform(id,code,menu_version) VALUES (1,'admin',14),(2,'canvas',1);
INSERT INTO permission_menu(platform_id,menu_type,name,code,i18n_key,path,component_path) VALUES (1,'page','Mail','message:mail:view','navigation.mail','/message/mail','message/mail');
INSERT INTO system_config_cache_generation(namespace,scope_key,generation) VALUES ('system.setting','global',13),('message.mail','global',3);
`).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func csvMigrationSQL(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-09-30-mail-recipient-rule-csv.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func csvMigrationSnapshot(t *testing.T, db *gorm.DB, ctx context.Context) string {
	t.Helper()
	var snapshot string
	if err := db.WithContext(ctx).Raw(`SELECT jsonb_build_object(
 'platforms',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM permission_auth_platform p),
 'menus',(SELECT jsonb_agg(to_jsonb(m) ORDER BY id) FROM permission_menu m),
 'settings',(SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM system_setting s),
 'generations',(SELECT jsonb_agg(to_jsonb(g) ORDER BY namespace,scope_key) FROM system_config_cache_generation g),
 'outbox',(SELECT jsonb_agg(to_jsonb(o) ORDER BY id) FROM system_config_cache_outbox o))::text`).Scan(&snapshot).Error; err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestMailRuleCSVMigrationSeedsOnlyAdminAndIsIdempotent(t *testing.T) {
	db, ctx := csvMigrationFixture(t)
	script := csvMigrationSQL(t)
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatal(err)
	}
	var valid bool
	if err := db.WithContext(ctx).Raw(`SELECT
 (SELECT menu_version=15 FROM permission_auth_platform WHERE code='admin') AND
 (SELECT menu_version=1 FROM permission_auth_platform WHERE code='canvas') AND
 (SELECT count(*)=2 FROM permission_menu m JOIN permission_menu p ON p.id=m.parent_id WHERE m.code IN ('message:mail:rule:import','message:mail:rule:export') AND m.platform_id=1 AND p.code='message:mail:view' AND m.menu_type='action' AND m.is_hidden=1 AND m.is_enabled=1 AND m.path IS NULL AND m.component_path IS NULL AND m.icon IS NULL AND m.i18n_key IS NULL) AND
 (SELECT count(*)=1 FROM system_setting WHERE setting_key='message.mail.recipient_rule.import_template_url' AND value='' AND value_type=1 AND is_enabled=1 AND is_builtin=1) AND
 (SELECT generation=14 FROM system_config_cache_generation WHERE namespace='system.setting') AND
 (SELECT generation=3 FROM system_config_cache_generation WHERE namespace='message.mail') AND
 (SELECT count(*)=1 FROM system_config_cache_outbox WHERE namespace='system.setting' AND generation=14)`).Scan(&valid).Error; err != nil || !valid {
		t.Fatalf("seed valid=%v err=%v", valid, err)
	}
	// A later user-supplied URL must survive reruns without timestamp/version churn.
	if err := db.WithContext(ctx).Exec(`UPDATE system_setting SET value='https://example.com/template.csv'`).Error; err != nil {
		t.Fatal(err)
	}
	before := csvMigrationSnapshot(t, db, ctx)
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatal(err)
	}
	if after := csvMigrationSnapshot(t, db, ctx); after != before {
		t.Fatalf("rerun changed snapshot\nbefore=%s\nafter=%s", before, after)
	}
}

func TestMailRuleCSVMigrationConflictRollsBackWholeSeed(t *testing.T) {
	for _, conflict := range []string{
		`INSERT INTO permission_menu(platform_id,menu_type,name,code) VALUES(1,'page','conflict','message:mail:rule:export')`,
		`INSERT INTO system_setting(setting_key,value,value_type,is_enabled,is_builtin) VALUES('message.mail.recipient_rule.import_template_url','42',2,1,1)`,
	} {
		t.Run(conflict, func(t *testing.T) {
			db, ctx := csvMigrationFixture(t)
			if err := db.WithContext(ctx).Exec(conflict).Error; err != nil {
				t.Fatal(err)
			}
			before := csvMigrationSnapshot(t, db, ctx)
			if err := db.WithContext(ctx).Exec(csvMigrationSQL(t)).Error; err == nil {
				t.Fatal("conflicting seed accepted")
			}
			if after := csvMigrationSnapshot(t, db, ctx); after != before {
				t.Fatal("failed migration did not roll back all facts")
			}
		})
	}
}
