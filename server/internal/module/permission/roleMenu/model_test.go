package rolemenu

import "testing"

func TestRoleMenuTableName(t *testing.T) {
	if got := (RoleMenu{}).TableName(); got != "permission_role_menu" {
		t.Fatalf("table name = %q", got)
	}
}
