package scheduler

import (
	"context"
	"encoding/json"
	"time"
)

type ScheduleStatus int16

const (
	ScheduleDisabled ScheduleStatus = 1
	ScheduleEnabled  ScheduleStatus = 2
)

type JobStatus int16

const (
	JobScheduled JobStatus = 1
	JobQueued    JobStatus = 2
	JobRunning   JobStatus = 3
	JobCompleted JobStatus = 4
	JobFailed    JobStatus = 5
	JobCanceled  JobStatus = 6
)

type RunStatus int16

const (
	RunRunning   RunStatus = 1
	RunSucceeded RunStatus = 2
	RunFailed    RunStatus = 3
)

type TriggerSource string

const (
	TriggerCron     TriggerSource = "cron"
	TriggerManual   TriggerSource = "manual"
	TriggerRetry    TriggerSource = "retry"
	TriggerBusiness TriggerSource = "business"
)

type ExecutionContext struct {
	JobID       int64
	Attempt     int
	MaxAttempts int
	TaskType    string
	Payload     json.RawMessage
	ScheduledAt time.Time
	StartedAt   time.Time
}

type TaskExecutor func(context.Context, ExecutionContext) error

type TaskDefinition struct {
	Type           string
	DisplayName    string
	AdminCreatable bool
	BuiltinKey     string
	DefaultParams  json.RawMessage
	ValidateParams func(json.RawMessage) error
	Queue          string
	Timeout        time.Duration
	MaxAttempts    int
	Backoff        func(attempt int) time.Duration
	Execute        TaskExecutor
}

type TaskOption struct {
	Type           string
	DisplayName    string
	AdminCreatable bool
	BuiltinKey     string
	DefaultParams  json.RawMessage
}
