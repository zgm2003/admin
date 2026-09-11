package log

import (
	"time"
)

const (
	Table = "message_sms_log"

	StatusPending = "pending"
	StatusSent    = "sent"
	StatusFailed  = "failed"
)

// Model is one append-only SMS sending audit row. The phone number exists only
// as ciphertext, a masked hint and an HMAC; there is no deleted_at and no
// delete path anywhere in this module.
type Model struct {
	ID                int64      `gorm:"column:id;primaryKey;autoIncrement"`
	PlatformID        int64      `gorm:"column:platform_id;not null"`
	ChallengeID       *string    `gorm:"column:challenge_id;type:varchar(128)"`
	UserID            *int64     `gorm:"column:user_id"`
	Scene             string     `gorm:"column:scene;type:varchar(32);not null"`
	TemplateID        int64      `gorm:"column:template_id;not null"`
	ToPhoneCiphertext string     `gorm:"column:to_phone_ciphertext;not null"`
	ToPhoneHint       string     `gorm:"column:to_phone_hint;type:varchar(32);not null"`
	ToPhoneHMAC       string     `gorm:"column:to_phone_hmac;type:varchar(128);not null"`
	Status            string     `gorm:"column:status;type:varchar(16);not null"`
	RequestID         string     `gorm:"column:request_id;type:varchar(128);not null"`
	SerialNo          string     `gorm:"column:serial_no;type:varchar(128);not null"`
	Fee               int        `gorm:"column:fee;not null"`
	ErrorCode         string     `gorm:"column:error_code;type:varchar(128);not null"`
	ErrorSummary      string     `gorm:"column:error_summary;type:varchar(512);not null"`
	LatencyMS         int64      `gorm:"column:latency_ms;not null"`
	SentAt            *time.Time `gorm:"column:sent_at"`
	CreatedAt         time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;not null"`
}

func (Model) TableName() string { return Table }
