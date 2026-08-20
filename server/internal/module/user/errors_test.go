package user

import (
	"errors"
	"net/http"
	"reflect"
	"testing"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
)

func TestUserErrorsHaveStableContractsAndPreserveCauses(t *testing.T) {
	cause := errors.New("internal detail")
	tests := []struct {
		name       string
		got        *apperror.Error
		wantStatus int
		wantCode   int
		wantKey    i18n.MessageKey
	}{
		{name: "not found", got: userNotFound(cause), wantStatus: http.StatusNotFound, wantCode: 16000, wantKey: i18n.KeyUserNotFound},
		{name: "username conflict", got: userUsernameConflict(cause), wantStatus: http.StatusConflict, wantCode: 16001, wantKey: i18n.KeyUserUsernameConflict},
		{name: "self operation", got: userSelfOperation(cause), wantStatus: http.StatusConflict, wantCode: 16002, wantKey: i18n.KeyUserSelfOperation},
		{name: "super admin protected", got: userSuperAdminProtected(cause), wantStatus: http.StatusForbidden, wantCode: 16003, wantKey: i18n.KeyUserSuperAdminProtected},
		{name: "last super admin", got: userLastSuperAdmin(cause), wantStatus: http.StatusConflict, wantCode: 16004, wantKey: i18n.KeyUserLastSuperAdmin},
		{name: "invalid roles", got: userInvalidRoles(cause), wantStatus: http.StatusBadRequest, wantCode: 16005, wantKey: i18n.KeyUserInvalidRoles},
		{name: "role not found", got: userRoleNotFound(cause), wantStatus: http.StatusNotFound, wantCode: 16006, wantKey: i18n.KeyUserRoleNotFound},
		{name: "data invalid", got: userDataInvalid(cause), wantStatus: http.StatusInternalServerError, wantCode: 16007, wantKey: i18n.KeyUserDataInvalid},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got.HTTPStatus != test.wantStatus || test.got.Code != test.wantCode || test.got.MessageKey != test.wantKey {
				t.Fatalf("error = %+v", test.got)
			}
			if !reflect.DeepEqual(test.got.Params, map[string]string(nil)) {
				t.Fatalf("params = %#v, want empty", test.got.Params)
			}
			if !errors.Is(test.got, cause) {
				t.Fatal("user error does not preserve its cause")
			}
		})
	}
}
