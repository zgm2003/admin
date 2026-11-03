package conversation

import (
	"gorm.io/gorm"
	"time"
)

type Model struct {
	ID                  int64          `json:"id,string" gorm:"primaryKey"`
	UserID              int64          `json:"-"`
	PlatformID          int64          `json:"-"`
	AgentID             int64          `json:"agentId,string"`
	Title               string         `json:"title"`
	LastMessageSequence int64          `json:"lastMessageSequence"`
	CreatedAt           time.Time      `json:"createdAt"`
	UpdatedAt           time.Time      `json:"updatedAt"`
	DeletedAt           gorm.DeletedAt `json:"-"`
}

func (Model) TableName() string { return "ai_conversation" }
