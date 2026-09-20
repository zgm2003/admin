package scheduler

import "errors"

var (
	ErrInvalidTaskDefinition = errors.New("scheduler task definition is invalid")
	ErrDuplicateTaskType     = errors.New("scheduler task type is duplicated")
	ErrDuplicateBuiltinKey   = errors.New("scheduler builtin key is duplicated")
	ErrUnknownTaskType       = errors.New("scheduler task type is unknown")
	ErrInvalidPayload        = errors.New("scheduler task payload is invalid")
	ErrInvalidCron           = errors.New("scheduler cron expression is invalid")
	ErrInvalidTimezone       = errors.New("scheduler timezone is invalid")
)
