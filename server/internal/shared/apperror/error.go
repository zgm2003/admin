package apperror

import "net/http"

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
	Message    string
	Cause      error
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func InvalidRequest(cause error) *Error {
	return newError(http.StatusBadRequest, CodeInvalidRequest, "请求参数错误", cause)
}

func Unauthorized(cause error) *Error {
	return newError(http.StatusUnauthorized, CodeUnauthorized, "未登录或登录已失效", cause)
}

func Forbidden(cause error) *Error {
	return newError(http.StatusForbidden, CodeForbidden, "无权执行该操作", cause)
}

func NotFound(cause error) *Error {
	return newError(http.StatusNotFound, CodeNotFound, "请求的资源不存在", cause)
}

func Conflict(message string, cause error) *Error {
	return newError(http.StatusConflict, CodeConflict, message, cause)
}

func DependencyUnavailable(cause error) *Error {
	return newError(http.StatusServiceUnavailable, CodeDependencyUnavailable, "服务暂未就绪", cause)
}

func Internal(cause error) *Error {
	return newError(http.StatusInternalServerError, CodeInternal, "服务内部错误", cause)
}

func newError(httpStatus, code int, message string, cause error) *Error {
	return &Error{HTTPStatus: httpStatus, Code: code, Message: message, Cause: cause}
}
