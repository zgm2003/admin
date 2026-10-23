package database_test

import (
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
