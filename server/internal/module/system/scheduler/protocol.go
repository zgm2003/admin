package scheduler

import (
	"encoding/json"
	"time"
)

// Schedule is the durable definition of a recurring task.
type Schedule struct {
	ID             int64
	Name           string
	Description    string
	TaskType       string
	CronExpression string
	Timezone       string
	Params         json.RawMessage
	IsEnabled      int16          `gorm:"column:is_enabled"`
	Status         ScheduleStatus `gorm:"-"`
	NextRunAt      *time.Time
	BuiltinKey     *string
	CreatedBy      *int64
	UpdatedBy      *int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

func (Schedule) TableName() string { return "system_scheduler_schedule" }

type Job struct {
	ID                int64
	ScheduleID        *int64
	TaskType          string
	Payload           json.RawMessage
	TriggerSource     TriggerSource
	SourceKey         string
	ScheduledAt       time.Time
	AvailableAt       time.Time
	Status            JobStatus
	AttemptCount      int
	MaxAttempts       int
	Queue             string
	TimeoutSeconds    int
	PublishToken      string
	PublishLeaseUntil *time.Time
	RunToken          string
	RunLeaseUntil     *time.Time
	WorkerID          string
	RetryOfJobID      *int64
	TriggeredBy       *int64
	ErrorClass        string
	LastError         string
	CompletedAt       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (Job) TableName() string { return "system_scheduler_job" }

type Run struct {
	ID            int64
	JobID         int64
	AttemptNo     int
	Status        RunStatus
	WorkerID      string
	StartedAt     time.Time
	FinishedAt    *time.Time
	DurationMS    *int64
	ErrorClass    string
	ErrorMessage  string
	ResultSummary string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (Run) TableName() string { return "system_scheduler_run" }

type DueSchedule struct {
	Schedule
}

type PublishableJob struct {
	Job
	DispatchToken string
}

type RunningJob struct {
	Job
	Run Run
}

type ScheduleInput struct {
	Name           string
	Description    string
	TaskType       string
	CronExpression string
	Timezone       string
	Params         json.RawMessage
	Enabled        bool
}

type ScheduleQuery struct {
	Status   *ScheduleStatus
	TaskType string
	Limit    int
	AfterID  int64
}

type JobQuery struct {
	ScheduleID *int64
	Status     *JobStatus
	TaskType   string
	Limit      int
	AfterID    int64
}

type RunQuery struct {
	JobID   int64
	Limit   int
	AfterID int64
}

type JobEnvelope struct {
	SchemaVersion int    `json:"schemaVersion"`
	JobID         int64  `json:"jobId"`
	Attempt       int    `json:"attempt"`
	DispatchToken string `json:"dispatchToken"`
}
