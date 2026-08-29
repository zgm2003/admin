package userrole

import "testing"

func TestUserRoleTableName(t *testing.T) {
	if got := (UserRole{}).TableName(); got != "rbac_user_role" {
		t.Fatalf("table name = %q", got)
	}
}
