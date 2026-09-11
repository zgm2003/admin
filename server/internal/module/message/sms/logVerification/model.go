package logVerification

import (
	"time"
)

const Table = "message_sms_log_verification"

// Model stores the verification code ciphertext of one sending attempt. It is
// strictly append-only and only the log detail may decrypt it.
type Model struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement"`
	PlatformID     int64     `gorm:"column:platform_id;not null"`
	SMSLogID       int64     `gorm:"column:sms_log_id;not null"`
	KeyVersion     string    `gorm:"column:key_version;type:varchar(16);not null"`
	CodeCiphertext string    `gorm:"column:code_ciphertext;not null"`
	ExpiresAt      time.Time `gorm:"column:expires_at;not null"`
	CreatedAt      time.Time `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null"`
}

func (Model) TableName() string { return Table }
