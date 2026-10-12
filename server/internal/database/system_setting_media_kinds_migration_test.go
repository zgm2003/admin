package database_test

import (
	"os"
	"path/filepath"
	"testing"
)

func mediaKindsSQL(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-10-12-system-setting-media-kinds.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSettingMediaKindsMigration(t *testing.T) {
	db, ctx := csvMigrationFixture(t)
	if err := db.WithContext(ctx).Exec(`ALTER TABLE system_setting ADD CONSTRAINT ck_system_setting_value_type CHECK(value_type IN(1,2,3,4,5));
 INSERT INTO system_setting(setting_key,value,value_type,is_enabled,is_builtin) VALUES
 ('app.brand.default_avatar','setting/.admin-storage/v2/p1/r1/c1/v1/2026/10/12/0123456789abcdef0123456789abcdef.png',5,1,1),
 ('message.mail.recipient_rule.import_template_object_key','',5,1,1),
 ('app.assets.clip','setting/.admin-storage/v2/p1/r1/c1/v1/2026/10/12/0123456789abcdef0123456789abcdef.mp4',5,1,0),
 ('app.assets.file','',5,1,0);`).Error; err != nil {
		t.Fatal(err)
	}
	var before string
	query := `SELECT jsonb_agg(jsonb_build_object('id',id,'key',setting_key,'value',value) ORDER BY id)::text FROM system_setting`
	if err := db.WithContext(ctx).Raw(query).Scan(&before).Error; err != nil {
		t.Fatal(err)
	}
	sql := mediaKindsSQL(t)
	if err := db.WithContext(ctx).Exec(sql).Error; err != nil {
		t.Fatal(err)
	}
	var after string
	if err := db.WithContext(ctx).Raw(query).Scan(&after).Error; err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("migration changed IDs, keys or object keys")
	}
	var valid bool
	if err := db.WithContext(ctx).Raw(`SELECT
 (SELECT count(*)=2 FROM system_setting WHERE value_type=6) AND
 (SELECT count(*)=2 FROM system_setting WHERE value_type=5) AND
 (SELECT generation=14 FROM system_config_cache_state WHERE namespace='system.setting') AND
 (SELECT generation=3 FROM system_config_cache_state WHERE namespace='message.mail') AND
 (SELECT count(*)=1 FROM system_config_cache_outbox WHERE namespace='system.setting' AND generation=14)`).Scan(&valid).Error; err != nil || !valid {
		t.Fatalf("valid=%v err=%v", valid, err)
	}
	snapshot := csvMigrationSnapshot(t, db, ctx)
	if err := db.WithContext(ctx).Exec(sql).Error; err != nil {
		t.Fatal(err)
	}
	if csvMigrationSnapshot(t, db, ctx) != snapshot {
		t.Fatal("rerun changed facts")
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO system_setting(setting_key,value,value_type,is_enabled,is_builtin) VALUES('app.assets.new','',6,1,0)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO system_setting(setting_key,value,value_type,is_enabled,is_builtin) VALUES('app.assets.image_attachment','setting/.admin-storage/v2/p1/r1/c1/v1/2026/10/12/1123456789abcdef0123456789abcdef.png',5,1,0)`).Error; err != nil {
		t.Fatal(err)
	}
	snapshot = csvMigrationSnapshot(t, db, ctx)
	if err := db.WithContext(ctx).Exec(sql).Error; err != nil {
		t.Fatal(err)
	}
	if csvMigrationSnapshot(t, db, ctx) != snapshot {
		t.Fatal("rerun reclassified a newly created file attachment")
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO system_setting(setting_key,value,value_type,is_enabled,is_builtin) VALUES('app.assets.bad','',7,1,0)`).Error; err == nil {
		t.Fatal("unknown type accepted")
	}
}

func TestSettingMediaKindsMigrationRollsBack(t *testing.T) {
	db, ctx := csvMigrationFixture(t)
	if err := db.WithContext(ctx).Exec(`ALTER TABLE system_setting ADD CONSTRAINT ck_system_setting_value_type CHECK(value_type IN(1,2,3,4,5));
 INSERT INTO system_setting(setting_key,value,value_type,is_enabled,is_builtin) VALUES ('app.brand.default_avatar','',5,1,1),('message.mail.recipient_rule.import_template_object_key','',5,1,1);
 CREATE FUNCTION reject_media_kind() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced write failure'; END $$;
 CREATE TRIGGER reject_media_kind BEFORE UPDATE ON system_setting FOR EACH ROW EXECUTE FUNCTION reject_media_kind();`).Error; err != nil {
		t.Fatal(err)
	}
	before := csvMigrationSnapshot(t, db, ctx)
	if err := db.WithContext(ctx).Exec(mediaKindsSQL(t)).Error; err == nil {
		t.Fatal("forced failure accepted")
	}
	if csvMigrationSnapshot(t, db, ctx) != before {
		t.Fatal("failure changed data or versions")
	}
	var definition string
	if err := db.WithContext(ctx).Raw(`SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid='system_setting'::regclass AND conname='ck_system_setting_value_type'`).Scan(&definition).Error; err != nil {
		t.Fatal(err)
	}
	if definition != "CHECK ((value_type = ANY (ARRAY[1, 2, 3, 4, 5])))" {
		t.Fatalf("failure changed constraint: %s", definition)
	}
}
