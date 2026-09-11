package operationlog

import (
	"net/http"
	"strings"
	"testing"
)

func TestFindRuleMatchesRateLimitPolicyUpdate(t *testing.T) {
	rule, ok := FindRule(http.MethodPut, "/api/admin/v1/message/mail/rate-limit-policy/:platformId/:key")
	if !ok {
		t.Fatal("rate limit policy update rule is not registered")
	}
	if rule.Module != "mail" || rule.Action != "mail.rate-limit.update" {
		t.Fatalf("rule = %+v", rule)
	}
	if !rule.CaptureRequest || !rule.CaptureResponse {
		t.Fatalf("rate limit update must capture request and response, got %+v", rule)
	}
}

func TestFindRuleDoesNotMatchUnknownRoute(t *testing.T) {
	if _, ok := FindRule(http.MethodPut, "/api/admin/v1/message/mail/rate-limit-policy/:key"); ok {
		t.Fatal("the list route must not match the update rule")
	}
}

func TestFindRuleRegistersSMSAndIdentityOperationsWithoutSensitiveBodies(t *testing.T) {
	checks := []struct {
		method, route, action string
		captureRequest        bool
	}{
		{http.MethodPut, "/api/admin/v1/message/sms/config", "sms.config.update", false},
		{http.MethodPost, "/api/admin/v1/message/sms/test", "sms.test", false},
		{http.MethodPut, "/api/admin/v1/message/sms/recipient-rule/:id", "sms.rule.update", true},
		{http.MethodGet, "/api/admin/v1/message/sms/log/:id", "sms.log.detail", false},
		{http.MethodPost, "/api/admin/v1/user/phone/send-code", "user.phone.update", false},
		{http.MethodPut, "/api/admin/v1/user/phone", "user.phone.update", false},
		{http.MethodPost, "/api/admin/v1/user/email/send-code", "user.email.update", false},
		{http.MethodPut, "/api/admin/v1/user/email", "user.email.update", false},
		{http.MethodPut, "/api/admin/v1/user/password/by-code", "user.password.update", true},
	}
	for _, check := range checks {
		rule, ok := FindRule(check.method, check.route)
		if !ok || rule.Action != check.action || rule.CaptureRequest != check.captureRequest {
			t.Fatalf("rule %s %s = %+v found=%v", check.method, check.route, rule, ok)
		}
	}
}

func TestSanitizeValueMasksPhoneAccountAndPatternFields(t *testing.T) {
	value, err := SanitizeJSON([]byte(`{"phone":"+8615671628271","nextPhone":"+8613800000000","account":"user@example.com","pattern":"+86156","code":"123456"}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(value), "15671628271") || strings.Contains(string(value), "13800000000") || strings.Contains(string(value), "user@example.com") || strings.Contains(string(value), "+86156") {
		t.Fatalf("sensitive operation summary leaked: %s", value)
	}
}
