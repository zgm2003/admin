package userrole

import "testing"

func TestUserRoleTableName(t *testing.T) {
	if got := (UserRole{}).TableName(); got != "permission_user_role" {
		t.Fatalf("table name = %q", got)
	}
}
