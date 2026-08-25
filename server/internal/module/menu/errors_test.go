package menu

import (
	"errors"
	"net/http"
	"reflect"
	"testing"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
)

func TestMenuPermissionConstants(t *testing.T) {
	if PermissionList != "system:menu:list" ||
		PermissionCreate != "system:menu:create" ||
		PermissionUpdate != "system:menu:update" ||
		PermissionDelete != "system:menu:delete" {
		t.Fatal("menu protocol constants changed")
	}
	rolePermissions := map[string]string{
		"list": PermissionRoleList, "create": PermissionRoleCreate, "update": PermissionRoleUpdate,
		"status": PermissionRoleStatus, "default": PermissionRoleDefault,
		"delete": PermissionRoleDelete, "authorize": PermissionRoleAuthorize,
	}
	for name, got := range rolePermissions {
		if got != "system:role:"+name {
			t.Errorf("role permission %s = %q", name, got)
		}
	}
	userPermissions := map[string]string{"list": PermissionUserList, "update": PermissionUserUpdate, "status": PermissionUserStatus, "delete": PermissionUserDelete, "roles": PermissionUserRoles}
	for name, got := range userPermissions {
		if got != "system:user:"+name {
			t.Errorf("user permission %s = %q", name, got)
		}
	}
	authPlatformPermissions := map[string]string{
		"list": PermissionAuthPlatformList, "create": PermissionAuthPlatformCreate,
		"update": PermissionAuthPlatformUpdate, "status": PermissionAuthPlatformStatus, "delete": PermissionAuthPlatformDelete,
	}
	for name, got := range authPlatformPermissions {
		if got != "system:auth-platform:"+name {
			t.Errorf("authentication platform permission %s = %q", name, got)
		}
	}
}

func TestMenuErrorsHaveStableContractsAndPreserveCauses(t *testing.T) {
	cause := errors.New("database detail")
	tests := []struct {
		name       string
		got        *apperror.Error
		wantStatus int
		wantCode   int
		wantKey    i18n.MessageKey
		wantParams map[string]string
	}{
		{name: "tree invalid", got: menuTreeInvalid(cause), wantStatus: http.StatusInternalServerError, wantCode: CodeMenuTreeInvalid, wantKey: i18n.KeyMenuTreeInvalid},
		{name: "not found", got: menuNotFound(cause), wantStatus: http.StatusNotFound, wantCode: CodeMenuNotFound, wantKey: i18n.KeyMenuNotFound},
		{name: "code conflict", got: menuCodeConflict("reports", cause), wantStatus: http.StatusConflict, wantCode: CodeMenuCodeConflict, wantKey: i18n.KeyMenuCodeConflict, wantParams: map[string]string{"code": "reports"}},
		{name: "path conflict", got: menuPathConflict("/reports", cause), wantStatus: http.StatusConflict, wantCode: CodeMenuPathConflict, wantKey: i18n.KeyMenuPathConflict, wantParams: map[string]string{"path": "/reports"}},
		{name: "invalid parent", got: menuInvalidParent(cause), wantStatus: http.StatusBadRequest, wantCode: CodeMenuInvalidParent, wantKey: i18n.KeyMenuInvalidParent},
		{name: "cycle detected", got: menuCycleDetected(cause), wantStatus: http.StatusBadRequest, wantCode: CodeMenuCycleDetected, wantKey: i18n.KeyMenuCycleDetected},
		{name: "parent disabled", got: menuParentDisabled("reports", cause), wantStatus: http.StatusConflict, wantCode: CodeMenuParentDisabled, wantKey: i18n.KeyMenuParentDisabled, wantParams: map[string]string{"code": "reports"}},
		{name: "structure conflict", got: menuStructureConflict("reports", cause), wantStatus: http.StatusConflict, wantCode: CodeMenuStructureConflict, wantKey: i18n.KeyMenuStructureConflict, wantParams: map[string]string{"code": "reports"}},
		{name: "invalid fields", got: menuInvalidFields(cause), wantStatus: http.StatusBadRequest, wantCode: CodeMenuInvalidFields, wantKey: i18n.KeyMenuInvalidFields},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got.HTTPStatus != test.wantStatus || test.got.Code != test.wantCode || test.got.MessageKey != test.wantKey {
				t.Fatalf("error = %+v", test.got)
			}
			if !reflect.DeepEqual(test.got.Params, test.wantParams) {
				t.Fatalf("params = %#v, want %#v", test.got.Params, test.wantParams)
			}
			if !errors.Is(test.got, cause) {
				t.Fatal("menu error does not unwrap its cause")
			}
		})
	}
}
