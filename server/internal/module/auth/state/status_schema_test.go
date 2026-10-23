package authstate

import "testing"

func TestUserStatusSchemaDoesNotChangeSessionsSchema(t *testing.T) {
	user := UserState{SchemaVersion: UserSchemaVersion, State: StateReady, UserID: 42, Generation: "unchanged", Status: true}
	if err := validateUserState(42, user); err != nil {
		t.Fatal(err)
	}
	user.SchemaVersion = 1
	if err := validateUserState(42, user); err == nil {
		t.Fatal("legacy user schema accepted")
	}
	sessions := SessionsState{SchemaVersion: 1, State: StateReady, Platform: "admin", UserID: 42, Generation: "unchanged"}
	if err := validateSessionsState("admin", 42, sessions); err != nil {
		t.Fatal("session schema must remain compatible", err)
	}
}
