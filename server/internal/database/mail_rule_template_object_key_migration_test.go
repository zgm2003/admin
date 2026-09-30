package database_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gorm"
)

const templateMigrationObjectKey = "file/.admin-storage/v2/p1/r1/c1/v1/2026/09/30/0123456789abcdef0123456789abcdef.csv"

func templateKeyMigrationSQL(t *testing.T, source, objectKey string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-09-30-mail-rule-template-object-key.sql"))
	if err != nil {
		t.Fatal(err)
	}
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
	return strings.NewReplacer(":'template_source_url'", quote(source), ":'template_object_key'", quote(objectKey)).Replace(string(data))
}

func templateKeyMigrationFixture(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	db, ctx := csvMigrationFixture(t)
	if err := db.WithContext(ctx).Exec(csvMigrationSQL(t)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`UPDATE system_setting SET value='https://example.com/template.csv' WHERE setting_key='message.mail.recipient_rule.import_template_url'`).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func TestMailRuleTemplateObjectKeyMigrationPreservesIDAndIsIdempotent(t *testing.T) {
	db, ctx := templateKeyMigrationFixture(t)
	var beforeID int64
	db.WithContext(ctx).Raw(`SELECT id FROM system_setting WHERE setting_key='message.mail.recipient_rule.import_template_url'`).Scan(&beforeID)
	script := templateKeyMigrationSQL(t, "https://example.com/template.csv", templateMigrationObjectKey)
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatal(err)
	}
	var valid bool
	if err := db.WithContext(ctx).Raw(`SELECT
 (SELECT count(*)=0 FROM system_setting WHERE setting_key='message.mail.recipient_rule.import_template_url') AND
 (SELECT id=? AND value=? AND value_type=1 AND is_builtin=1 AND is_enabled=1 FROM system_setting WHERE setting_key='message.mail.recipient_rule.import_template_object_key') AND
 (SELECT generation=15 FROM system_config_cache_generation WHERE namespace='system.setting') AND
 (SELECT generation=3 FROM system_config_cache_generation WHERE namespace='message.mail') AND
 (SELECT count(*)=1 FROM system_config_cache_outbox WHERE namespace='system.setting' AND generation=15)`, beforeID, templateMigrationObjectKey).Scan(&valid).Error; err != nil || !valid {
		t.Fatalf("valid=%v err=%v", valid, err)
	}
	before := csvMigrationSnapshot(t, db, ctx)
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatal(err)
	}
	if after := csvMigrationSnapshot(t, db, ctx); after != before {
		t.Fatal("rerun changed facts")
	}
}

func TestMailRuleTemplateObjectKeyMigrationRejectsConflictAndInvalidKey(t *testing.T) {
	for _, tc := range []struct{ name, prepare, source, key string }{
		{"changed source", "", "https://example.com/other.csv", templateMigrationObjectKey},
		{"full URL", "", "https://example.com/template.csv", "https://example.com/key.csv"},
		{"legacy path", "", "https://example.com/template.csv", "file/template.csv"},
		{"wrong type", "UPDATE system_setting SET value_type=2", "https://example.com/template.csv", templateMigrationObjectKey},
		{"target conflict", "INSERT INTO system_setting(setting_key,value,value_type,is_builtin,is_enabled) VALUES('message.mail.recipient_rule.import_template_object_key','conflict',1,1,1)", "https://example.com/template.csv", templateMigrationObjectKey},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, ctx := templateKeyMigrationFixture(t)
			if tc.prepare != "" {
				if err := db.WithContext(ctx).Exec(tc.prepare).Error; err != nil {
					t.Fatal(err)
				}
			}
			before := csvMigrationSnapshot(t, db, ctx)
			if err := db.WithContext(ctx).Exec(templateKeyMigrationSQL(t, tc.source, tc.key)).Error; err == nil {
				t.Fatal("invalid migration accepted")
			}
			if after := csvMigrationSnapshot(t, db, ctx); after != before {
				t.Fatal("failed migration changed facts")
			}
		})
	}
}
