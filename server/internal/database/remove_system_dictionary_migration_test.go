package database_test

import (
	"admin/server/internal/database/testschema"
	"context"
	"gorm.io/gorm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readRemoveDictionaryMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-10-10-remove-system-dictionary.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
func removalFixture(t *testing.T) (*gorm.DB, context.Context) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_remove_system_dictionary")
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE permission_auth_platform(id bigint PRIMARY KEY,menu_version bigint NOT NULL,updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE permission_menu(id bigint PRIMARY KEY,platform_id bigint NOT NULL REFERENCES permission_auth_platform(id),parent_id bigint REFERENCES permission_menu(id) ON DELETE RESTRICT,menu_type text NOT NULL,code text NOT NULL,path text,component_path text,deleted_at timestamptz);
CREATE TABLE permission_role_menu(id bigint PRIMARY KEY,menu_id bigint NOT NULL REFERENCES permission_menu(id) ON DELETE RESTRICT);
CREATE TABLE system_config_cache_generation(namespace text NOT NULL,scope_key text NOT NULL,generation bigint NOT NULL,PRIMARY KEY(namespace,scope_key));
CREATE TABLE system_config_cache_outbox(id bigint PRIMARY KEY,namespace text NOT NULL,scope_key text NOT NULL);
CREATE TABLE system_dictionary(id bigint PRIMARY KEY);
CREATE TABLE system_dictionary_item(id bigint PRIMARY KEY,dictionary_id bigint REFERENCES system_dictionary(id));
INSERT INTO permission_auth_platform VALUES(1,5,now()),(2,8,now());
INSERT INTO permission_menu VALUES(1,1,NULL,'directory','system',NULL,NULL,NULL),(2,1,1,'page','system:dictionary:view','/system/dictionary','system/dictionary',NULL),(3,1,2,'action','system:dictionary:list',NULL,NULL,NULL),(4,2,NULL,'page','system:setting:view','/system/setting','system/setting',NULL);
INSERT INTO permission_role_menu VALUES(1,2),(2,3),(3,4);
INSERT INTO system_config_cache_generation VALUES('system.dictionary','global',9),('system.dictionary','other',2),('message.mail','global',4);
INSERT INTO system_config_cache_outbox VALUES(1,'system.dictionary','global'),(2,'system.dictionary','other'),(3,'message.mail','global');
INSERT INTO system_dictionary VALUES(1); INSERT INTO system_dictionary_item VALUES(1,1);`).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}
func TestRemoveDictionaryMigrationIsScopedAndIdempotent(t *testing.T) {
	script := readRemoveDictionaryMigration(t)
	db, ctx := removalFixture(t)
	for i := 0; i < 2; i++ {
		if err := db.WithContext(ctx).Exec(script).Error; err != nil {
			t.Fatal(err)
		}
		var facts string
		if err := db.WithContext(ctx).Raw(`SELECT (SELECT string_agg(id::text||':'||menu_version::text,',' ORDER BY id) FROM permission_auth_platform)||'|'||(SELECT string_agg(id::text,',' ORDER BY id) FROM permission_menu)||'|'||(SELECT string_agg(id::text,',' ORDER BY id) FROM permission_role_menu)||'|'||(SELECT count(*)::text FROM system_config_cache_generation)||'|'||(SELECT count(*)::text FROM system_config_cache_outbox)||'|'||(to_regclass('system_dictionary') IS NULL)::text||'|'||(to_regclass('system_dictionary_item') IS NULL)::text`).Scan(&facts).Error; err != nil {
			t.Fatal(err)
		}
		if facts != "1:6,2:8|1,4|3|2|2|true|true" {
			t.Fatalf("run %d facts=%s", i, facts)
		}
	}
}
func TestRemoveDictionaryMigrationRejectsUnrelatedDescendantAtomically(t *testing.T) {
	script := readRemoveDictionaryMigration(t)
	db, ctx := removalFixture(t)
	if err := db.WithContext(ctx).Exec(`INSERT INTO permission_menu VALUES(9,1,2,'page','system:other:view','/system/other','system/other',NULL)`).Error; err != nil {
		t.Fatal(err)
	}
	err := db.WithContext(ctx).Exec(script).Error
	if err == nil || !strings.Contains(err.Error(), "unrelated descendant") {
		t.Fatalf("unexpected collision error: %v", err)
	}
	if err := db.WithContext(ctx).Exec("ROLLBACK").Error; err != nil {
		t.Fatal(err)
	}
	var version int64
	if err := db.WithContext(ctx).Raw(`SELECT menu_version FROM permission_auth_platform WHERE id=1`).Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	if version != 5 {
		t.Fatalf("version changed on rollback: %d", version)
	}
}
