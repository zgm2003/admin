package chat

import (
	"context"
	"errors"
	"net/http"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
)

func invalid(err error) error {
	var public *apperror.Error
	if errors.As(err, &public) {
		return err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return dependency(err)
	}
	return &apperror.Error{
		HTTPStatus: http.StatusBadRequest, Code: apperror.CodeInvalidRequest,
		MessageKey: i18n.KeyInvalidRequest, Cause: err,
	}
}

func dependency(err error) error { return apperror.DependencyUnavailable(err) }

func mapError(err error) error {
	var public *apperror.Error
	switch {
	case err == nil:
		return nil
	case errors.As(err, &public):
		return err
	case errors.Is(err, ErrConversationNotFound), errors.Is(err, ErrAgentNotFound), errors.Is(err, ErrMessageNotFound):
		return apperror.NotFound(err)
	default:
		return dependency(err)
	}
}
