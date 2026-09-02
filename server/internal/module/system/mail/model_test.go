package mail

import (
	"encoding/json"
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
	got := map[string]string{ConfigTable: (Config{}).TableName(), TemplateTable: (Template{}).TableName(), LogTable: (Log{}).TableName(), VerificationTable: (Verification{}).TableName(), RecipientRuleTable: (RecipientRule{}).TableName()}
	for key, value := range got {
		if key != value {
			t.Fatalf("table constant %q maps to %q", key, value)
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
