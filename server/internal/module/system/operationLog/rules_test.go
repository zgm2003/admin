package operationlog

import (
	"net/http"
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
