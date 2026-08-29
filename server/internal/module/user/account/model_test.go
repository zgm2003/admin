package account

import (
	"reflect"
	"testing"
	"time"
)

func TestAccountTableAndTimestamps(t *testing.T) {
	if got := (User{}).TableName(); got != "user_account" {
		t.Fatalf("table name = %q", got)
	}
	typ := reflect.TypeOf(User{})
	for _, field := range []string{"CreatedAt", "UpdatedAt"} {
		value, ok := typ.FieldByName(field)
		if !ok || value.Type != reflect.TypeOf(time.Time{}) {
			t.Fatalf("missing explicit %s", field)
		}
	}
}
