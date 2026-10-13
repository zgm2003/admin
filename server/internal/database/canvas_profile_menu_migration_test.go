package database_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/gorm"
)

func TestCanvasProfileMenuMigration(t *testing.T) {
	db, ctx := canvasUploadFixture(t)
	before := canvasProfileUntouched(t, db, ctx)
	for pass := 0; pass < 2; pass++ {
		if err := runCanvasProfileMenuMigration(t, db, ctx); err != nil {
			t.Fatal(err)
		}
		var count int64
		if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_menu WHERE platform_id=2 AND deleted_at IS NULL AND code IN ('user:profile:view','user:profile:detail','user:profile:update','user:password:update','user:email:update','user:phone:update')`).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 6 {
			t.Fatalf("profile nodes=%d, want 6", count)
		}
		if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_menu p JOIN permission_menu a ON a.parent_id=p.id AND a.platform_id=p.platform_id WHERE p.platform_id=2 AND p.code='user:profile:view' AND p.menu_type='page' AND p.parent_id IS NULL AND p.is_hidden=1 AND p.is_enabled=1 AND p.path='/user/profile' AND p.component_path='user/profile' AND p.i18n_key='layout.user.profile' AND p.deleted_at IS NULL AND a.menu_type='action' AND a.is_hidden=1 AND a.is_enabled=1 AND a.path IS NULL AND a.component_path IS NULL AND a.i18n_key IS NULL AND a.icon IS NULL AND a.deleted_at IS NULL`).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 5 {
			t.Fatalf("profile action shapes=%d, want 5", count)
		}
		if err := db.WithContext(ctx).Raw(`SELECT menu_version FROM permission_auth_platform WHERE id=2`).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 2 {
			t.Fatalf("Canvas menu version=%d, want 2 on both runs", count)
		}
		if canvasProfileUntouched(t, db, ctx) != before {
			t.Fatal("changed grants, user versions, upload rules or unrelated menus")
		}
	}
}

func TestCanvasProfileMenuConflictRollsBack(t *testing.T) {
	for _, sql := range []string{
		`INSERT INTO permission_menu(platform_id,menu_type,name,code,i18n_key,path,component_path) VALUES(2,'page','Conflict','canvas:conflict:view','navigation.test','/user/profile','user/profile')`,
		`INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,is_hidden) VALUES(2,10,'action','Wrong parent','user:profile:detail',1)`,
	} {
		t.Run(sql, func(t *testing.T) {
			db, ctx := canvasUploadFixture(t)
			if err := db.WithContext(ctx).Exec(sql).Error; err != nil {
				t.Fatal(err)
			}
			before := canvasFullDigest(t, db, ctx)
			if err := runCanvasProfileMenuMigration(t, db, ctx); err == nil {
				t.Fatal("conflicting facts accepted")
			}
			if canvasFullDigest(t, db, ctx) != before {
				t.Fatal("failed migration changed facts")
			}
		})
	}
}

func runCanvasProfileMenuMigration(t *testing.T, db *gorm.DB, ctx context.Context) error {
	t.Helper()
	sql, err := os.ReadFile(filepath.Join(repoRoot(t), "docs/database/2026-10-13-canvas-profile-menu.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(string(sql)).Error })
}

func canvasProfileUntouched(t *testing.T, db *gorm.DB, ctx context.Context) string {
	return canvasDigest(t, db, ctx, `SELECT md5(jsonb_build_object(
 'menus',(SELECT jsonb_agg(to_jsonb(m) ORDER BY id) FROM permission_menu m WHERE platform_id<>2 OR code NOT IN ('user:profile:view','user:profile:detail','user:profile:update','user:password:update','user:email:update','user:phone:update')),
 'platforms',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM permission_auth_platform p WHERE id<>2),
 'roles',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM permission_role r),
 'grants',(SELECT jsonb_agg(to_jsonb(g) ORDER BY id) FROM permission_role_menu g),
 'membership',(SELECT jsonb_agg(to_jsonb(u) ORDER BY user_id,role_id) FROM permission_user_role u),
 'versions',(SELECT jsonb_agg(to_jsonb(v) ORDER BY user_id) FROM permission_access_version v),
 'rules',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM storage_upload_rule r),
 'codes',(SELECT jsonb_agg(to_jsonb(c) ORDER BY id) FROM storage_upload_rule_code c))::text)`)
}
