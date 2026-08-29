package session

import (
	"net/http"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
)

func sessionNotFound(cause error) *apperror.Error {
	return &apperror.Error{
		HTTPStatus: http.StatusNotFound,
		Code:       apperror.CodeNotFound,
		MessageKey: i18n.KeySessionNotFound,
		Cause:      cause,
	}
}

func sessionCurrentProtected(cause error) *apperror.Error {
	return &apperror.Error{
		HTTPStatus: http.StatusForbidden,
		Code:       apperror.CodeForbidden,
		MessageKey: i18n.KeySessionCurrentProtected,
		Cause:      cause,
	}
}
