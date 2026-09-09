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

const moduleLowerCamelMigrationPath = "../../../docs/database/2026-09-09-module-lower-camel.sql"

func TestModuleLowerCamelMigrationPreservesIDsGrantsAndVersionsOnce(t *testing.T) {
	database, ctx := openModuleLowerCamelSchema(t, false)
	script, err := os.ReadFile(moduleLowerCamelMigrationPath)
	if err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 2; run++ {
		if err := database.WithContext(ctx).Exec(string(script)).Error; err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		var result struct {
			Canonical, Grants, AdminMenuVersion, CanvasMenuVersion, AdminPolicyVersion int64
		}
		if err := database.WithContext(ctx).Raw(`SELECT
(SELECT count(*) FROM permission_menu WHERE id IN (10,11,20,21,30,31,40,41,50,51) AND
 code IN ('permission:authPlatform:view','permission:authPlatform:list','system:operationLog:view','system:operationLog:list','user:loginLog:view','user:loginLog:list','storage:cosConfig:create','storage:cosConfig:update','storage:uploadRule:create','storage:uploadRule:update')) canonical,
(SELECT count(*) FROM permission_role_menu WHERE role_id=7 AND menu_id IN (11,21,31,40,41,50,51)) grants,
(SELECT menu_version FROM permission_auth_platform WHERE id=1) admin_menu_version,
(SELECT menu_version FROM permission_auth_platform WHERE id=2) canvas_menu_version,
(SELECT policy_version FROM permission_auth_platform WHERE id=1) admin_policy_version`).Scan(&result).Error; err != nil {
			t.Fatal(err)
		}
		if result.Canonical != 10 || result.Grants != 7 || result.AdminMenuVersion != 3 || result.CanvasMenuVersion != 9 || result.AdminPolicyVersion != 5 {
			t.Fatalf("run %d result=%+v", run, result)
		}
	}
	var page struct{ Path, ComponentPath, I18nKey string }
	if err := database.WithContext(ctx).Raw(`SELECT path,component_path,i18n_key FROM permission_menu WHERE id=10`).Scan(&page).Error; err != nil {
		t.Fatal(err)
	}
	if page.Path != "/permission/authPlatform" || page.ComponentPath != "permission/authPlatform" || page.I18nKey != "navigation.permissionAuthPlatform" {
		t.Fatalf("page=%+v", page)
	}
}

func TestModuleLowerCamelMigrationRejectsCanonicalCollisionAndRollsBack(t *testing.T) {
	database, ctx := openModuleLowerCamelSchema(t, true)
	script, err := os.ReadFile(moduleLowerCamelMigrationPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.WithContext(ctx).Connection(func(connection *gorm.DB) error {
		_, migrationErr := connection.Statement.ConnPool.ExecContext(ctx, string(script))
		if _, rollbackErr := connection.Statement.ConnPool.ExecContext(ctx, "ROLLBACK"); rollbackErr != nil {
			return rollbackErr
		}
		if migrationErr == nil {
			t.Error("migration accepted a canonical code collision")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var result struct{ Legacy, Canonical, MenuVersion int64 }
	if err := database.WithContext(ctx).Raw(`SELECT
(SELECT count(*) FROM permission_menu WHERE id=10 AND code='permission:authplatform:view') legacy,
(SELECT count(*) FROM permission_menu WHERE id=99 AND code='permission:authPlatform:view') canonical,
(SELECT menu_version FROM permission_auth_platform WHERE id=1) menu_version`).Scan(&result).Error; err != nil {
		t.Fatal(err)
	}
	if result.Legacy != 1 || result.Canonical != 1 || result.MenuVersion != 2 {
		t.Fatalf("failed migration left partial state: %+v", result)
	}
}

func openModuleLowerCamelSchema(t *testing.T, collision bool) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	_ = godotenv.Load("../../.env")
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	database, ctx := testschema.Open(t, settings.PostgresDSN, "test_module_lower_camel")
	fixture := `
CREATE TABLE permission_auth_platform(id bigint PRIMARY KEY,code text NOT NULL,policy_version bigint NOT NULL,menu_version bigint NOT NULL,updated_at timestamptz NOT NULL);
CREATE TABLE permission_menu(id bigint PRIMARY KEY,platform_id bigint NOT NULL,parent_id bigint,code text NOT NULL,menu_type text NOT NULL,path text,component_path text,i18n_key text,updated_at timestamptz NOT NULL,deleted_at timestamptz);
CREATE UNIQUE INDEX ux_test_menu_code ON permission_menu(platform_id,code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX ux_test_menu_path ON permission_menu(platform_id,path) WHERE path IS NOT NULL AND deleted_at IS NULL;
CREATE TABLE permission_role_menu(role_id bigint NOT NULL,menu_id bigint NOT NULL,deleted_at timestamptz);
INSERT INTO permission_auth_platform VALUES (1,'admin',5,2,now()),(2,'canvas',8,9,now());
INSERT INTO permission_menu VALUES
(10,1,NULL,'permission:authplatform:view','page','/permission/authplatform','permission/authplatform','navigation.permissionAuthplatform',now(),NULL),
(11,1,10,'permission:authplatform:list','action',NULL,NULL,NULL,now(),NULL),
(20,1,NULL,'system:operationlog:view','page','/system/operationlog','system/operationlog','navigation.systemOperationlog',now(),NULL),
(21,1,20,'system:operationlog:list','action',NULL,NULL,NULL,now(),NULL),
(30,1,NULL,'user:loginlog:view','page','/user/loginlog','user/loginlog','navigation.userLoginlog',now(),NULL),
(31,1,30,'user:loginlog:list','action',NULL,NULL,NULL,now(),NULL),
(40,1,NULL,'storage:cosconfig:create','action',NULL,NULL,NULL,now(),NULL),
(41,1,NULL,'storage:cosconfig:update','action',NULL,NULL,NULL,now(),NULL),
(50,1,NULL,'storage:uploadrule:create','action',NULL,NULL,NULL,now(),NULL),
(51,1,NULL,'storage:uploadrule:update','action',NULL,NULL,NULL,now(),NULL);
INSERT INTO permission_role_menu VALUES (7,11,NULL),(7,21,NULL),(7,31,NULL),(7,40,NULL),(7,41,NULL),(7,50,NULL),(7,51,NULL);`
	if collision {
		fixture += `INSERT INTO permission_menu VALUES (99,1,NULL,'permission:authPlatform:view','page','/permission/other','permission/other','navigation.other',now(),NULL);`
	}
	if err := database.WithContext(ctx).Exec(fixture).Error; err != nil {
		t.Fatal(err)
	}
	return database, ctx
}
