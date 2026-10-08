package setting

import (
	"strings"
	"testing"
	"time"

	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

func TestMailRuleTemplateSettingAcceptsOnlyXlsxObjectKeys(t *testing.T) {
	key := "message.mail.recipient_rule.import_template_object_key"
	objectKey := "setting/.admin-storage/v2/p1/r1/c1/v1/2026/10/08/0123456789abcdef0123456789abcdef.xlsx"
	for _, value := range []string{"https://example.com/t.xlsx", "javascript:alert(1)", "setting/t.xlsx", objectKey + "?x=1", "setting/.admin-storage/v2/p0/r1/c1/v1/2026/10/08/0123456789abcdef0123456789abcdef.xlsx", strings.TrimSuffix(objectKey, ".xlsx") + ".csv", strings.TrimSuffix(objectKey, ".xlsx") + ".xls", strings.TrimSuffix(objectKey, ".xlsx") + ".xlsm"} {
		if err := validateInput(key, value, 5, ""); err == nil {
			t.Fatalf("invalid object key accepted: %q", value)
		}
	}
	if err := validateInput(key, "1", ValueTypeNumber, ""); err == nil {
		t.Fatal("numeric template object key accepted")
	}
	if err := validateInput(key, "", 5, ""); err != nil {
		t.Fatalf("empty link rejected: %v", err)
	}
	if !isRequiredSetting(key) {
		t.Fatal("template setting can be disabled")
	}
	if err := validateInput(key, objectKey, 5, ""); err != nil {
		t.Fatalf("valid object key rejected: %v", err)
	}
}

func TestMailRuleTemplateCacheRejectsOldFileFormats(t *testing.T) {
	key := "setting/.admin-storage/v2/p1/r1/c1/v1/2026/10/08/0123456789abcdef0123456789abcdef.xlsx"
	now := time.Now().UTC()
	row := Record{ID: 13, Key: sharedsetting.MailRecipientRuleImportTemplateObjectKey, Value: key, ValueType: ValueTypeMedia, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	encoded, err := encodeRecordPayload(1, row)
	if err != nil {
		t.Fatalf("valid Excel media record cannot be cached: %v", err)
	}
	decoded, err := decodeRecordPayload(encoded)
	if err != nil || decoded.Value != key || decoded.ValueType != ValueTypeMedia {
		t.Fatalf("media record roundtrip failed: %+v, %v", decoded, err)
	}
	for _, suffix := range []string{".csv", ".xls", ".xlsm"} {
		invalidKey := strings.TrimSuffix(key, ".xlsx") + suffix
		if _, err := decodeRecordPayload(strings.ReplaceAll(encoded, key, invalidKey)); err == nil {
			t.Fatalf("invalid template snapshot accepted: %s", suffix)
		}
		row.Value = invalidKey
		if _, err := encodeRecordPayload(1, row); err == nil {
			t.Fatalf("invalid template source cached: %s", suffix)
		}
	}
}
