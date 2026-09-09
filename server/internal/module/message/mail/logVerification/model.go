package logverification

import (
	"time"

	"gorm.io/gorm"
)

const Table = "message_mail_log_verification"

type Model struct {
	ID             int64          `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	PlatformID     int64          `gorm:"column:platform_id;not null" json:"platformId"`
	MailLogID      int64          `gorm:"column:mail_log_id;not null" json:"mailLogId"`
	KeyVersion     string         `gorm:"column:key_version;type:varchar(16);not null" json:"-"`
	CodeCiphertext string         `gorm:"column:code_ciphertext;not null" json:"-"`
	ExpiresAt      time.Time      `gorm:"column:expires_at;not null" json:"expiresAt"`
	CreatedAt      time.Time      `gorm:"column:created_at;not null" json:"createdAt"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz" json:"-"`
}

func (Model) TableName() string { return "message_mail_log_verification" }
