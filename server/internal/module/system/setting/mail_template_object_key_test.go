package setting

import (
	"testing"
)

func TestMailRuleTemplateSettingAcceptsOnlyCSVObjectKeys(t *testing.T) {
	key := "message.mail.recipient_rule.import_template_object_key"
	objectKey := "file/.admin-storage/v2/p1/r1/c1/v1/2026/09/30/0123456789abcdef0123456789abcdef.csv"
	for _, value := range []string{"https://example.com/t.csv", "javascript:alert(1)", "file/t.csv", objectKey + "?x=1", "file/.admin-storage/v2/p0/r1/c1/v1/2026/09/30/0123456789abcdef0123456789abcdef.csv"} {
		if err := validateInput(key, value, ValueTypeString, ""); err == nil {
			t.Fatalf("invalid object key accepted: %q", value)
		}
	}
	if err := validateInput(key, "1", ValueTypeNumber, ""); err == nil {
		t.Fatal("numeric template object key accepted")
	}
	if err := validateInput(key, "", ValueTypeString, ""); err != nil {
		t.Fatalf("empty link rejected: %v", err)
	}
	if !isRequiredSetting(key) {
		t.Fatal("template setting can be disabled")
	}
	if err := validateInput(key, objectKey, ValueTypeString, ""); err != nil {
		t.Fatalf("valid object key rejected: %v", err)
	}
}
