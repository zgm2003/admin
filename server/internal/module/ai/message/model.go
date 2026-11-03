package message

import (
	"encoding/json"
	"time"
)

type Model struct {
	ID             int64           `json:"id,string" gorm:"primaryKey"`
	ConversationID int64           `json:"conversationId,string"`
	RunID          *int64          `json:"runId,omitempty,string"`
	Sequence       int64           `json:"sequence"`
	Role           int16           `json:"role"`
	Content        json.RawMessage `json:"content" gorm:"type:jsonb"`
	Status         int16           `json:"status"`
	ProviderItemID *string         `json:"-"`
	ToolCallID     *string         `json:"-"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

func (Model) TableName() string { return "ai_message" }
