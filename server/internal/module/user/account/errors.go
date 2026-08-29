package account

import (
	"net/http"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
)

const (
	CodeUserNotFound            = 16000
	CodeUserUsernameConflict    = 16001
	CodeUserSelfOperation       = 16002
	CodeUserSuperAdminProtected = 16003
	CodeUserLastSuperAdmin      = 16004
	CodeUserInvalidRoles        = 16005
	CodeUserRoleNotFound        = 16006
	CodeUserDataInvalid         = 16007
	CodeUserPhoneConflict       = 16008
)

func userNotFound(cause error) *apperror.Error {
	return newUserError(http.StatusNotFound, CodeUserNotFound, i18n.KeyUserNotFound, cause)
}

func userUsernameConflict(cause error) *apperror.Error {
	return newUserError(http.StatusConflict, CodeUserUsernameConflict, i18n.KeyUserUsernameConflict, cause)
}

func userPhoneConflict(cause error) *apperror.Error {
	return newUserError(http.StatusConflict, CodeUserPhoneConflict, i18n.KeyUserPhoneConflict, cause)
}

func userSelfOperation(cause error) *apperror.Error {
	return newUserError(http.StatusConflict, CodeUserSelfOperation, i18n.KeyUserSelfOperation, cause)
}

func userSuperAdminProtected(cause error) *apperror.Error {
	return newUserError(http.StatusForbidden, CodeUserSuperAdminProtected, i18n.KeyUserSuperAdminProtected, cause)
}

func userLastSuperAdmin(cause error) *apperror.Error {
	return newUserError(http.StatusConflict, CodeUserLastSuperAdmin, i18n.KeyUserLastSuperAdmin, cause)
}

func userInvalidRoles(cause error) *apperror.Error {
	return newUserError(http.StatusBadRequest, CodeUserInvalidRoles, i18n.KeyUserInvalidRoles, cause)
}

func userRoleNotFound(cause error) *apperror.Error {
	return newUserError(http.StatusNotFound, CodeUserRoleNotFound, i18n.KeyUserRoleNotFound, cause)
}

func userDataInvalid(cause error) *apperror.Error {
	return newUserError(http.StatusInternalServerError, CodeUserDataInvalid, i18n.KeyUserDataInvalid, cause)
}

func newUserError(httpStatus, code int, key i18n.MessageKey, cause error) *apperror.Error {
	return &apperror.Error{HTTPStatus: httpStatus, Code: code, MessageKey: key, Cause: cause}
}
