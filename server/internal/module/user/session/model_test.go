package session

import (
	"reflect"
	"testing"
	"time"
)

func TestSessionTableAndTimestamps(t *testing.T) {
	if got := (Session{}).TableName(); got != "user_session" {
		t.Fatalf("table name = %q", got)
	}
	typ := reflect.TypeOf(Session{})
	for _, field := range []string{"CreatedAt", "UpdatedAt"} {
		value, ok := typ.FieldByName(field)
		if !ok || value.Type != reflect.TypeOf(time.Time{}) {
			t.Fatalf("missing explicit %s", field)
		}
	}
}
