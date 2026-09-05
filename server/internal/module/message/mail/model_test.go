package mail

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestFixedTemplatesHaveStableTencentIDs(t *testing.T) {
	want := map[string]int{SceneLogin: 47941, SceneForget: 47942, SceneBindEmail: 47943, SceneChangePassword: 47944}
	got := FixedTemplates()
	if len(got) != 4 {
		t.Fatalf("templates=%d", len(got))
	}
	for _, v := range got {
		if want[v.Scene] != v.TencentTemplateID || len(v.Variables) != 2 {
			t.Fatalf("invalid template: %+v", v)
		}
	}
}
func TestMailTableNames(t *testing.T) {
	got := map[string]string{
		"config constant":         ConfigTable,
		"config model":            (Config{}).TableName(),
		"template constant":       TemplateTable,
		"template model":          (Template{}).TableName(),
		"log constant":            LogTable,
		"log model":               (Log{}).TableName(),
		"verification constant":   VerificationTable,
		"verification model":      (Verification{}).TableName(),
		"recipient rule constant": RecipientRuleTable,
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
		PermissionDetail,
		PermissionConfigUpdate,
		PermissionConfigDelete,
		PermissionTest,
		PermissionTemplateUpdate,
		PermissionTemplateStatus,
		PermissionLogDelete,
		PermissionRuleCreate,
		PermissionRuleUpdate,
		PermissionRuleStatus,
		PermissionRuleDelete,
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
		"message:mail:log:delete",
		"message:mail:rule:create",
		"message:mail:rule:update",
		"message:mail:rule:status",
		"message:mail:rule:delete",
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

func TestTemplateUpdateValuesPersistVariableMaps(t *testing.T) {
	values, err := templateUpdateValues(TemplateUpdateInput{
		Name: "登录验证码", Subject: "登录验证码", TencentTemplateID: 47941,
		Variables: map[string]string{"code": "123456"}, ExampleVariables: map[string]string{"ttl_minutes": "10"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(values["variables"].([]byte)) != `{"code":"123456"}` || string(values["example_variables"].([]byte)) != `{"ttl_minutes":"10"}` {
		t.Fatalf("variable values were not serialized: %#v", values)
	}
}

func TestMailVariableValidationRequiresFixedKeys(t *testing.T) {
	valid := map[string]string{"code": "123456", "ttl_minutes": "10"}
	if err := validateMailVariables(valid, true); err != nil {
		t.Fatalf("valid variables rejected: %v", err)
	}
	for name, value := range map[string]map[string]string{
		"missing code": {"ttl_minutes": "10"},
		"missing ttl":  {"code": "123456"},
		"unknown key":  {"code": "123456", "ttl_minutes": "10", "extra": "x"},
		"empty code":   {"code": "", "ttl_minutes": "10"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateMailVariables(value, true); err == nil {
				t.Fatal("invalid variables accepted")
			}
		})
	}
}

func TestMailConfigEmailRejectsDisplayName(t *testing.T) {
	if _, err := normalizeMailConfigEmail("Admin <admin@example.com>"); err == nil {
		t.Fatal("display-name sender accepted")
	}
	if got, err := normalizeMailConfigEmail(" Admin@Example.COM "); err != nil || got != "admin@example.com" {
		t.Fatalf("plain sender normalization = %q,%v", got, err)
	}
}
