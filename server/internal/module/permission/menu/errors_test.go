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
	if PermissionList != "permission:menu:list" ||
		PermissionCreate != "permission:menu:create" ||
		PermissionUpdate != "permission:menu:update" ||
		PermissionDelete != "permission:menu:delete" ||
		PermissionRebuildAccessCache != "permission:access-cache:rebuild" {
		t.Fatal("menu protocol constants changed")
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
		{name: "protected", got: menuProtected("access", cause), wantStatus: http.StatusConflict, wantCode: CodeMenuProtected, wantKey: i18n.KeyMenuProtected, wantParams: map[string]string{"code": "access"}},
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
