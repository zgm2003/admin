package apperror

import (
	"net/http"

	"admin/server/internal/shared/i18n"
)

const (
	CodeInternal              = 10000
	CodeInvalidRequest        = 10001
	CodeUnauthorized          = 10002
	CodeForbidden             = 10003
	CodeNotFound              = 10004
	CodeConflict              = 10005
	CodeDependencyUnavailable = 10006
)

type Error struct {
	HTTPStatus int
	Code       int
	MessageKey i18n.MessageKey
	Params     map[string]string
	Cause      error
}

func (e *Error) Error() string {
	return string(e.MessageKey)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func InvalidRequest(cause error) *Error {
	return newError(http.StatusBadRequest, CodeInvalidRequest, i18n.KeyInvalidRequest, nil, cause)
}

func Unauthorized(cause error) *Error {
	return newError(http.StatusUnauthorized, CodeUnauthorized, i18n.KeyUnauthorized, nil, cause)
}

func Forbidden(cause error) *Error {
	return newError(http.StatusForbidden, CodeForbidden, i18n.KeyForbidden, nil, cause)
}

func ForbiddenWithParams(key i18n.MessageKey, params map[string]string, cause error) *Error {
	return newError(http.StatusForbidden, CodeForbidden, key, params, cause)
}

func NotFound(cause error) *Error {
	return newError(http.StatusNotFound, CodeNotFound, i18n.KeyNotFound, nil, cause)
}

func Conflict(key i18n.MessageKey, params map[string]string, cause error) *Error {
	return newError(http.StatusConflict, CodeConflict, key, params, cause)
}

func DependencyUnavailable(cause error) *Error {
	return newError(http.StatusServiceUnavailable, CodeDependencyUnavailable, i18n.KeyDependencyUnavailable, nil, cause)
}

func Internal(cause error) *Error {
	return newError(http.StatusInternalServerError, CodeInternal, i18n.KeyInternal, nil, cause)
}

func newError(httpStatus, code int, key i18n.MessageKey, params map[string]string, cause error) *Error {
	return &Error{HTTPStatus: httpStatus, Code: code, MessageKey: key, Params: params, Cause: cause}
}
