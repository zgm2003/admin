package profile

import (
	"reflect"
	"testing"
	"time"
)

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
