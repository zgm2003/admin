package database_test

import (
	"admin/server/internal/database/testschema"
	"fmt"
	"gorm.io/gorm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusColumnsMigrationRenamesEveryEnabledColumn(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-10-23-status-columns.sql"))
	if err != nil {
		t.Fatal(err)
	}
	script := strings.ToLower(string(data))
	for _, table := range []string{
		"user_account",
		"permission_auth_platform",
		"permission_role",
		"permission_menu",
		"message_mail_config",
		"message_mail_template",
		"message_mail_recipient_rule",
		"message_sms_config",
		"message_sms_template",
		"message_sms_recipient_rule",
		"storage_cos_config",
		"storage_upload_rule",
		"system_setting",
		"system_scheduler_schedule",
		"payment_config",
		"payment_recharge_package",
	} {
		want := "('" + table + "')"
		if !strings.Contains(script, want) {
			t.Errorf("migration is missing table %q", table)
		}
	}
	if !strings.Contains(script, "rename column is_enabled to status") {
		t.Fatal("migration does not rename the source column")
	}
}

var statusTables = []string{"user_account", "permission_auth_platform", "permission_role", "permission_menu", "message_mail_config", "message_mail_template", "message_mail_recipient_rule", "message_sms_config", "message_sms_template", "message_sms_recipient_rule", "storage_cos_config", "storage_upload_rule", "system_setting", "system_scheduler_schedule", "payment_config", "payment_recharge_package"}

func TestStatusColumnsMigrationPostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	for _, scenario := range []string{"preserves and repeats", "column conflict", "invalid shape", "index conflict", "constraint conflict"} {
		t.Run(scenario, func(t *testing.T) {
			db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_status_columns")
			fixture := "BEGIN;"
			for _, table := range statusTables {
				ddl := fmt.Sprintf(`CREATE TABLE %s (id BIGINT PRIMARY KEY, is_enabled SMALLINT NOT NULL DEFAULT 1 CHECK (is_enabled IN (0,1)), created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, deleted_at TIMESTAMPTZ, next_run_at TIMESTAMPTZ, cos_config_id BIGINT, platform_id BIGINT, setting_key TEXT); INSERT INTO %s(id,is_enabled) VALUES(1,0),(2,1)`, table, table)
				fixture += ddl + ";"
			}
			for _, ddl := range []string{
				`CREATE INDEX ix_storage_cos_config_enabled_created_at ON storage_cos_config(is_enabled,created_at DESC) WHERE deleted_at IS NULL`,
				`CREATE INDEX ix_storage_upload_rule_config_enabled_created_at ON storage_upload_rule(cos_config_id,is_enabled,created_at DESC) WHERE deleted_at IS NULL`,
				`CREATE UNIQUE INDEX ux_storage_upload_rule_platform_enabled ON storage_upload_rule(platform_id) WHERE is_enabled=1 AND deleted_at IS NULL`,
				`CREATE INDEX ix_system_scheduler_schedule_due ON system_scheduler_schedule(next_run_at,id) WHERE deleted_at IS NULL AND is_enabled=1`,
				`CREATE INDEX ix_system_setting_enabled_key ON system_setting(is_enabled,setting_key) WHERE deleted_at IS NULL`,
			} {
				fixture += ddl + ";"
			}
			if err := db.WithContext(ctx).Exec(fixture + "COMMIT;").Error; err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "column conflict":
				if err := db.Exec(`ALTER TABLE payment_recharge_package ADD COLUMN status SMALLINT`).Error; err != nil {
					t.Fatal(err)
				}
			case "invalid shape":
				if err := db.Exec(`ALTER TABLE payment_recharge_package ALTER COLUMN is_enabled DROP NOT NULL`).Error; err != nil {
					t.Fatal(err)
				}
			case "index conflict":
				if err := db.Exec(`CREATE INDEX ix_system_setting_status_key ON system_setting(id)`).Error; err != nil {
					t.Fatal(err)
				}
			case "constraint conflict":
				if err := db.Exec(`ALTER TABLE message_mail_config ADD CONSTRAINT message_mail_config_status_check CHECK (id>0)`).Error; err != nil {
					t.Fatal(err)
				}
			}
			raw, err := os.ReadFile(filepath.Join(repoRoot(t), "docs/database/2026-10-23-status-columns.sql"))
			if err != nil {
				t.Fatal(err)
			}
			digest := func(column string) string {
				var result string
				q := fmt.Sprintf(`SELECT md5(string_agg(row_to_json(s)::text,',' ORDER BY id)) FROM (SELECT id,%s AS value,created_at,updated_at,deleted_at FROM user_account) s`, column)
				if err := db.WithContext(ctx).Raw(q).Scan(&result).Error; err != nil {
					t.Fatal(err)
				}
				return result
			}
			before := digest("is_enabled")
			apply := func() error {
				return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(string(raw)).Error })
			}
			err = apply()
			if scenario != "preserves and repeats" {
				if err == nil {
					t.Fatal("conflict accepted")
				}
				var count int64
				if err := db.Raw(`SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='user_account' AND column_name='is_enabled'`).Scan(&count).Error; err != nil || count != 1 {
					t.Fatal("partial migration was not rolled back", err, count)
				}
				if digest("is_enabled") != before {
					t.Fatal("rollback changed data")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if digest("status") != before {
				t.Fatal("migration changed rows")
			}
			var first, second string
			indexDigest := `SELECT md5(string_agg(indexrelid::text||pg_get_indexdef(indexrelid),',' ORDER BY indexrelid)) FROM pg_index WHERE indrelid IN (SELECT oid FROM pg_class WHERE relnamespace=(SELECT oid FROM pg_namespace WHERE nspname=current_schema()))`
			if err := db.Raw(indexDigest).Scan(&first).Error; err != nil {
				t.Fatal(err)
			}
			if err := apply(); err != nil {
				t.Fatal("replay", err)
			}
			if err := db.Raw(indexDigest).Scan(&second).Error; err != nil {
				t.Fatal(err)
			}
			if first != second || digest("status") != before {
				t.Fatal("replay changed schema/data")
			}
			for _, table := range statusTables {
				var values []int
				if err := db.Raw("SELECT status FROM " + table + " ORDER BY id").Scan(&values).Error; err != nil || len(values) != 2 || values[0] != 0 || values[1] != 1 {
					t.Fatal("status values changed", table, err, values)
				}
				if err := db.Transaction(func(tx *gorm.DB) error { return tx.Exec("UPDATE " + table + " SET status=2 WHERE id=1").Error }); err == nil {
					t.Fatal("binary CHECK missing", table)
				}
			}
		})
	}
}
