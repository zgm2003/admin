package apperror_test

import (
	"errors"
	"net/http"
	"reflect"
	"testing"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
)

func TestApplicationErrorConstructorsUseMessageKeys(t *testing.T) {
	cause := errors.New("cause")
	tests := []struct {
		name       string
		got        *apperror.Error
		wantStatus int
		wantCode   int
		wantKey    i18n.MessageKey
	}{
		{name: "internal", got: apperror.Internal(cause), wantStatus: http.StatusInternalServerError, wantCode: apperror.CodeInternal, wantKey: i18n.KeyInternal},
		{name: "invalid request", got: apperror.InvalidRequest(cause), wantStatus: http.StatusBadRequest, wantCode: apperror.CodeInvalidRequest, wantKey: i18n.KeyInvalidRequest},
		{name: "unauthorized", got: apperror.Unauthorized(cause), wantStatus: http.StatusUnauthorized, wantCode: apperror.CodeUnauthorized, wantKey: i18n.KeyUnauthorized},
		{name: "forbidden", got: apperror.Forbidden(cause), wantStatus: http.StatusForbidden, wantCode: apperror.CodeForbidden, wantKey: i18n.KeyForbidden},
		{name: "not found", got: apperror.NotFound(cause), wantStatus: http.StatusNotFound, wantCode: apperror.CodeNotFound, wantKey: i18n.KeyNotFound},
		{name: "dependency unavailable", got: apperror.DependencyUnavailable(cause), wantStatus: http.StatusServiceUnavailable, wantCode: apperror.CodeDependencyUnavailable, wantKey: i18n.KeyDependencyUnavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got.HTTPStatus != test.wantStatus || test.got.Code != test.wantCode || test.got.MessageKey != test.wantKey {
				t.Fatalf("error = %+v", test.got)
			}
			if !errors.Is(test.got, cause) {
				t.Fatal("application error does not unwrap its cause")
			}
			if test.got.Error() != string(test.wantKey) {
				t.Fatalf("Error() = %q, want %q", test.got.Error(), test.wantKey)
			}
		})
	}
}

func TestApplicationErrorConstructorsPreserveExplicitParameters(t *testing.T) {
	cause := errors.New("cause")
	params := map[string]string{"permission": "account:user:create"}

	conflict := apperror.Conflict(i18n.KeyUsernameConflict, nil, cause)
	if conflict.MessageKey != i18n.KeyUsernameConflict || conflict.Code != apperror.CodeConflict {
		t.Fatalf("Conflict() = %+v", conflict)
	}

	forbidden := apperror.ForbiddenWithParams(i18n.KeyPermissionDenied, params, cause)
	if forbidden.MessageKey != i18n.KeyPermissionDenied || !reflect.DeepEqual(forbidden.Params, params) {
		t.Fatalf("ForbiddenWithParams() = %+v", forbidden)
	}
}
