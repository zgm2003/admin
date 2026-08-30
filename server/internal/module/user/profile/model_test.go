package profile

import (
	"reflect"
	"testing"
	"time"
)

func TestUpdateRequestAcceptsObjectKeyAndRejectsURL(t *testing.T) {
	valid, err := (updateRequest{Username: "alice", Avatar: "avatar/2026/08/30/a.png"}).input()
	if err != nil || valid.Avatar != "avatar/2026/08/30/a.png" {
		t.Fatalf("valid avatar = %+v, err=%v", valid, err)
	}
	if _, err := (updateRequest{Username: "alice", Avatar: "https://cdn.example/avatar/a.png"}).input(); err == nil {
		t.Fatal("avatar URL was accepted")
	}
}

func TestProfileTableAndTimestamps(t *testing.T) {
	if got := (Profile{}).TableName(); got != "user_profile" {
		t.Fatalf("table name = %q", got)
	}
	typ := reflect.TypeOf(Profile{})
	for _, field := range []string{"CreatedAt", "UpdatedAt"} {
		value, ok := typ.FieldByName(field)
		if !ok || value.Type != reflect.TypeOf(time.Time{}) {
			t.Fatalf("missing explicit %s", field)
		}
	}
}
