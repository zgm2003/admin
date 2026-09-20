package scheduler

import (
	"encoding/json"
	"time"
)

type scheduleResponse struct {
	ID             int64           `json:"id"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	TaskType       string          `json:"taskType"`
	CronExpression string          `json:"cronExpression"`
	Timezone       string          `json:"timezone"`
	Params         json.RawMessage `json:"params"`
	IsEnabled      bool            `json:"isEnabled"`
	NextRunAt      *time.Time      `json:"nextRunAt"`
	BuiltinKey     string          `json:"builtinKey"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

type taskOptionResponse struct {
	Type           string          `json:"type"`
	DisplayName    string          `json:"displayName"`
	AdminCreatable bool            `json:"adminCreatable"`
	BuiltinKey     string          `json:"builtinKey"`
	DefaultParams  json.RawMessage `json:"defaultParams"`
}

func taskOptionDTO(value TaskOption) taskOptionResponse {
	return taskOptionResponse{Type: value.Type, DisplayName: value.DisplayName, AdminCreatable: value.AdminCreatable, BuiltinKey: value.BuiltinKey, DefaultParams: value.DefaultParams}
}

func scheduleDTO(row Schedule) scheduleResponse {
	builtinKey := ""
	if row.BuiltinKey != nil {
		builtinKey = *row.BuiltinKey
	}
	return scheduleResponse{row.ID, row.Name, row.Description, row.TaskType, row.CronExpression, row.Timezone, row.Params, row.IsEnabled == 1, row.NextRunAt, builtinKey, row.CreatedAt, row.UpdatedAt}
}

type jobResponse struct {
	ID            int64           `json:"id"`
	ScheduleID    *int64          `json:"scheduleId"`
	TaskType      string          `json:"taskType"`
	Payload       json.RawMessage `json:"payload"`
	TriggerSource TriggerSource   `json:"triggerSource"`
	ScheduledAt   time.Time       `json:"scheduledAt"`
	AvailableAt   time.Time       `json:"availableAt"`
	Status        JobStatus       `json:"status"`
	AttemptCount  int             `json:"attemptCount"`
	MaxAttempts   int             `json:"maxAttempts"`
	ErrorClass    string          `json:"errorClass"`
	LastError     string          `json:"lastError"`
	CompletedAt   *time.Time      `json:"completedAt"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

func jobDTO(row Job) jobResponse {
	return jobResponse{row.ID, row.ScheduleID, row.TaskType, row.Payload, row.TriggerSource, row.ScheduledAt, row.AvailableAt, row.Status, row.AttemptCount, row.MaxAttempts, row.ErrorClass, row.LastError, row.CompletedAt, row.CreatedAt, row.UpdatedAt}
}

type runResponse struct {
	ID            int64      `json:"id"`
	JobID         int64      `json:"jobId"`
	AttemptNo     int        `json:"attemptNo"`
	Status        RunStatus  `json:"status"`
	WorkerID      string     `json:"workerId"`
	StartedAt     time.Time  `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
	DurationMS    *int64     `json:"durationMs"`
	ErrorClass    string     `json:"errorClass"`
	ErrorMessage  string     `json:"errorMessage"`
	ResultSummary string     `json:"resultSummary"`
}

func runDTO(row Run) runResponse {
	return runResponse{row.ID, row.JobID, row.AttemptNo, row.Status, row.WorkerID, row.StartedAt, row.FinishedAt, row.DurationMS, row.ErrorClass, row.ErrorMessage, row.ResultSummary}
}
