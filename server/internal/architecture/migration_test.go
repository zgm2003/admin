package architecture_test

import (
	"os"
	"testing"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestNamingAndMailMigrationPreservesIdentityAndIsIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration test")
	}
	if err := godotenv.Load("../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_naming")
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE auth_platform (id bigserial PRIMARY KEY, code text NOT NULL, CONSTRAINT ck_auth_platform_code CHECK (code <> ''));
CREATE UNIQUE INDEX ux_auth_platform_code_active ON auth_platform(code);
INSERT INTO auth_platform(code) VALUES ('admin'),('canvas');
CREATE TABLE permission_menu (id bigint PRIMARY KEY, platform_id bigint NOT NULL REFERENCES auth_platform(id), code text NOT NULL, path text, component_path text, i18n_key text, updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(platform_id,code));
CREATE TABLE permission_role_menu (role_id bigint, menu_id bigint REFERENCES permission_menu(id));
CREATE TABLE permission_access_version (user_id bigint PRIMARY KEY, version bigint NOT NULL, updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE message_mail_rate_limit_policy (policy_key text PRIMARY KEY, mode text NOT NULL, dimension text NOT NULL, limit_count int NOT NULL, window_seconds int NOT NULL, revision bigint NOT NULL, updated_at timestamptz NOT NULL DEFAULT now());
INSERT INTO permission_menu(id,platform_id,code,path,component_path,i18n_key) VALUES
(1,1,'account',NULL,NULL,'navigation.account'),
(2,1,'account:user:view','/account/users','account/users','navigation.accountUsers'),
(3,1,'account:user:roles',NULL,NULL,NULL),
(4,1,'account:user:loginlog:view','/account/login-logs','account/login-logs','navigation.accountLoginLogs'),
(5,2,'account:user:view','/account/users','account/users','navigation.accountUsers'),
(6,1,'account:profile:view','/account/profile','account/profile','layout.account.profile'),
(7,2,'canvas:test:view','/test','test','navigation.test'),
(9,1,'auth:platform:view','/permission/auth-platforms','permission/auth-platforms','navigation.accessAuthPlatforms');
INSERT INTO permission_role_menu VALUES (11,2),(11,3),(12,5);
INSERT INTO permission_access_version(user_id,version) VALUES (11,8);
INSERT INTO message_mail_rate_limit_policy(policy_key,mode,dimension,limit_count,window_seconds,revision) VALUES
('business_email_minute','business','platform_scene_email',2,60,4),
('business_email_10m','business','platform_scene_email',9,600,4),
('business_ip_minute','business','platform_ip',10,60,4),
('business_scene_minute','business','platform_scene',30,60,4),
('admin_test_user_10m','admin_test','admin_user',5,600,4),
('admin_test_ip_minute','admin_test','ip',10,60,4),
('admin_test_email_10m','admin_test','email',3,600,4);
`).Error; err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("../../../docs/database/2026-09-08-module-naming-mail-policy.sql")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("later failure rolls back platform rename", func(t *testing.T) {
		if err := db.WithContext(ctx).Exec(`INSERT INTO permission_menu(id,platform_id,code) VALUES (10,1,'user:account:view')`).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.WithContext(ctx).Connection(func(conn *gorm.DB) error {
			_, migrationErr := conn.Statement.ConnPool.ExecContext(ctx, string(script))
			if _, err := conn.Statement.ConnPool.ExecContext(ctx, "ROLLBACK"); err != nil {
				return err
			}
			if migrationErr == nil {
				t.Error("expected menu code collision after table rename")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		var intact bool
		if err := db.WithContext(ctx).Raw(`SELECT to_regclass(current_schema()||'.auth_platform') IS NOT NULL
AND to_regclass(current_schema()||'.permission_auth_platform') IS NULL
AND to_regclass(current_schema()||'.ux_auth_platform_code_active') IS NOT NULL
AND pg_get_serial_sequence('auth_platform','id')=current_schema()||'.auth_platform_id_seq'
AND (SELECT count(*) FROM message_mail_rate_limit_policy)=7`).Scan(&intact).Error; err != nil {
			t.Fatal(err)
		}
		if !intact {
			t.Fatal("failed migration left a partial platform rename")
		}
		if err := db.WithContext(ctx).Exec(`DELETE FROM permission_menu WHERE id=10`).Error; err != nil {
			t.Fatal(err)
		}
	})
	for run := 0; run < 2; run++ {
		if err := db.WithContext(ctx).Exec(string(script)).Error; err != nil {
			t.Fatal(err)
		}
		var result struct{ Grants, Policies, Version, Quota, Pages int64 }
		if err := db.WithContext(ctx).Raw(`SELECT
(SELECT count(*) FROM permission_role_menu r JOIN permission_menu m ON m.id=r.menu_id WHERE m.code IN ('user:account:view','user:account:authorize')) grants,
(SELECT count(*) FROM message_mail_rate_limit_policy WHERE dimension='platform_email') policies,
(SELECT version FROM permission_access_version WHERE user_id=11) version,
(SELECT sum(limit_count) FROM message_mail_rate_limit_policy) quota,
(SELECT count(*) FROM permission_menu WHERE component_path='user/account' AND path='/user/account' AND i18n_key='navigation.userAccount') pages`).Scan(&result).Error; err != nil {
			t.Fatal(err)
		}
		if result.Grants != 3 || result.Policies != 2 || result.Version != 9 || result.Quota != 11 || result.Pages != 2 {
			t.Fatalf("run %d: %+v", run, result)
		}
		var platformResult struct {
			Rows, Page, ForeignKeys                  int64
			OldAbsent, IndexRenamed, SequenceRenamed bool
		}
		if err := db.WithContext(ctx).Raw(`SELECT
(SELECT count(*) FROM permission_auth_platform WHERE (id=1 AND code='admin') OR (id=2 AND code='canvas')) rows,
(SELECT count(*) FROM permission_menu WHERE id=9 AND platform_id=1 AND code='permission:authplatform:view' AND component_path='permission/authplatform' AND path='/permission/authplatform' AND i18n_key='navigation.permissionAuthplatform') page,
(SELECT count(*) FROM pg_constraint WHERE conrelid='permission_menu'::regclass AND confrelid='permission_auth_platform'::regclass) foreign_keys,
to_regclass(current_schema()||'.auth_platform') IS NULL old_absent,
to_regclass(current_schema()||'.ux_permission_auth_platform_code_active') IS NOT NULL index_renamed,
pg_get_serial_sequence('permission_auth_platform','id')=current_schema()||'.permission_auth_platform_id_seq' sequence_renamed`).Scan(&platformResult).Error; err != nil {
			t.Fatal(err)
		}
		if platformResult.Rows != 2 || platformResult.Page != 1 || platformResult.ForeignKeys != 1 || !platformResult.OldAbsent || !platformResult.IndexRenamed || !platformResult.SequenceRenamed {
			t.Fatalf("platform identity changed: %+v", platformResult)
		}
	}
	t.Run("collision rolls back all changes", func(t *testing.T) {
		if err := db.WithContext(ctx).Exec(`INSERT INTO permission_menu(id,platform_id,code) VALUES (8,1,'account:user:view')`).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.WithContext(ctx).Connection(func(conn *gorm.DB) error {
			_, migrationErr := conn.Statement.ConnPool.ExecContext(ctx, string(script))
			if _, err := conn.Statement.ConnPool.ExecContext(ctx, "ROLLBACK"); err != nil {
				return err
			}
			if migrationErr == nil {
				t.Error("migration accepted conflicting canonical menu code")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		var result struct{ Legacy, Policies, Version int64 }
		if err := db.WithContext(ctx).Raw(`SELECT
(SELECT count(*) FROM permission_menu WHERE id=8 AND code='account:user:view') legacy,
(SELECT count(*) FROM message_mail_rate_limit_policy) policies,
(SELECT version FROM permission_access_version WHERE user_id=11) version`).Scan(&result).Error; err != nil {
			t.Fatal(err)
		}
		if result.Legacy != 1 || result.Policies != 2 || result.Version != 9 {
			t.Fatalf("partial migration after collision: %+v", result)
		}
	})
}
