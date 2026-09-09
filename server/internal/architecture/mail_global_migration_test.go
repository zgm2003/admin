package architecture_test

import (
	"context"
	"os"
	"testing"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

const mailGlobalMigrationPath = "../../../docs/database/2026-09-08-mail-global-configuration.sql"

func TestMailGlobalConfigurationMigrationIsIdempotentAndPreservesFacts(t *testing.T) {
	db, ctx := openMailGlobalMigrationSchema(t, "test_mail_global", "")
	script, err := os.ReadFile(mailGlobalMigrationPath)
	if err != nil {
		t.Fatal(err)
	}

	for run := 0; run < 2; run++ {
		if err := db.WithContext(ctx).Exec(string(script)).Error; err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
	}

	var result struct {
		PlatformColumns, Configs, Templates, Rules, GlobalIndexes int64
	}
	if err := db.WithContext(ctx).Raw(`SELECT
(SELECT count(*) FROM information_schema.columns
 WHERE table_schema=current_schema()
   AND table_name IN ('message_mail_config','message_mail_template','message_mail_recipient_rule')
   AND column_name='platform_id') platform_columns,
(SELECT count(*) FROM message_mail_config WHERE id=10 AND from_email='sender@example.com' AND deleted_at IS NULL) configs,
(SELECT count(*) FROM message_mail_template WHERE id IN (20,21) AND scene IN ('login','forget') AND deleted_at IS NULL) templates,
(SELECT count(*) FROM message_mail_recipient_rule WHERE id=30 AND pattern='example.com' AND deleted_at IS NULL) rules,
(SELECT count(*) FROM pg_indexes WHERE schemaname=current_schema()
 AND indexname IN ('ux_message_mail_config_active_singleton','ux_message_mail_template_scene_active','ux_message_mail_rule_scope_pattern_action_active')) global_indexes`).Scan(&result).Error; err != nil {
		t.Fatal(err)
	}
	if result.PlatformColumns != 0 || result.Configs != 1 || result.Templates != 2 || result.Rules != 1 || result.GlobalIndexes != 3 {
		t.Fatalf("migration result = %+v", result)
	}

	if err := db.WithContext(ctx).Exec(`INSERT INTO message_mail_config(id,secret_id_ciphertext,secret_key_ciphertext,secret_id_hint,secret_key_hint,region,from_email,from_name,ttl_minutes,is_enabled,last_test_error,created_at,updated_at) VALUES (11,'a','b','a','b','ap-guangzhou','other@example.com','Other',5,1,'',now(),now())`).Error; err == nil {
		t.Fatal("global mail configuration accepted a second active row")
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO message_mail_template(id,scene,name,subject,tencent_template_id,variables,example_variables,is_enabled,created_at,updated_at) VALUES (22,'login','Duplicate','Duplicate',47941,'{}','{}',1,now(),now())`).Error; err == nil {
		t.Fatal("global mail templates accepted a duplicate active scene")
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO message_mail_recipient_rule(id,scope,pattern,action,name,remark,is_enabled,created_at,updated_at) VALUES (31,'domain','example.com','deny','Duplicate','',1,now(),now())`).Error; err == nil {
		t.Fatal("global mail rules accepted a duplicate active rule")
	}
}

func TestMailGlobalConfigurationMigrationRejectsAmbiguousFactsAndRollsBack(t *testing.T) {
	cases := []struct {
		name  string
		extra string
	}{
		{name: "config", extra: `INSERT INTO message_mail_config(id,platform_id,secret_id_ciphertext,secret_key_ciphertext,secret_id_hint,secret_key_hint,region,from_email,from_name,ttl_minutes,is_enabled,last_test_error,created_at,updated_at) VALUES (11,2,'a','b','a','b','ap-guangzhou','other@example.com','Other',5,1,'',now(),now());`},
		{name: "template", extra: `INSERT INTO message_mail_template(id,platform_id,scene,name,subject,tencent_template_id,variables,example_variables,is_enabled,created_at,updated_at) VALUES (22,2,'login','Duplicate','Duplicate',47941,'{}','{}',1,now(),now());`},
		{name: "rule", extra: `INSERT INTO message_mail_recipient_rule(id,platform_id,scope,pattern,action,name,remark,is_enabled,created_at,updated_at) VALUES (31,2,'domain','example.com','deny','Duplicate','',1,now(),now());`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			db, ctx := openMailGlobalMigrationSchema(t, "test_mail_global_conflict_"+test.name, test.extra)
			script, err := os.ReadFile(mailGlobalMigrationPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.WithContext(ctx).Connection(func(connection *gorm.DB) error {
				_, migrationErr := connection.Statement.ConnPool.ExecContext(ctx, string(script))
				if _, rollbackErr := connection.Statement.ConnPool.ExecContext(ctx, "ROLLBACK"); rollbackErr != nil {
					return rollbackErr
				}
				if migrationErr == nil {
					t.Error("migration accepted ambiguous mail facts")
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			var intact struct{ PlatformColumns, OldIndexes int64 }
			if err := db.WithContext(ctx).Raw(`SELECT
(SELECT count(*) FROM information_schema.columns
 WHERE table_schema=current_schema()
   AND table_name IN ('message_mail_config','message_mail_template','message_mail_recipient_rule')
   AND column_name='platform_id') platform_columns,
(SELECT count(*) FROM pg_indexes WHERE schemaname=current_schema()
 AND indexname IN ('ux_message_mail_config_platform_active','ux_message_mail_template_platform_scene_active','ux_message_mail_rule_platform_scope_pattern_action_active')) old_indexes`).Scan(&intact).Error; err != nil {
				t.Fatal(err)
			}
			if intact.PlatformColumns != 3 || intact.OldIndexes != 3 {
				t.Fatalf("failed migration left partial DDL: %+v", intact)
			}
		})
	}
}

func openMailGlobalMigrationSchema(t *testing.T, name, extra string) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	if err := godotenv.Load("../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	db, ctx := testschema.Open(t, settings.PostgresDSN, name)
	fixture := `
CREATE TABLE permission_auth_platform(id bigint PRIMARY KEY);
INSERT INTO permission_auth_platform VALUES (1),(2);
CREATE TABLE message_mail_config(
 id bigint PRIMARY KEY, platform_id bigint NOT NULL REFERENCES permission_auth_platform(id),
 secret_id_ciphertext text NOT NULL, secret_key_ciphertext text NOT NULL,
 secret_id_hint varchar(32) NOT NULL, secret_key_hint varchar(32) NOT NULL,
 region varchar(64) NOT NULL, endpoint varchar(255), from_email varchar(254) NOT NULL,
 from_name varchar(128) NOT NULL, reply_to varchar(254), ttl_minutes smallint NOT NULL,
 is_enabled smallint NOT NULL, last_test_at timestamptz, last_test_error varchar(512) NOT NULL,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL, deleted_at timestamptz);
CREATE UNIQUE INDEX ux_message_mail_config_platform_active ON message_mail_config(platform_id) WHERE deleted_at IS NULL;
CREATE TABLE message_mail_template(
 id bigint PRIMARY KEY, platform_id bigint NOT NULL REFERENCES permission_auth_platform(id),
 scene varchar(32) NOT NULL, name varchar(128) NOT NULL, subject varchar(255) NOT NULL,
 tencent_template_id integer NOT NULL, variables jsonb NOT NULL, example_variables jsonb NOT NULL,
 is_enabled smallint NOT NULL, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL, deleted_at timestamptz);
CREATE UNIQUE INDEX ux_message_mail_template_platform_scene_active ON message_mail_template(platform_id,scene) WHERE deleted_at IS NULL;
CREATE TABLE message_mail_recipient_rule(
 id bigint PRIMARY KEY, platform_id bigint NOT NULL REFERENCES permission_auth_platform(id),
 scope varchar(16) NOT NULL, pattern varchar(254) NOT NULL, action varchar(16) NOT NULL,
 name varchar(128) NOT NULL, remark varchar(512) NOT NULL, is_enabled smallint NOT NULL,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL, deleted_at timestamptz);
CREATE UNIQUE INDEX ux_message_mail_rule_platform_scope_pattern_action_active ON message_mail_recipient_rule(platform_id,scope,pattern,action) WHERE deleted_at IS NULL;
INSERT INTO message_mail_config(id,platform_id,secret_id_ciphertext,secret_key_ciphertext,secret_id_hint,secret_key_hint,region,from_email,from_name,ttl_minutes,is_enabled,last_test_error,created_at,updated_at)
VALUES (10,1,'secret-id','secret-key','id','key','ap-guangzhou','sender@example.com','Sender',5,1,'',now(),now());
INSERT INTO message_mail_template(id,platform_id,scene,name,subject,tencent_template_id,variables,example_variables,is_enabled,created_at,updated_at) VALUES
(20,1,'login','Login','Login',47941,'{}','{}',1,now(),now()),
(21,1,'forget','Forget','Forget',47942,'{}','{}',1,now(),now());
INSERT INTO message_mail_recipient_rule(id,platform_id,scope,pattern,action,name,remark,is_enabled,created_at,updated_at)
VALUES (30,1,'domain','example.com','deny','Example','',1,now(),now());
` + extra
	if err := db.WithContext(ctx).Exec(fixture).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}
