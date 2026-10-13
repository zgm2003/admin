package database_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"admin/server/internal/database/testschema"
	"admin/server/internal/module/storage/uploadRule"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCanvasAuthUploadMigration(t *testing.T) {
	db, ctx := canvasUploadFixture(t)
	before := canvasUnrelatedDigest(t, db, ctx)
	for pass := 0; pass < 2; pass++ {
		if err := runCanvasMigration(t, db, ctx); err != nil {
			t.Fatal(err)
		}
		for query, want := range map[string]int64{
			`SELECT menu_version FROM permission_auth_platform WHERE id=1`:    19,
			`SELECT menu_version FROM permission_auth_platform WHERE id=2`:    2,
			`SELECT version FROM permission_access_version WHERE user_id=100`: 5,
			`SELECT version FROM permission_access_version WHERE user_id=101`: 8,
			`SELECT version FROM permission_access_version WHERE user_id=102`: 9,
			`SELECT count(*) FROM permission_menu WHERE platform_id=2 AND code='canvas:workspace:view' AND menu_type='page' AND parent_id IS NULL AND is_hidden=1 AND is_enabled=1 AND path='/canvas/workspace' AND component_path='canvas/workspace' AND i18n_key='navigation.canvasWorkspace'`:                                                                        1,
			`SELECT count(*) FROM permission_menu a JOIN permission_menu p ON p.id=a.parent_id WHERE a.platform_id=2 AND a.code='storage:object:upload' AND p.platform_id=2 AND p.code='canvas:workspace:view' AND a.menu_type='action' AND a.is_hidden=1 AND a.is_enabled=1 AND a.path IS NULL AND a.component_path IS NULL AND a.i18n_key IS NULL AND a.icon IS NULL`: 1,
			`SELECT count(*) FROM permission_role_menu rm JOIN permission_menu m ON m.id=rm.menu_id WHERE rm.role_id=42 AND m.platform_id=2 AND m.code='storage:object:upload' AND rm.deleted_at IS NULL`:                                                                                                                                                               1,
			`SELECT count(*) FROM permission_role_menu rm JOIN permission_menu m ON m.id=rm.menu_id WHERE rm.role_id=42 AND m.code='canvas:workspace:view' AND rm.deleted_at IS NULL`:                                                                                                                                                                                   0,
			`SELECT count(*) FROM storage_upload_rule WHERE platform_id=2`: 1,
		} {
			var got int64
			if err := db.WithContext(ctx).Raw(query).Scan(&got).Error; err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("%s = %d; want %d", query, got, want)
			}
		}
		if canvasUnrelatedDigest(t, db, ctx) != before {
			t.Fatal("unrelated Admin facts or Canvas test menus changed")
		}
		target, err := uploadrule.NewRepository(db).FindUploadTarget(ctx, 2, "canvas")
		if err != nil {
			t.Fatal(err)
		}
		if target.PlatformID != 2 || target.CosConfigID != 7 || target.MaxFileSizeBytes != 104857600 || target.AccessMode != "public" || strings.Join(target.AllowedExtensions, ",") != "png,jpg" || strings.Join(target.AllowedMimeTypes, ",") != "image/png,image/jpeg" {
			t.Fatalf("target = %+v", target)
		}
		if _, err := uploadrule.NewRepository(db).FindUploadTarget(ctx, 1, "canvas"); err == nil {
			t.Fatal("Canvas rule leaked to Admin")
		}
	}
}

func TestCanvasAuthUploadMigrationRejectsConflictsAtomically(t *testing.T) {
	for name, conflict := range map[string]string{
		"page shape":             `INSERT INTO permission_menu(platform_id,menu_type,name,code,i18n_key,path,component_path,is_hidden) VALUES(2,'page','Conflict','canvas:workspace:view','navigation.canvasWorkspace','/wrong','wrong',1)`,
		"page path":              `INSERT INTO permission_menu(platform_id,menu_type,name,code,i18n_key,path,component_path) VALUES(2,'page','Conflict','other:view','navigation.test','/canvas/workspace','canvas/workspace')`,
		"action shape":           `INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,is_hidden) VALUES(2,10,'action','Wrong parent','storage:object:upload',1)`,
		"other active rule":      `INSERT INTO storage_upload_rule(platform_id,name,cos_config_id,max_file_size_bytes,allowed_extensions,allowed_mime_types,access_mode,is_enabled) VALUES(2,'Other',7,1,ARRAY['png'],ARRAY['image/png'],'private',1)`,
		"missing source":         `DELETE FROM storage_upload_rule_code WHERE code='file'`,
		"disabled COS":           `UPDATE storage_cos_config SET is_enabled=0 WHERE id=7`,
		"missing access version": `DELETE FROM permission_access_version WHERE user_id=100`,
	} {
		t.Run(name, func(t *testing.T) {
			db, ctx := canvasUploadFixture(t)
			if err := db.WithContext(ctx).Exec(conflict).Error; err != nil {
				t.Fatal(err)
			}
			before := canvasFullDigest(t, db, ctx)
			if err := runCanvasMigration(t, db, ctx); err == nil {
				t.Fatal("conflict accepted")
			}
			if canvasFullDigest(t, db, ctx) != before {
				t.Fatal("failed migration changed facts")
			}
		})
	}
}

func TestCanvasAuthUploadMigrationPreservesLaterRuleEdits(t *testing.T) {
	db, ctx := canvasUploadFixture(t)
	if err := runCanvasMigration(t, db, ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`UPDATE storage_upload_rule SET name='Custom Canvas',max_file_size_bytes=512 WHERE platform_id=2; UPDATE storage_upload_rule SET max_file_size_bytes=1024 WHERE platform_id=1`).Error; err != nil {
		t.Fatal(err)
	}
	before := canvasFullDigest(t, db, ctx)
	if err := runCanvasMigration(t, db, ctx); err != nil {
		t.Fatal(err)
	}
	if canvasFullDigest(t, db, ctx) != before {
		t.Fatal("repeat overwrote managed rules")
	}
}

func canvasUploadFixture(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_canvas_upload")
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	for _, sql := range []string{
		`CREATE TABLE permission_auth_platform(id BIGINT PRIMARY KEY,code TEXT NOT NULL,is_enabled SMALLINT NOT NULL,menu_version BIGINT NOT NULL,policy_version BIGINT NOT NULL DEFAULT 1,updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE permission_menu(id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,platform_id BIGINT NOT NULL,parent_id BIGINT,menu_type TEXT NOT NULL,name TEXT NOT NULL,code TEXT NOT NULL,i18n_key TEXT,path TEXT,component_path TEXT,icon TEXT,remark TEXT,sort_order INTEGER NOT NULL DEFAULT 0,is_enabled SMALLINT NOT NULL DEFAULT 1,is_hidden SMALLINT NOT NULL DEFAULT 0,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),deleted_at TIMESTAMPTZ)`,
		`CREATE UNIQUE INDEX fixture_menu_code ON permission_menu(platform_id,code) WHERE deleted_at IS NULL`,
		`CREATE TABLE permission_role(id BIGINT PRIMARY KEY,code TEXT NOT NULL,is_default SMALLINT NOT NULL,is_enabled SMALLINT NOT NULL,updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE permission_role_menu(id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,role_id BIGINT NOT NULL,menu_id BIGINT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),deleted_at TIMESTAMPTZ)`,
		`CREATE UNIQUE INDEX fixture_grant ON permission_role_menu(role_id,menu_id) WHERE deleted_at IS NULL`,
		`CREATE TABLE user_account(id BIGINT PRIMARY KEY,is_enabled SMALLINT NOT NULL,deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE permission_user_role(user_id BIGINT NOT NULL,role_id BIGINT NOT NULL,deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE permission_access_version(user_id BIGINT PRIMARY KEY,version BIGINT NOT NULL,updated_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		`CREATE TABLE storage_cos_config(id BIGINT PRIMARY KEY,current_version BIGINT NOT NULL,is_enabled SMALLINT NOT NULL,deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE storage_cos_config_version(cos_config_id BIGINT NOT NULL,version BIGINT NOT NULL,bucket_domain TEXT,PRIMARY KEY(cos_config_id,version))`,
		`CREATE TABLE storage_upload_rule(id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,platform_id BIGINT NOT NULL,name TEXT NOT NULL,cos_config_id BIGINT NOT NULL,max_file_size_bytes BIGINT NOT NULL,allowed_extensions TEXT[] NOT NULL,allowed_mime_types TEXT[] NOT NULL,access_mode TEXT NOT NULL,is_enabled SMALLINT NOT NULL,remark TEXT NOT NULL DEFAULT '',created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),deleted_at TIMESTAMPTZ)`,
		`CREATE UNIQUE INDEX fixture_enabled_rule ON storage_upload_rule(platform_id) WHERE is_enabled=1 AND deleted_at IS NULL`,
		`CREATE TABLE storage_upload_rule_code(id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,rule_id BIGINT NOT NULL,code TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),deleted_at TIMESTAMPTZ)`,
		`INSERT INTO permission_auth_platform(id,code,is_enabled,menu_version) VALUES(1,'admin',1,19),(2,'canvas',1,1)`,
		`INSERT INTO permission_menu(id,platform_id,menu_type,name,code,i18n_key,path,component_path) VALUES(10,2,'page','Test','canvas:test:view','navigation.test','/test','test'),(12,1,'page','Admin storage','storage:object:view','navigation.storageObject','/storage/object','storage/object')`,
		`INSERT INTO permission_menu(id,platform_id,parent_id,menu_type,name,code,is_hidden) VALUES(11,2,10,'action','Test action','canvas:test:button',1),(13,1,12,'action','Admin upload','storage:object:upload',1)`,
		`SELECT setval(pg_get_serial_sequence('permission_menu','id'),13)`,
		`INSERT INTO permission_role(id,code,is_default,is_enabled) VALUES(42,'registered_user',1,1),(41,'super_admin',0,1)`,
		`INSERT INTO permission_role_menu(role_id,menu_id) VALUES(42,13)`,
		`INSERT INTO user_account VALUES(100,1,NULL),(101,0,NULL),(102,1,NULL)`,
		`INSERT INTO permission_user_role VALUES(100,42,NULL),(101,42,NULL),(102,41,NULL)`,
		`INSERT INTO permission_access_version(user_id,version) VALUES(100,4),(101,8),(102,9)`,
		`INSERT INTO storage_cos_config VALUES(7,1,1,NULL); INSERT INTO storage_cos_config_version VALUES(7,1,'https://bucket.invalid')`,
		`INSERT INTO storage_upload_rule(platform_id,name,cos_config_id,max_file_size_bytes,allowed_extensions,allowed_mime_types,access_mode,is_enabled) VALUES(1,'Source file policy',7,104857600,ARRAY['png','jpg'],ARRAY['image/png','image/jpeg'],'public',1)`,
		`INSERT INTO storage_upload_rule_code(rule_id,code) VALUES(1,'file'),(1,'avatar'),(1,'setting')`,
	} {
		if err := db.WithContext(ctx).Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db, ctx
}

func runCanvasMigration(t *testing.T, db *gorm.DB, ctx context.Context) error {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(repoRoot(t), "docs/database/2026-10-13-canvas-auth-upload.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(string(content)).Error })
}

func canvasUnrelatedDigest(t *testing.T, db *gorm.DB, ctx context.Context) string {
	return canvasDigest(t, db, ctx, `SELECT md5(jsonb_build_object(
 'menus',(SELECT jsonb_agg(to_jsonb(m) ORDER BY id) FROM permission_menu m WHERE platform_id=1 OR id IN (10,11)),
 'rules',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM storage_upload_rule r WHERE platform_id=1),
 'codes',(SELECT jsonb_agg(to_jsonb(c) ORDER BY id) FROM storage_upload_rule_code c WHERE rule_id=1),
 'admin',(SELECT to_jsonb(p) FROM permission_auth_platform p WHERE id=1),
 'grants',(SELECT jsonb_agg(to_jsonb(g) ORDER BY id) FROM permission_role_menu g WHERE menu_id=13))::text)`)
}

func canvasFullDigest(t *testing.T, db *gorm.DB, ctx context.Context) string {
	return canvasDigest(t, db, ctx, `SELECT md5(jsonb_build_object(
 'menus',(SELECT jsonb_agg(to_jsonb(m) ORDER BY id) FROM permission_menu m),
 'rules',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM storage_upload_rule r),
 'codes',(SELECT jsonb_agg(to_jsonb(c) ORDER BY id) FROM storage_upload_rule_code c),
 'platforms',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM permission_auth_platform p),
 'roles',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM permission_role r),
 'versions',(SELECT jsonb_agg(to_jsonb(v) ORDER BY user_id) FROM permission_access_version v),
 'grants',(SELECT jsonb_agg(to_jsonb(g) ORDER BY id) FROM permission_role_menu g))::text)`)
}

func canvasDigest(t *testing.T, db *gorm.DB, ctx context.Context, query string) string {
	t.Helper()
	var value string
	if err := db.WithContext(ctx).Raw(query).Scan(&value).Error; err != nil {
		t.Fatal(err)
	}
	return value
}
