package toolcall

import (
	"encoding/json"
	"time"
)

type Model struct {
	ID                int64 `gorm:"primaryKey"`
	RunID             int64
	ProviderRequestID int64
	ToolVersionID     int64
	CallID            string
	ExposedName       string
	Arguments         json.RawMessage `gorm:"type:jsonb"`
	Result            json.RawMessage `gorm:"type:jsonb"`
	Status            int16
	IdempotencyKey    string
	ExecutionAttempts int
	StartedAt         *time.Time
	FinishedAt        *time.Time
	ErrorCode         *string
	ErrorMessage      *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (Model) TableName() string { return "ai_tool_call" }
