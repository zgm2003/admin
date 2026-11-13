package chat

import (
	"time"

	"gorm.io/gorm"
)

type MessageRole int16

const (
	RoleUser      MessageRole = 1
	RoleAssistant MessageRole = 2
)

type MessageStatus int16

const (
	MessageStreaming MessageStatus = 1
	MessageCompleted MessageStatus = 2
	MessageStopped   MessageStatus = 3
	MessageFailed    MessageStatus = 4
)

type Conversation struct {
	ID            int64          `gorm:"column:id;primaryKey;autoIncrement"`
	PlatformID    int64          `gorm:"column:platform_id;not null"`
	UserID        int64          `gorm:"column:user_id;not null"`
	AgentID       int64          `gorm:"column:agent_id;not null"`
	Title         string         `gorm:"column:title;type:varchar(128);not null"`
	LastMessageAt time.Time      `gorm:"column:last_message_at;type:timestamptz;not null"`
	MessageCount  int32          `gorm:"column:message_count;not null"`
	Version       int64          `gorm:"column:version;not null"`
	CreatedAt     time.Time      `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt     time.Time      `gorm:"column:updated_at;type:timestamptz;not null"`
	DeletedAt     gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz;index"`
}

func (Conversation) TableName() string { return "ai_chat_conversation" }

type Message struct {
	ID             int64         `gorm:"column:id;primaryKey;autoIncrement"`
	ConversationID int64         `gorm:"column:conversation_id;not null"`
	Seq            int32         `gorm:"column:seq;not null"`
	Role           MessageRole   `gorm:"column:role;type:smallint;not null"`
	Content        string        `gorm:"column:content;type:text;not null"`
	Status         MessageStatus `gorm:"column:status;type:smallint;not null"`
	RunID          *int64        `gorm:"column:run_id"`
	DeliverySeq    int64         `gorm:"column:delivery_seq;not null"`
	CreatedAt      time.Time     `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt      time.Time     `gorm:"column:updated_at;type:timestamptz;not null"`

	// Read-only settlement projection joined from ai_usage_charge.
	UsageAmountUnits       *int64 `gorm:"->;column:usage_amount_units"`
	UsageInputTokens       *int64 `gorm:"->;column:usage_input_tokens"`
	UsageCachedInputTokens *int64 `gorm:"->;column:usage_cached_input_tokens"`
	UsageOutputTokens      *int64 `gorm:"->;column:usage_output_tokens"`
}

func (Message) TableName() string { return "ai_chat_message" }

func (m Message) RunIDValue() int64 {
	if m.RunID == nil {
		return 0
	}
	return *m.RunID
}

// AgentRow is the lightweight agent projection used by conversation selection.
type AgentRow struct {
	ID           int64  `gorm:"column:id"`
	Name         string `gorm:"column:name"`
	Description  string `gorm:"column:description"`
	Scene        int16  `gorm:"column:scene"`
	ProviderName string `gorm:"column:provider_name"`
	ModelName    string `gorm:"column:model_name"`
	TimeoutMS    int32  `gorm:"column:timeout_ms"`
}

// ActiveRunSummary is a read-only projection of the run currently owning a
// conversation. Chat does not import the run module; this query is a narrow
// cross-table read used only to build the message snapshot.
type ActiveRunSummary struct {
	ID                 int64  `gorm:"column:id"`
	RequestID          string `gorm:"column:request_id"`
	Status             int16  `gorm:"column:status"`
	BillingStatus      int16  `gorm:"column:billing_status"`
	AssistantMessageID int64  `gorm:"column:assistant_message_id"`
	DeliverySeq        int64  `gorm:"column:delivery_seq"`
}
