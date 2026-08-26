package menu

import (
	"net/http"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
)

const (
	CodeMenuTreeInvalid       = 14000
	CodeMenuNotFound          = 14001
	CodeMenuCodeConflict      = 14002
	CodeMenuPathConflict      = 14003
	CodeMenuInvalidParent     = 14004
	CodeMenuCycleDetected     = 14005
	CodeMenuProtected         = 14006
	CodeMenuParentDisabled    = 14007
	CodeMenuStructureConflict = 14008
	CodeMenuInvalidFields     = 14009
)

func menuTreeInvalid(cause error) *apperror.Error {
	return newMenuError(http.StatusInternalServerError, CodeMenuTreeInvalid, i18n.KeyMenuTreeInvalid, nil, cause)
}

func menuNotFound(cause error) *apperror.Error {
	return newMenuError(http.StatusNotFound, CodeMenuNotFound, i18n.KeyMenuNotFound, nil, cause)
}

func menuCodeConflict(code string, cause error) *apperror.Error {
	return newMenuError(http.StatusConflict, CodeMenuCodeConflict, i18n.KeyMenuCodeConflict, map[string]string{"code": code}, cause)
}

func menuPathConflict(path string, cause error) *apperror.Error {
	return newMenuError(http.StatusConflict, CodeMenuPathConflict, i18n.KeyMenuPathConflict, map[string]string{"path": path}, cause)
}

func menuInvalidParent(cause error) *apperror.Error {
	return newMenuError(http.StatusBadRequest, CodeMenuInvalidParent, i18n.KeyMenuInvalidParent, nil, cause)
}

func menuCycleDetected(cause error) *apperror.Error {
	return newMenuError(http.StatusBadRequest, CodeMenuCycleDetected, i18n.KeyMenuCycleDetected, nil, cause)
}

func menuProtected(code string, cause error) *apperror.Error {
	return newMenuError(http.StatusConflict, CodeMenuProtected, i18n.KeyMenuProtected, map[string]string{"code": code}, cause)
}

func menuParentDisabled(code string, cause error) *apperror.Error {
	return newMenuError(http.StatusConflict, CodeMenuParentDisabled, i18n.KeyMenuParentDisabled, map[string]string{"code": code}, cause)
}

func menuStructureConflict(code string, cause error) *apperror.Error {
	return newMenuError(http.StatusConflict, CodeMenuStructureConflict, i18n.KeyMenuStructureConflict, map[string]string{"code": code}, cause)
}

func menuInvalidFields(cause error) *apperror.Error {
	return newMenuError(http.StatusBadRequest, CodeMenuInvalidFields, i18n.KeyMenuInvalidFields, nil, cause)
}

func newMenuError(httpStatus, code int, key i18n.MessageKey, params map[string]string, cause error) *apperror.Error {
	return &apperror.Error{
		HTTPStatus: httpStatus,
		Code:       code,
		MessageKey: key,
		Params:     params,
		Cause:      cause,
	}
}
