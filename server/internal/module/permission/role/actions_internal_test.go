package role

import (
	"context"
	"testing"

	"admin/server/internal/shared/yesno"
)

func TestRoleActionPolicyMatrix(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  ListItem
		want Actions
	}{
		{"ordinary", ListItem{Code: "staff", IsEnabled: yesno.Yes}, Actions{true, true, true, true, true}},
		{"super", ListItem{Code: CodeSuperAdmin, IsEnabled: yesno.Yes}, Actions{}},
		{"registered default", ListItem{Code: CodeRegisteredUser, IsDefault: yesno.Yes, IsEnabled: yesno.Yes}, Actions{Authorize: true}},
		{"default", ListItem{Code: "staff", IsDefault: yesno.Yes, IsEnabled: yesno.Yes}, Actions{Update: true, Authorize: true}},
		{"disabled", ListItem{Code: "staff", IsEnabled: yesno.No}, Actions{Update: true, Status: true, Delete: true, Authorize: true}},
		{"used", ListItem{Code: "staff", IsEnabled: yesno.Yes, UserCount: 4}, Actions{Update: true, Status: true, SetDefault: true, Authorize: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, labels := roleActions(context.Background(), tc.row)
			if got != tc.want || labels.Update == "" || labels.Delete == "" {
				t.Fatalf("got %+v labels %+v want %+v", got, labels, tc.want)
			}
		})
	}
}
