package role

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
)

func TestRoleProtocolConstantsAndValidation(t *testing.T) {
	if CodeSuperAdmin != "super_admin" || CodeRegisteredUser != "registered_user" {
		t.Fatal("system role codes changed")
	}
	for _, code := range []string{"abc", "ai_tester", "a12", "a" + strings.Repeat("b", 63)} {
		if !IsValidCode(code) {
			t.Errorf("IsValidCode(%q) = false", code)
		}
	}
	for _, code := range []string{"", "ab", "1role", "Role", "role-name", "role name", "a" + strings.Repeat("b", 64)} {
		if IsValidCode(code) {
			t.Errorf("IsValidCode(%q) = true", code)
		}
	}
	if !IsSystemCode(CodeSuperAdmin) || !IsSystemCode(CodeRegisteredUser) || IsSystemCode("admin") {
		t.Fatal("IsSystemCode accepted or rejected an unexpected code")
	}
}

func TestRoleErrorsHaveStableContractsAndPreserveCauses(t *testing.T) {
	cause := errors.New("internal detail")
	tests := []struct {
		name       string
		got        *apperror.Error
		wantStatus int
		wantCode   int
		wantKey    i18n.MessageKey
		wantParams map[string]string
	}{
		{name: "not found", got: roleNotFound(cause), wantStatus: http.StatusNotFound, wantCode: 15000, wantKey: i18n.KeyRoleNotFound},
		{name: "code conflict", got: roleCodeConflict("ai_tester", cause), wantStatus: http.StatusConflict, wantCode: 15001, wantKey: i18n.KeyRoleCodeConflict, wantParams: map[string]string{"code": "ai_tester"}},
		{name: "name conflict", got: roleNameConflict("AI Tester", cause), wantStatus: http.StatusConflict, wantCode: 15002, wantKey: i18n.KeyRoleNameConflict, wantParams: map[string]string{"name": "AI Tester"}},
		{name: "system protected", got: roleSystemProtected(CodeSuperAdmin, cause), wantStatus: http.StatusConflict, wantCode: 15003, wantKey: i18n.KeyRoleSystemProtected, wantParams: map[string]string{"code": CodeSuperAdmin}},
		{name: "default protected", got: roleDefaultProtected("member", cause), wantStatus: http.StatusConflict, wantCode: 15004, wantKey: i18n.KeyRoleDefaultProtected, wantParams: map[string]string{"code": "member"}},
		{name: "users attached", got: roleUsersAttached("member", cause), wantStatus: http.StatusConflict, wantCode: 15005, wantKey: i18n.KeyRoleUsersAttached, wantParams: map[string]string{"code": "member"}},
		{name: "invalid state", got: roleInvalidState(cause), wantStatus: http.StatusBadRequest, wantCode: 15006, wantKey: i18n.KeyRoleInvalidState},
		{name: "invalid permission", got: roleInvalidPermission(cause), wantStatus: http.StatusBadRequest, wantCode: 15007, wantKey: i18n.KeyRoleInvalidPermission},
		{name: "super admin authorization", got: roleSuperAdminAuthorization(cause), wantStatus: http.StatusConflict, wantCode: 15008, wantKey: i18n.KeyRoleSuperAdminAuthorization},
		{name: "data invalid", got: roleDataInvalid(cause), wantStatus: http.StatusInternalServerError, wantCode: 15009, wantKey: i18n.KeyRoleDataInvalid},
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
				t.Fatal("role error does not preserve its cause")
			}
		})
	}
}
