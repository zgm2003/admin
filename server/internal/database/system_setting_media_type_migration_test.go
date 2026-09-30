package database_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/gorm"
)

func mediaTypeFixture(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	db, ctx := csvMigrationFixture(t)
	if err := db.WithContext(ctx).Exec(`ALTER TABLE system_setting ADD CONSTRAINT ck_system_setting_value_type CHECK(value_type IN(1,2,3,4));
INSERT INTO system_setting(setting_key,value,value_type,is_enabled,is_builtin) VALUES
('app.brand.default_avatar','avatar/.admin-storage/v2/p1/r1/c1/v1/2026/09/30/0123456789abcdef0123456789abcdef.png',1,1,1),
('message.mail.recipient_rule.import_template_object_key','file/.admin-storage/v2/p1/r1/c1/v1/2026/09/30/0123456789abcdef0123456789abcdef.csv',1,1,1),
('app.brand.title_zh_cn','Title',1,1,1);`).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func mediaTypeSQL(t *testing.T) string {
	t.Helper()
	script, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-09-30-system-setting-media-value-type.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(script)
}

func TestSettingMediaTypeMigrationPreservesValuesAndIsIdempotent(t *testing.T) {
	db, ctx := mediaTypeFixture(t)
	var beforeValues string
	if err := db.WithContext(ctx).Raw(`SELECT jsonb_agg(jsonb_build_object('id',id,'key',setting_key,'value',value) ORDER BY id)::text FROM system_setting`).Scan(&beforeValues).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(mediaTypeSQL(t)).Error; err != nil {
		t.Fatal(err)
	}
	var afterValues string
	if err := db.WithContext(ctx).Raw(`SELECT jsonb_agg(jsonb_build_object('id',id,'key',setting_key,'value',value) ORDER BY id)::text FROM system_setting`).Scan(&afterValues).Error; err != nil {
		t.Fatal(err)
	}
	if afterValues != beforeValues {
		t.Fatal("migration rewrote setting IDs, keys or strings")
	}
	var valid bool
	if err := db.WithContext(ctx).Raw(`SELECT
 (SELECT count(*)=2 FROM system_setting WHERE value_type=5) AND
 (SELECT value_type=1 FROM system_setting WHERE setting_key='app.brand.title_zh_cn') AND
 (SELECT generation=14 FROM system_config_cache_generation WHERE namespace='system.setting') AND
 (SELECT generation=3 FROM system_config_cache_generation WHERE namespace='message.mail') AND
 (SELECT count(*)=1 FROM system_config_cache_outbox WHERE namespace='system.setting' AND generation=14)`).Scan(&valid).Error; err != nil || !valid {
		t.Fatalf("valid=%v err=%v", valid, err)
	}
	before := csvMigrationSnapshot(t, db, ctx)
	if err := db.WithContext(ctx).Exec(mediaTypeSQL(t)).Error; err != nil {
		t.Fatal(err)
	}
	if csvMigrationSnapshot(t, db, ctx) != before {
		t.Fatal("rerun changed facts")
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO system_setting(setting_key,value,value_type,is_enabled,is_builtin) VALUES('app.media.custom','',5,1,0)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO system_setting(setting_key,value,value_type,is_enabled,is_builtin) VALUES('app.media.invalid','',6,1,0)`).Error; err == nil {
		t.Fatal("type 6 accepted")
	}
}

func TestSettingMediaTypeMigrationRejectsBadFactsAndRollsBack(t *testing.T) {
	for _, statement := range []string{
		`UPDATE system_setting SET value='https://example.com/file.csv' WHERE setting_key='message.mail.recipient_rule.import_template_object_key'`,
		`UPDATE system_setting SET value_type=2 WHERE setting_key='app.brand.default_avatar'`,
		`DELETE FROM system_setting WHERE setting_key='app.brand.default_avatar'`,
		`DELETE FROM system_config_cache_generation WHERE namespace='system.setting'`,
	} {
		t.Run(statement, func(t *testing.T) {
			db, ctx := mediaTypeFixture(t)
			if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
				t.Fatal(err)
			}
			before := csvMigrationSnapshot(t, db, ctx)
			if err := db.WithContext(ctx).Exec(mediaTypeSQL(t)).Error; err == nil {
				t.Fatal("bad source accepted")
			}
			if csvMigrationSnapshot(t, db, ctx) != before {
				t.Fatal("failed migration changed rows or generations")
			}
			var definition string
			if err := db.WithContext(ctx).Raw(`SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid='system_setting'::regclass AND conname='ck_system_setting_value_type'`).Scan(&definition).Error; err != nil {
				t.Fatal(err)
			}
			if definition != "CHECK ((value_type = ANY (ARRAY[1, 2, 3, 4])))" {
				t.Fatalf("constraint changed: %s", definition)
			}
		})
	}
}
