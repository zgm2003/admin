package accessversion

import "testing"

func TestAccessVersionTableName(t *testing.T) {
	if got := (Version{}).TableName(); got != "rbac_access_version" {
		t.Fatalf("table name = %q", got)
	}
}
