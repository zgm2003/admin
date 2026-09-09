package mail

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	mailconfig "admin/server/internal/module/message/mail/config"
	maillog "admin/server/internal/module/message/mail/log"
	logverification "admin/server/internal/module/message/mail/logVerification"
	ratelimitpolicy "admin/server/internal/module/message/mail/rateLimitPolicy"
	recipientrule "admin/server/internal/module/message/mail/recipientRule"
	mailtemplate "admin/server/internal/module/message/mail/template"
)

func TestMailTableNames(t *testing.T) {
	got := map[string]string{
		"config constant":         mailconfig.Table,
		"config model":            (Config{}).TableName(),
		"template constant":       mailtemplate.Table,
		"template model":          (Template{}).TableName(),
		"log constant":            maillog.Table,
		"log model":               (Log{}).TableName(),
		"verification constant":   logverification.Table,
		"verification model":      (Verification{}).TableName(),
		"recipient rule constant": recipientrule.Table,
		"recipient rule model":    (RecipientRule{}).TableName(),
	}
	want := map[string]string{
		"config constant":         "message_mail_config",
		"config model":            "message_mail_config",
		"template constant":       "message_mail_template",
		"template model":          "message_mail_template",
		"log constant":            "message_mail_log",
		"log model":               "message_mail_log",
		"verification constant":   "message_mail_log_verification",
		"verification model":      "message_mail_log_verification",
		"recipient rule constant": "message_mail_recipient_rule",
		"recipient rule model":    "message_mail_recipient_rule",
	}
	for name, value := range got {
		if value != want[name] {
			t.Fatalf("%s = %q, want %q", name, value, want[name])
		}
	}
}

func TestMailConfigurationModelsAreGlobalWhileDeliveryFactsKeepPlatform(t *testing.T) {
	for name, model := range map[string]any{
		"config": Config{}, "template": Template{}, "recipient rule": RecipientRule{},
	} {
		if _, found := reflect.TypeOf(model).FieldByName("PlatformID"); found {
			t.Fatalf("%s still belongs to an authentication platform", name)
		}
	}
	for name, model := range map[string]any{"log": Log{}, "verification": Verification{}} {
		if _, found := reflect.TypeOf(model).FieldByName("PlatformID"); !found {
			t.Fatalf("%s lost its source platform audit field", name)
		}
		if _, found := reflect.TypeOf(model).FieldByName("DeletedAt"); found {
			t.Fatalf("immutable %s exposes a soft-delete field", name)
		}
	}
}

func TestRateLimitPolicyTimestampFieldsUseTimestamptzTags(t *testing.T) {
	typeOfPolicy := reflect.TypeOf(RateLimitPolicy{})
	for _, fieldName := range []string{"CreatedAt", "UpdatedAt"} {
		field, ok := typeOfPolicy.FieldByName(fieldName)
		if !ok {
			t.Fatalf("RateLimitPolicy.%s is missing", fieldName)
		}
		if !strings.Contains(field.Tag.Get("gorm"), "type:timestamptz") {
			t.Fatalf("RateLimitPolicy.%s gorm tag = %q, want type:timestamptz", fieldName, field.Tag.Get("gorm"))
		}
	}
}

func TestMailPermissionCodesUseMessageDomain(t *testing.T) {
	got := []string{
		PermissionView,
		PermissionList,
		maillog.PermissionDetail,
		mailconfig.PermissionUpdate,
		mailconfig.PermissionDelete,
		PermissionTest,
		mailtemplate.PermissionUpdate,
		mailtemplate.PermissionStatus,
		recipientrule.PermissionCreate,
		recipientrule.PermissionUpdate,
		recipientrule.PermissionStatus,
		recipientrule.PermissionDelete,
		ratelimitpolicy.PermissionUpdate,
	}
	want := []string{
		"message:mail:view",
		"message:mail:list",
		"message:mail:detail",
		"message:mail:config:update",
		"message:mail:config:delete",
		"message:mail:test",
		"message:mail:template:update",
		"message:mail:template:status",
		"message:mail:rule:create",
		"message:mail:rule:update",
		"message:mail:rule:status",
		"message:mail:rule:delete",
		"message:mail:rate-limit:update",
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("permission[%d] = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestAdminTestResultUsesCamelCaseJSON(t *testing.T) {
	body, err := json.Marshal(AdminTestResult{LogID: 12, Status: StatusSent, RequestID: "request-1", MessageID: "message-1"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"logId":12,"status":"sent","requestId":"request-1","messageId":"message-1"}`
	if string(body) != want {
		t.Fatalf("unexpected response: %s", body)
	}
}
