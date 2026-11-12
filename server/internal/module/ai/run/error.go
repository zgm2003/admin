package run

import (
	"context"
	"errors"
	"net/http"

	"admin/server/internal/module/ai/chat"
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

func insufficient(err error) error {
	return &apperror.Error{
		HTTPStatus: http.StatusPaymentRequired, Code: apperror.CodeInvalidRequest,
		MessageKey: i18n.KeyAIChatInsufficientBalance, Cause: err,
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
	case errors.Is(err, chat.ErrConversationNotFound):
		return apperror.NotFound(err)
	case errors.Is(err, ErrRunNotFound), errors.Is(err, ErrAgentNotFound):
		return apperror.NotFound(err)
	case errors.Is(err, ErrActiveRun):
		return apperror.Conflict(i18n.KeyConflict, nil, err)
	default:
		return dependency(err)
	}
}
