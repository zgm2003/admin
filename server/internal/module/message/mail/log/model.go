package log

import (
	"time"
)

const Table = "message_mail_log"

type Model struct {
	ID           int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	PlatformID   int64      `gorm:"column:platform_id;not null" json:"platformId"`
	ChallengeID  *string    `gorm:"column:challenge_id;type:varchar(128)" json:"-"`
	UserID       *int64     `gorm:"column:user_id" json:"userId"`
	Scene        string     `gorm:"column:scene;type:varchar(32);not null" json:"scene"`
	TemplateID   int        `gorm:"column:template_id;not null" json:"templateId"`
	ToEmail      string     `gorm:"column:to_email;type:varchar(254);not null" json:"toEmail"`
	Subject      string     `gorm:"column:subject;type:varchar(255);not null" json:"subject"`
	Status       string     `gorm:"column:status;type:varchar(16);not null" json:"status"`
	RequestID    string     `gorm:"column:request_id;type:varchar(128);not null" json:"requestId"`
	MessageID    string     `gorm:"column:message_id;type:varchar(128);not null" json:"messageId"`
	ErrorCode    string     `gorm:"column:error_code;type:varchar(128);not null" json:"errorCode"`
	ErrorSummary string     `gorm:"column:error_summary;type:varchar(512);not null" json:"errorSummary"`
	LatencyMs    int64      `gorm:"column:latency_ms;not null" json:"latencyMs"`
	SentAt       *time.Time `gorm:"column:sent_at;type:timestamptz" json:"sentAt"`
	CreatedAt    time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (Model) TableName() string { return "message_mail_log" }
