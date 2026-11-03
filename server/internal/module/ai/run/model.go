package run

import (
	"encoding/json"
	"time"
)

type Model struct {
	ID                 int64           `gorm:"column:id;primaryKey"`
	UserID             int64           `gorm:"column:user_id"`
	PlatformID         int64           `gorm:"column:platform_id"`
	RequestKey         string          `gorm:"column:request_key"`
	RequestFingerprint string          `gorm:"column:request_fingerprint"`
	ConversationID     *int64          `gorm:"column:conversation_id"`
	AgentID            int64           `gorm:"column:agent_id"`
	BindingID          int64           `gorm:"column:binding_id"`
	ProviderModelID    int64           `gorm:"column:provider_model_id"`
	ProviderVersionID  int64           `gorm:"column:provider_version_id"`
	OfficialModelID    int64           `gorm:"column:official_model_id"`
	PriceVersionID     int64           `gorm:"column:price_version_id"`
	Operation          string          `gorm:"column:operation"`
	ExecutionStatus    int16           `gorm:"column:execution_status"`
	UsageStatus        int16           `gorm:"column:usage_status"`
	BillingStatus      int16           `gorm:"column:billing_status"`
	TransferStatus     int16           `gorm:"column:transfer_status"`
	InputSnapshot      json.RawMessage `gorm:"column:input_snapshot;type:jsonb"`
	ConfigSnapshot     json.RawMessage `gorm:"column:config_snapshot;type:jsonb"`
	PolicySnapshot     json.RawMessage `gorm:"column:policy_snapshot;type:jsonb"`
	MultiplierPPM      int64           `gorm:"column:multiplier_ppm"`
	EstimatedUnits     int64           `gorm:"column:estimated_units"`
	ReservationStatus  int16           `gorm:"column:reservation_status"`
	StateVersion       int64           `gorm:"column:state_version"`
	NextEventSequence  int64           `gorm:"column:next_event_sequence"`
	AcceptedAt         time.Time       `gorm:"column:accepted_at"`
	DeadlineAt         time.Time       `gorm:"column:deadline_at"`
	StartedAt          *time.Time      `gorm:"column:started_at"`
	FinishedAt         *time.Time      `gorm:"column:finished_at"`
	CancelRequestedAt  *time.Time      `gorm:"column:cancel_requested_at"`
	ErrorCode          *string         `gorm:"column:error_code"`
	ErrorMessage       *string         `gorm:"column:error_message"`
	CreatedAt          time.Time       `gorm:"column:created_at"`
	UpdatedAt          time.Time       `gorm:"column:updated_at"`
}

func (Model) TableName() string { return "ai_run" }

type CreateInput struct {
	ConversationID *int64
	UserID         int64
	PlatformID     int64
	AgentID        int64
	RequestKey     string
	Operation      string
	Input          json.RawMessage
	EstimatedUnits int64
	VariableUsage  bool
	DeadlineAt     time.Time
}

type InputFileSnapshot struct {
	ID        string `json:"id"`
	ObjectKey string `json:"objectKey"`
	MIMEType  string `json:"mimeType"`
	SizeBytes int64  `json:"sizeBytes"`
	ETag      string `json:"etag"`
}
