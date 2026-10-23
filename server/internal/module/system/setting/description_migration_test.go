package setting

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

var builtinDescriptionCases = []struct{ key, previous, want string }{
	{"app.brand.default_avatar", "Default avatar COS object key using the avatar upload rule", "默认头像 COS 对象键（使用头像上传规则）"},
	{"app.brand.title_en_us", "Admin English brand title", "管理后台英文品牌标题"},
	{"app.brand.title_zh_cn", "Admin 中文品牌标题", "管理后台中文品牌标题"},
	{"app.legal.privacy_policy", "Published privacy policy HTML", "已发布的隐私政策 HTML 内容"},
	{"app.legal.user_agreement", "Published user agreement HTML", "已发布的用户协议 HTML 内容"},
	{"auth.captcha.slide_padding", "Captcha slide tolerance in pixels", "滑块验证码允许偏差（像素）"},
	{"auth.captcha.ttl_minutes", "Captcha challenge lifetime in minutes", "验证码有效期（分钟）"},
}

func descriptionMigrationFixture(t *testing.T) (*gorm.DB, context.Context, string) {
	t.Helper()
	db, ctx := openSettingDatabase(t)
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for _, item := range builtinDescriptionCases {
		seedSettingRow(t, db, ctx, Model{Key: item.key, Value: "preserve-value", ValueType: ValueTypeString,
			Description: item.previous, Status: yesno.Yes, IsBuiltin: yesno.Yes, CreatedAt: now, UpdatedAt: now})
	}
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../../docs/database/2026-10-23-setting-builtin-descriptions.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return db, ctx, string(raw)
}

func descriptionSnapshot(t *testing.T, db *gorm.DB, ctx context.Context, normalize bool) string {
	t.Helper()
	projection := "to_jsonb(s)"
	if normalize {
		projection += " - 'description' - 'updated_at'"
	}
	var snapshot string
	if err := db.WithContext(ctx).Raw("SELECT jsonb_agg(" + projection + " ORDER BY id)::text FROM system_setting s").Scan(&snapshot).Error; err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestSettingBuiltinDescriptionMigrationTranslatesAndReplays(t *testing.T) {
	db, ctx, sql := descriptionMigrationFixture(t)
	before := descriptionSnapshot(t, db, ctx, true)
	apply := func() error {
		return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(sql).Error })
	}
	if err := apply(); err != nil {
		t.Fatal(err)
	}
	for _, item := range builtinDescriptionCases {
		var row Model
		if err := db.WithContext(ctx).Where("setting_key = ?", item.key).First(&row).Error; err != nil {
			t.Fatal(err)
		}
		if row.Description != item.want || !row.UpdatedAt.After(row.CreatedAt) {
			t.Errorf("%s description=%q want=%q updated=%v", item.key, row.Description, item.want, row.UpdatedAt)
		}
	}
	if got := descriptionSnapshot(t, db, ctx, true); got != before {
		t.Fatal("migration changed setting keys, values, types, status, or ownership")
	}
	assertSettingGeneration(t, db, ctx, "global", 2)
	assertSettingOutbox(t, db, ctx, "global", 1, 2, false)
	after := descriptionSnapshot(t, db, ctx, false)
	if err := apply(); err != nil {
		t.Fatal(err)
	}
	if got := descriptionSnapshot(t, db, ctx, false); got != after {
		t.Fatal("replay rewrote settings")
	}
	assertSettingGeneration(t, db, ctx, "global", 2)
	assertSettingOutbox(t, db, ctx, "global", 1, 2, false)
}

func TestSettingBuiltinDescriptionMigrationPreservesCustomRows(t *testing.T) {
	db, ctx, sql := descriptionMigrationFixture(t)
	if err := db.WithContext(ctx).Exec(`UPDATE system_setting SET description='Maintainer custom note' WHERE setting_key='app.brand.title_en_us';
UPDATE system_setting SET deleted_at=CURRENT_TIMESTAMP WHERE setting_key='app.brand.default_avatar';
UPDATE system_setting SET is_builtin=0 WHERE setting_key='auth.captcha.slide_padding';
UPDATE system_setting SET setting_key='custom.description' WHERE setting_key='app.legal.privacy_policy'`).Error; err != nil {
		t.Fatal(err)
	}
	var before string
	query := `SELECT jsonb_agg(to_jsonb(s) ORDER BY id)::text FROM system_setting s
WHERE setting_key IN ('app.brand.title_en_us','app.brand.default_avatar','auth.captcha.slide_padding','custom.description')`
	if err := db.WithContext(ctx).Raw(query).Scan(&before).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(sql).Error }); err != nil {
		t.Fatal(err)
	}
	var after string
	if err := db.WithContext(ctx).Raw(query).Scan(&after).Error; err != nil || after != before {
		t.Fatal("migration overwrote a custom, deleted, non-builtin, or unrelated row", err)
	}
	assertSettingGeneration(t, db, ctx, "global", 2)
}

func TestSettingBuiltinDescriptionMigrationRollsBackCacheFailure(t *testing.T) {
	for _, scenario := range []string{"missing state", "generation overflow", "outbox conflict"} {
		t.Run(scenario, func(t *testing.T) {
			db, ctx, sql := descriptionMigrationFixture(t)
			change := map[string]string{
				"missing state":       `DELETE FROM system_config_cache_state`,
				"generation overflow": `UPDATE system_config_cache_state SET generation=9223372036854775807`,
				"outbox conflict":     `INSERT INTO system_config_cache_outbox(namespace,scope_key,generation) VALUES('system.setting','global',2)`,
			}[scenario]
			if err := db.WithContext(ctx).Exec(change).Error; err != nil {
				t.Fatal(err)
			}
			before := descriptionSnapshot(t, db, ctx, false)
			if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(sql).Error }); err == nil {
				t.Fatal("cache failure accepted")
			}
			if got := descriptionSnapshot(t, db, ctx, false); got != before {
				t.Fatal("failure left partial description updates")
			}
		})
	}
}

func TestSettingBuiltinDescriptionMigrationConcurrentReplay(t *testing.T) {
	db, ctx, sql := descriptionMigrationFixture(t)
	results := make(chan error, 2)
	for range 2 {
		go func() {
			results <- db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(sql).Error })
		}()
	}
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	assertSettingGeneration(t, db, ctx, "global", 2)
	assertSettingOutbox(t, db, ctx, "global", 1, 2, false)
}
