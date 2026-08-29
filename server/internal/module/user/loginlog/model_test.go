package loginlog

import "testing"

func TestLoginLogUsesPersistentTable(t *testing.T) {
	if got := (LoginLog{}).TableName(); got != "user_login_log" {
		t.Fatalf("table name = %q, want user_login_log", got)
	}
}

func TestLoginLogEventValidationRequiresPlatformAndEventShape(t *testing.T) {
	loginType := "password"
	cases := []Event{
		{EventType: "login", LoginType: &loginType, PlatformID: 0, IsSuccess: 1},
		{EventType: "logout", LoginType: &loginType, PlatformID: 1, IsSuccess: 1},
	}
	for _, event := range cases {
		if err := ValidateEvent(event); err == nil {
			t.Fatalf("invalid event was accepted: %+v", event)
		}
	}
}
