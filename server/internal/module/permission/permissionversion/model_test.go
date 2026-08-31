package permissionversion

import "testing"

func TestAccessVersionTableName(t *testing.T) {
	if got := (Version{}).TableName(); got != "permission_access_version" {
		t.Fatalf("table name = %q", got)
	}
}
