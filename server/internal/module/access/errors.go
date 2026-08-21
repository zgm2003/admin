package access

import (
	"net/http"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
)

const CodeAccessUpdating = 17006

func accessUpdating(cause error) error {
	return &apperror.Error{
		HTTPStatus: http.StatusServiceUnavailable, Code: CodeAccessUpdating,
		MessageKey: i18n.KeyAccessUpdating, Cause: cause,
	}
}
