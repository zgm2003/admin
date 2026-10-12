package database_test

import (
	"admin/server/internal/database/testschema"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStorageFileMigration(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		name := "idempotent"
		if conflict {
			name = "conflict_rollback"
		}
		t.Run(name, func(t *testing.T) {
			db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_storage_file")
			for _, s := range []string{
				`CREATE TABLE system_scheduler_schedule(id BIGSERIAL PRIMARY KEY,name TEXT,description TEXT,task_type TEXT,cron_expression TEXT,timezone TEXT,params JSONB,is_enabled SMALLINT,next_run_at TIMESTAMPTZ,builtin_key TEXT,created_at TIMESTAMPTZ DEFAULT now(),updated_at TIMESTAMPTZ DEFAULT now(),deleted_at TIMESTAMPTZ)`,
				`CREATE TABLE permission_auth_platform(id BIGINT PRIMARY KEY,code TEXT,is_enabled SMALLINT,menu_version BIGINT,updated_at TIMESTAMPTZ DEFAULT now(),deleted_at TIMESTAMPTZ)`,
				`CREATE TABLE permission_menu(id BIGSERIAL PRIMARY KEY,platform_id BIGINT,parent_id BIGINT,menu_type TEXT,name TEXT,code TEXT,i18n_key TEXT,path TEXT,component_path TEXT,icon TEXT,sort_order INTEGER DEFAULT 0,is_enabled SMALLINT DEFAULT 1,is_hidden SMALLINT DEFAULT 0,created_at TIMESTAMPTZ DEFAULT now(),updated_at TIMESTAMPTZ DEFAULT now(),deleted_at TIMESTAMPTZ)`,
				`CREATE TABLE permission_role(id BIGINT PRIMARY KEY,code TEXT,is_enabled SMALLINT,deleted_at TIMESTAMPTZ)`,
				`CREATE TABLE permission_role_menu(id BIGSERIAL PRIMARY KEY,role_id BIGINT,menu_id BIGINT,created_at TIMESTAMPTZ DEFAULT now(),updated_at TIMESTAMPTZ DEFAULT now(),deleted_at TIMESTAMPTZ)`,
				`INSERT INTO permission_auth_platform VALUES(1,'admin',1,17,now(),NULL),(2,'canvas',1,1,now(),NULL)`,
				`INSERT INTO permission_menu(id,platform_id,menu_type,name,code,i18n_key) VALUES(10,1,'directory','云服务','storage','navigation.storage'),(11,2,'directory','云服务','storage','navigation.storage')`,
				`INSERT INTO permission_role VALUES(42,'registered_user',1,NULL),(41,'super_admin',1,NULL)`,
				`INSERT INTO permission_role_menu(role_id,menu_id) VALUES(41,10)`,
			} {
				if err := db.WithContext(ctx).Exec(s).Error; err != nil {
					t.Fatal(err)
				}
			}
			if conflict {
				if err := db.WithContext(ctx).Exec(`INSERT INTO permission_menu(platform_id,menu_type,name,code,path,component_path,i18n_key) VALUES(1,'page','冲突','another:view','/storage/file','other','other')`).Error; err != nil {
					t.Fatal(err)
				}
			}
			b, err := os.ReadFile(filepath.Join(repoRoot(t), "docs/database/2026-10-12-storage-file.sql"))
			if err != nil {
				t.Fatal(err)
			}
			err = db.WithContext(ctx).Exec(string(b)).Error
			if conflict {
				if err == nil {
					t.Fatal("expected conflict")
				}
				var exists bool
				if err := db.WithContext(ctx).Raw(`SELECT to_regclass(current_schema()||'.storage_file') IS NOT NULL`).Scan(&exists).Error; err != nil {
					t.Fatal(err)
				}
				if exists {
					t.Fatal("DDL did not rollback")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			var scheduleTime string
			if err := db.WithContext(ctx).Raw(`SELECT next_run_at::text FROM system_scheduler_schedule WHERE builtin_key='storage.file.scan'`).Scan(&scheduleTime).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.WithContext(ctx).Exec(string(b)).Error; err != nil {
				t.Fatal(err)
			}

			var repeatedTime string
			if err := db.WithContext(ctx).Raw(`SELECT next_run_at::text FROM system_scheduler_schedule WHERE builtin_key='storage.file.scan'`).Scan(&repeatedTime).Error; err != nil {
				t.Fatal(err)
			}
			if repeatedTime != scheduleTime {
				t.Fatal("repeat advanced scan next_run_at")
			}
			for query, want := range map[string]int64{
				`SELECT count(*) FROM system_scheduler_schedule WHERE builtin_key='storage.file.scan' AND task_type='storage.file.scan' AND cron_expression='* * * * *' AND timezone='UTC' AND params='{}'::jsonb AND is_enabled=1`: 1,
				`SELECT menu_version FROM permission_auth_platform WHERE id=1`: 18, `SELECT menu_version FROM permission_auth_platform WHERE id=2`: 1,
				`SELECT count(*) FROM permission_menu WHERE code LIKE 'storage:file:%' AND platform_id=1`: 4, `SELECT count(*) FROM permission_menu WHERE code LIKE 'storage:file:%' AND platform_id=2`: 0,
				`SELECT count(*) FROM permission_role_menu WHERE role_id=42`: 4, `SELECT count(*) FROM permission_role_menu WHERE role_id=41 AND menu_id=10`: 1,
				`SELECT count(*) FROM permission_menu WHERE code='storage:file:view' AND parent_id=10 AND path='/storage/file' AND component_path='storage/file' AND i18n_key='menu.storage.file' AND menu_type='page' AND is_hidden=0`:                         1,
				`SELECT count(*) FROM permission_menu WHERE code IN ('storage:file:list','storage:file:detail','storage:file:delete') AND menu_type='action' AND is_hidden=1 AND path IS NULL AND icon IS NULL AND component_path IS NULL AND i18n_key IS NULL`: 3,
			} {
				var got int64
				if err := db.WithContext(ctx).Raw(query).Scan(&got).Error; err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Fatalf("%s = %d want %d", query, got, want)
				}
			}
			insert := `INSERT INTO storage_file(user_id,platform_id,kind,cos_config_id,config_version,upload_rule_id,rule_code,object_key,original_name,mime_type,size_bytes,status) VALUES(1,1,1,1,1,1,'avatar','unique','x.png','image/png',1,2)`
			if err := db.WithContext(ctx).Exec(insert).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.WithContext(ctx).Exec(insert).Error; err == nil {
				t.Fatal("duplicate key accepted")
			}

			if err := db.WithContext(ctx).Exec(`INSERT INTO storage_file(user_id,platform_id,kind,cos_config_id,config_version,upload_rule_id,rule_code,object_key,original_name,mime_type,size_bytes,status,created_at) SELECT 1+n%100,1,1,1,1,1,'avatar','bulk-'||n,'x.png','image/png',1,2,now()-n*interval '1 second' FROM generate_series(1,100000) n; ANALYZE storage_file;`).Error; err != nil {
				t.Fatal(err)
			}
			var plan string
			if err := db.WithContext(ctx).Raw(`EXPLAIN (FORMAT JSON) SELECT id FROM storage_file WHERE platform_id=1 AND user_id=7 AND kind=1 AND status=2 AND deleted_at IS NULL ORDER BY created_at DESC,id DESC LIMIT 20`).Scan(&plan).Error; err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(plan, "ix_storage_file_owner_list") {
				t.Fatalf("owner index unused: %s", plan)
			}
			if err := db.WithContext(ctx).Exec(`UPDATE storage_file SET status=1,credential_expires_at=now()-interval '2 minutes' WHERE id%1000=0; UPDATE storage_file SET upload_object_key='temp-'||id,credential_expires_at=now()-interval '2 minutes' WHERE id%1000=1; ANALYZE storage_file;`).Error; err != nil {
				t.Fatal(err)
			}
			for query, index := range map[string]string{
				`EXPLAIN (FORMAT JSON) SELECT id FROM storage_file WHERE id>0 AND deleted_at IS NULL AND status IN (1,3) AND (status=3 OR credential_expires_at<=now()-interval '1 minute') AND (operation_lease_until IS NULL OR operation_lease_until<=now()-interval '1 minute') ORDER BY id LIMIT 100`:              "ix_storage_file_cleanup",
				`EXPLAIN (FORMAT JSON) SELECT id FROM storage_file WHERE id>0 AND deleted_at IS NULL AND status=2 AND upload_object_key IS NOT NULL AND credential_expires_at<=now()-interval '1 minute' AND (operation_lease_until IS NULL OR operation_lease_until<=now()-interval '1 minute') ORDER BY id LIMIT 100`: "ix_storage_file_temporary_cleanup",
			} {
				var result string
				if err := db.WithContext(ctx).Raw(query).Scan(&result).Error; err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(result, index) {
					t.Fatalf("cleanup index %s unused: %s", index, result)
				}
			}

			if err := db.WithContext(ctx).Exec(`DROP INDEX ix_storage_file_cleanup; CREATE INDEX ix_storage_file_cleanup ON storage_file(id)`).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.WithContext(ctx).Exec(string(b)).Error; err == nil {
				t.Fatal("incompatible existing index accepted")
			}
			var version int64
			if err := db.WithContext(ctx).Raw(`SELECT menu_version FROM permission_auth_platform WHERE id=1`).Scan(&version).Error; err != nil {
				t.Fatal(err)
			}
			if version != 18 {
				t.Fatal("conflict altered platform version")
			}
			for _, tail := range []string{`size_bytes=0`, `kind=0`, `status=4`, `user_id=0`} {
				if err := db.WithContext(ctx).Exec(`UPDATE storage_file SET ` + tail).Error; err == nil {
					t.Fatalf("invalid %s accepted", tail)
				}
			}
		})
	}
}
