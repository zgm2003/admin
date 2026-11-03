package providerrequest

import (
	"encoding/json"
	"time"
)

type Fact struct {
	ID                 int64 `gorm:"primaryKey"`
	RunID              int64
	Sequence           int
	RequestKey         string
	RequestFingerprint string
	Status             int16
	UpstreamRequestID  *string
	RequestSummary     json.RawMessage `gorm:"type:jsonb"`
	ResponseItems      json.RawMessage `gorm:"type:jsonb"`
	Usage              json.RawMessage `gorm:"type:jsonb"`
	UsageStatus        int16
	StartedAt          *time.Time
	FinishedAt         *time.Time
	ErrorCode          *string
	ErrorMessage       *string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (Fact) TableName() string { return "ai_provider_request" }
