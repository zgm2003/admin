package authplatform

import (
	"errors"
	"net/http"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
)

const (
	CodeNotFound         = 17000
	CodeCodeConflict     = 17001
	CodeBuiltinProtected = 17002
	CodeInvalidPolicy    = 17003
	CodeDisabled         = 17004
	CodeSessionUpdating  = 17005
)

var (
	ErrCodeConflict = errors.New("active authentication platform code conflicts")
	ErrUpdating     = errors.New("authentication platform policy is updating")
)

func notFound(cause error) error {
	return &apperror.Error{HTTPStatus: http.StatusNotFound, Code: CodeNotFound, MessageKey: i18n.KeyAuthPlatformNotFound, Cause: cause}
}

func codeConflict(cause error) error {
	return &apperror.Error{HTTPStatus: http.StatusConflict, Code: CodeCodeConflict, MessageKey: i18n.KeyAuthPlatformCodeConflict, Cause: cause}
}

func builtinProtected(cause error) error {
	return &apperror.Error{HTTPStatus: http.StatusConflict, Code: CodeBuiltinProtected, MessageKey: i18n.KeyAuthPlatformBuiltinProtected, Cause: cause}
}

func invalidPolicy(cause error) error {
	return &apperror.Error{HTTPStatus: http.StatusBadRequest, Code: CodeInvalidPolicy, MessageKey: i18n.KeyAuthPlatformInvalidPolicy, Cause: cause}
}

func disabled(cause error) error {
	return &apperror.Error{HTTPStatus: http.StatusForbidden, Code: CodeDisabled, MessageKey: i18n.KeyAuthPlatformDisabled, Cause: cause}
}

func sessionUpdating(cause error) error {
	return &apperror.Error{HTTPStatus: http.StatusServiceUnavailable, Code: CodeSessionUpdating, MessageKey: i18n.KeyAuthSessionUpdating, Cause: cause}
}

func SessionUpdating(cause error) error {
	return sessionUpdating(cause)
}

func dependencyUnavailable(cause error) error {
	return &apperror.Error{HTTPStatus: http.StatusServiceUnavailable, Code: apperror.CodeDependencyUnavailable, MessageKey: i18n.KeyAuthPlatformUnavailable, Cause: cause}
}
