package loginlog

import (
	"strings"
	"testing"

	"admin/server/internal/shared/yesno"
)

func TestEventEnumsUseFixedNumericValues(t *testing.T) {
	if EventRegister != 1 || EventLogin != 2 || EventLogout != 3 {
		t.Fatalf("event enum values = %d,%d,%d", EventRegister, EventLogin, EventLogout)
	}
	if LoginPassword != 1 || LoginEmail != 2 || LoginPhone != 3 {
		t.Fatalf("login enum values = %d,%d,%d", LoginPassword, LoginEmail, LoginPhone)
	}
}

func TestValidateEventRequiresTypedShape(t *testing.T) {
	password := LoginPassword
	email := LoginEmail
	cases := []struct {
		name  string
		event Event
		valid bool
	}{
		{name: "register email", event: Event{PlatformID: 1, Account: "u@example.com", EventType: EventRegister, LoginType: &email, IsSuccess: yesno.Yes, ReasonCode: "success"}, valid: true},
		{name: "login password", event: Event{PlatformID: 1, Account: "u@example.com", EventType: EventLogin, LoginType: &password, IsSuccess: yesno.Yes, ReasonCode: "success"}, valid: true},
		{name: "logout no login type", event: Event{PlatformID: 1, Account: "user_abc", EventType: EventLogout, IsSuccess: yesno.Yes, ReasonCode: "success"}, valid: true},
		{name: "unknown event", event: Event{PlatformID: 1, Account: "u", EventType: EventType(99), LoginType: &email, IsSuccess: yesno.Yes, ReasonCode: "success"}},
		{name: "unknown login type", event: Event{PlatformID: 1, Account: "u", EventType: EventLogin, LoginType: pointerLoginType(LoginType(99)), IsSuccess: yesno.Yes, ReasonCode: "success"}},
		{name: "missing login type", event: Event{PlatformID: 1, Account: "u", EventType: EventLogin, IsSuccess: yesno.Yes, ReasonCode: "success"}},
		{name: "logout login type present", event: Event{PlatformID: 1, Account: "u", EventType: EventLogout, LoginType: &email, IsSuccess: yesno.Yes, ReasonCode: "success"}},
		{name: "empty account", event: Event{PlatformID: 1, Account: " ", EventType: EventLogout, IsSuccess: yesno.Yes, ReasonCode: "success"}},
		{name: "invalid success", event: Event{PlatformID: 1, Account: "u", EventType: EventLogout, IsSuccess: yesno.Value(9), ReasonCode: "success"}},
		{name: "too long account", event: Event{PlatformID: 1, Account: strings.Repeat("a", 255), EventType: EventLogout, IsSuccess: yesno.Yes, ReasonCode: "success"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateEvent(tc.event)
			if tc.valid && err != nil {
				t.Fatalf("ValidateEvent() error = %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatalf("ValidateEvent() accepted %+v", tc.event)
			}
		})
	}
}

func pointerLoginType(value LoginType) *LoginType { return &value }
