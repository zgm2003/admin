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

func TestAdminRegistrationMigrationUsesLeastPrivilegeDefaultRole(t *testing.T) {
	db, ctx := openAdminRegistrationMigrationSchema(t, true)
	script, err := os.ReadFile("../../../docs/database/2026-09-08-admin-email-registration.sql")
	if err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 2; run++ {
		if err := db.WithContext(ctx).Exec(string(script)).Error; err != nil {
			t.Fatal(err)
		}
		var result struct {
			AllowRegister, PolicyVersion, MenuVersion, Grants, RegisteredVersion, SuperVersion, ProfileParents int64
		}
		if err := db.WithContext(ctx).Raw(`SELECT
(SELECT allow_register FROM permission_auth_platform WHERE id=1) allow_register,
(SELECT policy_version FROM permission_auth_platform WHERE id=1) policy_version,
(SELECT menu_version FROM permission_auth_platform WHERE id=1) menu_version,
(SELECT count(*) FROM permission_role_menu rm JOIN permission_menu m ON m.id=rm.menu_id
 WHERE rm.role_id=42 AND rm.deleted_at IS NULL
   AND m.code IN ('user:profile:view','user:profile:detail','user:profile:update','user:password:update','storage:object:upload')) grants,
(SELECT version FROM permission_access_version WHERE user_id=67) registered_version,
(SELECT version FROM permission_access_version WHERE user_id=169) super_version,
(SELECT count(parent_id) FROM permission_menu WHERE id=10) profile_parents`).Scan(&result).Error; err != nil {
			t.Fatal(err)
		}
		if result.AllowRegister != 1 || result.PolicyVersion != 3 || result.MenuVersion != 2 || result.Grants != 5 || result.RegisteredVersion != 5 || result.SuperVersion != 10 || result.ProfileParents != 0 {
			t.Fatalf("run %d: %+v", run, result)
		}
		var extra int64
		if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_role_menu WHERE role_id=42 AND deleted_at IS NULL`).Scan(&extra).Error; err != nil || extra != 5 {
			t.Fatalf("default role has %d direct grants: %v", extra, err)
		}
	}
}

func TestAdminRegistrationMigrationRollsBackWhenProfileContractIsIncomplete(t *testing.T) {
	db, ctx := openAdminRegistrationMigrationSchema(t, false)
	script, err := os.ReadFile("../../../docs/database/2026-09-08-admin-email-registration.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(string(script)).Error; err == nil {
		t.Fatal("migration accepted an incomplete profile permission contract")
	}
	var result struct{ AllowRegister, PolicyVersion, Grants, Version int64 }
	if err := db.WithContext(ctx).Raw(`SELECT
(SELECT allow_register FROM permission_auth_platform WHERE id=1) allow_register,
(SELECT policy_version FROM permission_auth_platform WHERE id=1) policy_version,
(SELECT count(*) FROM permission_role_menu WHERE role_id=42 AND deleted_at IS NULL) grants,
(SELECT version FROM permission_access_version WHERE user_id=67) version`).Scan(&result).Error; err != nil {
		t.Fatal(err)
	}
	if result.AllowRegister != 0 || result.PolicyVersion != 2 || result.Grants != 3 || result.Version != 4 {
		t.Fatalf("failed migration left partial changes: %+v", result)
	}
}

func openAdminRegistrationMigrationSchema(t *testing.T, complete bool) (*gorm.DB, context.Context) {
	t.Helper()
	if err := godotenv.Load("../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_admin_registration")
	thirdAction := ""
	if complete {
		thirdAction = `(14,1,10,'user:password:update','action',1,NULL),`
	}
	fixture := `
CREATE TABLE permission_auth_platform (id bigint PRIMARY KEY, code text NOT NULL, is_builtin smallint NOT NULL, is_enabled smallint NOT NULL, deleted_at timestamptz, allow_register smallint NOT NULL, policy_version bigint NOT NULL, menu_version bigint NOT NULL DEFAULT 1, updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE permission_role (id bigint PRIMARY KEY, code text NOT NULL, is_default smallint NOT NULL, is_enabled smallint NOT NULL, deleted_at timestamptz, updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE permission_menu (id bigint PRIMARY KEY, platform_id bigint NOT NULL, parent_id bigint, code text NOT NULL, menu_type text NOT NULL, is_enabled smallint NOT NULL, deleted_at timestamptz);
ALTER TABLE permission_menu ADD COLUMN path text, ADD COLUMN component_path text,
  ADD COLUMN is_hidden smallint NOT NULL DEFAULT 1,
  ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
CREATE TABLE permission_role_menu (id bigserial PRIMARY KEY, role_id bigint NOT NULL, menu_id bigint NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz);
CREATE UNIQUE INDEX ux_test_role_menu_active ON permission_role_menu(role_id,menu_id) WHERE deleted_at IS NULL;
CREATE TABLE permission_user_role (user_id bigint NOT NULL, role_id bigint NOT NULL, deleted_at timestamptz);
CREATE TABLE permission_access_version (user_id bigint PRIMARY KEY, version bigint NOT NULL, updated_at timestamptz NOT NULL DEFAULT now());
INSERT INTO permission_auth_platform(id,code,is_builtin,is_enabled,deleted_at,allow_register,policy_version,menu_version,updated_at)
VALUES (1,'admin',1,1,NULL,0,2,1,now());
INSERT INTO permission_role VALUES (41,'super_admin',0,1,NULL,now()),(42,'registered_user',1,1,NULL,now());
INSERT INTO permission_menu(id,platform_id,parent_id,code,menu_type,is_enabled,deleted_at) VALUES
(9,1,NULL,'user','directory',1,NULL),
(10,1,9,'user:profile:view','page',1,NULL),
(11,1,10,'user:profile:detail','action',1,NULL),
(12,1,10,'user:profile:update','action',1,NULL),
` + thirdAction + `
(20,1,NULL,'permission:menu:view','page',1,NULL),
(21,1,20,'permission:menu:update','action',1,NULL),
(30,1,NULL,'storage:object:view','page',1,NULL),
(31,1,30,'storage:object:upload','action',1,NULL);
UPDATE permission_menu SET path='/user/profile',component_path='user/profile' WHERE id=10;
INSERT INTO permission_role_menu(role_id,menu_id)
SELECT 42,id FROM permission_menu WHERE id IN (11,12,14,31);
INSERT INTO permission_user_role VALUES (67,42,NULL),(169,41,NULL);
INSERT INTO permission_access_version VALUES (67,4,now()),(169,10,now());`
	if err := db.WithContext(ctx).Exec(fixture).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}
