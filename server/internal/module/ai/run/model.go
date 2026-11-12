package run

import "time"

type RunStatus int16

const (
	RunQueued    RunStatus = 1
	RunRunning   RunStatus = 2
	RunSucceeded RunStatus = 3
	RunFailed    RunStatus = 4
	RunCanceled  RunStatus = 5
	RunTimeout   RunStatus = 6
)

func (status RunStatus) Active() bool {
	return status == RunQueued || status == RunRunning
}

type BillingStatus int16

const (
	BillingPending  BillingStatus = 1
	BillingSettled  BillingStatus = 2
	BillingUnbilled BillingStatus = 3
	BillingFailed   BillingStatus = 4
)

type Run struct {
	ID                 int64         `gorm:"column:id;primaryKey;autoIncrement"`
	RequestID          string        `gorm:"column:request_id;type:varchar(64);not null"`
	PlatformID         int64         `gorm:"column:platform_id;not null"`
	UserID             int64         `gorm:"column:user_id;not null"`
	ConversationID     int64         `gorm:"column:conversation_id;not null"`
	AgentID            int64         `gorm:"column:agent_id;not null"`
	ProviderID         int64         `gorm:"column:provider_id;not null"`
	ProviderModelID    int64         `gorm:"column:provider_model_id;not null"`
	ProviderProtocol   int16         `gorm:"column:provider_protocol;type:smallint;not null"`
	AgentName          string        `gorm:"column:agent_name;type:varchar(128);not null"`
	ProviderName       string        `gorm:"column:provider_name;type:varchar(128);not null"`
	ModelKey           string        `gorm:"column:model_key;type:varchar(128);not null"`
	ModelName          string        `gorm:"column:model_name;type:varchar(128);not null"`
	TimeoutMS          int32         `gorm:"column:timeout_ms;not null"`
	PricingSnapshot    string        `gorm:"column:pricing_snapshot;type:jsonb;not null"`
	Status             RunStatus     `gorm:"column:status;type:smallint;not null"`
	BillingStatus      BillingStatus `gorm:"column:billing_status;type:smallint;not null"`
	ErrorCode          string        `gorm:"column:error_code;type:varchar(64);not null"`
	ErrorMessage       string        `gorm:"column:error_message;type:varchar(1024);not null"`
	InputTokens        int64         `gorm:"column:input_tokens;not null"`
	CachedInputTokens  int64         `gorm:"column:cached_input_tokens;not null"`
	CacheWriteTokens   int64         `gorm:"column:cache_write_tokens;not null"`
	OutputTokens       int64         `gorm:"column:output_tokens;not null"`
	AmountUnits        int64         `gorm:"column:amount_units;not null"`
	BalanceBeforeUnits *int64        `gorm:"column:balance_before_units"`
	BalanceAfterUnits  *int64        `gorm:"column:balance_after_units"`
	FirstTokenMS       *int32        `gorm:"column:first_token_ms"`
	DurationMS         *int32        `gorm:"column:duration_ms"`
	CancelRequestedAt  *time.Time    `gorm:"column:cancel_requested_at;type:timestamptz"`
	StartedAt          *time.Time    `gorm:"column:started_at;type:timestamptz"`
	FinishedAt         *time.Time    `gorm:"column:finished_at;type:timestamptz"`
	CreatedAt          time.Time     `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt          time.Time     `gorm:"column:updated_at;type:timestamptz;not null"`
}

func (Run) TableName() string { return "ai_chat_run" }

type RunEvent struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	RunID     int64     `gorm:"column:run_id;not null"`
	Seq       int32     `gorm:"column:seq;not null"`
	Type      string    `gorm:"column:type;type:varchar(48);not null"`
	Message   string    `gorm:"column:message;type:varchar(500);not null"`
	Data      string    `gorm:"column:data;type:jsonb;not null"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null"`
}

func (RunEvent) TableName() string { return "ai_chat_run_event" }

type UsageCharge struct {
	ID                 int64     `gorm:"column:id;primaryKey;autoIncrement"`
	RunID              int64     `gorm:"column:run_id;not null"`
	PlatformID         int64     `gorm:"column:platform_id;not null"`
	UserID             int64     `gorm:"column:user_id;not null"`
	ProviderID         int64     `gorm:"column:provider_id;not null"`
	ProviderModelID    int64     `gorm:"column:provider_model_id;not null"`
	Status             int16     `gorm:"column:status;type:smallint;not null"`
	InputTokens        int64     `gorm:"column:input_tokens;not null"`
	CachedInputTokens  int64     `gorm:"column:cached_input_tokens;not null"`
	CacheWriteTokens   int64     `gorm:"column:cache_write_tokens;not null"`
	OutputTokens       int64     `gorm:"column:output_tokens;not null"`
	RatePPM            int64     `gorm:"column:rate_ppm;not null"`
	PricingSnapshot    string    `gorm:"column:pricing_snapshot;type:jsonb;not null"`
	AmountUnits        int64     `gorm:"column:amount_units;not null"`
	BalanceBeforeUnits *int64    `gorm:"column:balance_before_units"`
	BalanceAfterUnits  *int64    `gorm:"column:balance_after_units"`
	UnbilledReason     string    `gorm:"column:unbilled_reason;type:varchar(255);not null"`
	CreatedAt          time.Time `gorm:"column:created_at;type:timestamptz;not null"`
}

func (UsageCharge) TableName() string { return "ai_usage_charge" }

// AgentConfig is the execution snapshot source for a chat run.
type AgentConfig struct {
	ID                 int64     `gorm:"column:id"`
	Name               string    `gorm:"column:name"`
	Description        string    `gorm:"column:description"`
	SystemPrompt       string    `gorm:"column:system_prompt"`
	Scene              int16     `gorm:"column:scene"`
	AgentType          int16     `gorm:"column:agent_type"`
	TimeoutMS          int32     `gorm:"column:timeout_ms"`
	ProviderModelID    int64     `gorm:"column:provider_model_id"`
	ProviderID         int64     `gorm:"column:provider_id"`
	ProviderName       string    `gorm:"column:provider_name"`
	ProviderProtocol   int16     `gorm:"column:provider_protocol"`
	BaseURL            string    `gorm:"column:base_url"`
	APIKeyCiphertext   string    `gorm:"column:api_key_ciphertext"`
	EncryptionKeyID    string    `gorm:"column:encryption_key_id"`
	TokenRatePPM       int64     `gorm:"column:token_rate_multiplier_ppm"`
	ModelKey           string    `gorm:"column:model_key"`
	ModelName          string    `gorm:"column:display_name"`
	UpstreamModel      string    `gorm:"column:upstream_model"`
	PricingMode        int16     `gorm:"column:pricing_mode"`
	OfficialModelID    *int64    `gorm:"column:official_model_id"`
	InputCNYMicros     int64     `gorm:"column:input_cny_micros"`
	OutputCNYMicros    int64     `gorm:"column:output_cny_micros"`
	CacheReadCNYMicros *int64    `gorm:"column:cache_read_cny_micros"`
	CacheWriteCNY      *int64    `gorm:"column:cache_write_cny_micros"`
	CreatedAt          time.Time `gorm:"column:created_at"`
}

type PriceSnapshot struct {
	Currency           string `json:"currency"`
	InputCNYMicros     int64  `json:"inputCnyMicros"`
	OutputCNYMicros    int64  `json:"outputCnyMicros"`
	CacheReadCNYMicros *int64 `json:"cacheReadCnyMicros,omitempty"`
	CacheWriteCNY      *int64 `json:"cacheWriteCnyMicros,omitempty"`
	RatePPM            int64  `json:"ratePpm"`
}
