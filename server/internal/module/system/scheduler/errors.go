package scheduler

import (
	"admin/server/internal/shared/i18n"
	"context"
	"errors"
)

var (
	ErrInvalidTaskDefinition = errors.New("scheduler task definition is invalid")
	ErrDuplicateTaskType     = errors.New("scheduler task type is duplicated")
	ErrDuplicateBuiltinKey   = errors.New("scheduler builtin key is duplicated")
	ErrUnknownTaskType       = errors.New("scheduler task type is unknown")
	ErrInvalidPayload        = errors.New("scheduler task payload is invalid")
	ErrInvalidCron           = errors.New("scheduler cron expression is invalid")
	ErrInvalidTimezone       = errors.New("scheduler timezone is invalid")
)

// executionErrorMessage exposes fixed public diagnostics, never raw dependency errors.
func executionErrorMessage(ctx context.Context, class string) (string, error) {
	key := i18n.KeySchedulerErrorTaskFailed
	switch class {
	case "timeout":
		key = i18n.KeySchedulerErrorTimeout
	case "canceled":
		key = i18n.KeySchedulerErrorCanceled
	case "unknown-task-type":
		key = i18n.KeySchedulerErrorUnknownTask
	case "invalid-payload":
		key = i18n.KeySchedulerErrorInvalidPayload
	case "worker-lost":
		key = i18n.KeySchedulerErrorWorkerLost
	case "publish":
		key = i18n.KeySchedulerErrorPublish
	case "transport-lost":
		key = i18n.KeySchedulerErrorTransportLost
	}
	return i18n.Translate(i18n.LocaleFromContext(ctx), key, nil)
}
