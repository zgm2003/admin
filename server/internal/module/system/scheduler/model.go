package scheduler

import (
	"context"
	"encoding/json"
	"time"
)

type ScheduleStatus string

const (
	ScheduleEnabled  ScheduleStatus = "enabled"
	ScheduleDisabled ScheduleStatus = "disabled"
)

type JobStatus string

const (
	JobScheduled JobStatus = "scheduled"
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
	JobCanceled  JobStatus = "canceled"
)

type RunStatus string

const (
	RunRunning   RunStatus = "running"
	RunSucceeded RunStatus = "succeeded"
	RunFailed    RunStatus = "failed"
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
