package role

import (
	"net/http"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
)

const (
	CodeRoleNotFound                = 15000
	CodeRoleCodeConflict            = 15001
	CodeRoleNameConflict            = 15002
	CodeRoleSystemProtected         = 15003
	CodeRoleDefaultProtected        = 15004
	CodeRoleUsersAttached           = 15005
	CodeRoleInvalidState            = 15006
	CodeRoleInvalidPermission       = 15007
	CodeRoleSuperAdminAuthorization = 15008
	CodeRoleDataInvalid             = 15009
)

func roleNotFound(cause error) *apperror.Error {
	return newRoleError(http.StatusNotFound, CodeRoleNotFound, i18n.KeyRoleNotFound, nil, cause)
}

func roleCodeConflict(code string, cause error) *apperror.Error {
	return newRoleError(http.StatusConflict, CodeRoleCodeConflict, i18n.KeyRoleCodeConflict, map[string]string{"code": code}, cause)
}

func roleNameConflict(name string, cause error) *apperror.Error {
	return newRoleError(http.StatusConflict, CodeRoleNameConflict, i18n.KeyRoleNameConflict, map[string]string{"name": name}, cause)
}

func roleSystemProtected(code string, cause error) *apperror.Error {
	return newRoleError(http.StatusConflict, CodeRoleSystemProtected, i18n.KeyRoleSystemProtected, map[string]string{"code": code}, cause)
}

func roleDefaultProtected(code string, cause error) *apperror.Error {
	return newRoleError(http.StatusConflict, CodeRoleDefaultProtected, i18n.KeyRoleDefaultProtected, map[string]string{"code": code}, cause)
}

func roleUsersAttached(code string, cause error) *apperror.Error {
	return newRoleError(http.StatusConflict, CodeRoleUsersAttached, i18n.KeyRoleUsersAttached, map[string]string{"code": code}, cause)
}

func roleInvalidState(cause error) *apperror.Error {
	return newRoleError(http.StatusBadRequest, CodeRoleInvalidState, i18n.KeyRoleInvalidState, nil, cause)
}

func roleInvalidPermission(cause error) *apperror.Error {
	return newRoleError(http.StatusBadRequest, CodeRoleInvalidPermission, i18n.KeyRoleInvalidPermission, nil, cause)
}

func roleSuperAdminAuthorization(cause error) *apperror.Error {
	return newRoleError(http.StatusConflict, CodeRoleSuperAdminAuthorization, i18n.KeyRoleSuperAdminAuthorization, nil, cause)
}

func roleDataInvalid(cause error) *apperror.Error {
	return newRoleError(http.StatusInternalServerError, CodeRoleDataInvalid, i18n.KeyRoleDataInvalid, nil, cause)
}

func newRoleError(httpStatus, code int, key i18n.MessageKey, params map[string]string, cause error) *apperror.Error {
	return &apperror.Error{HTTPStatus: httpStatus, Code: code, MessageKey: key, Params: params, Cause: cause}
}
