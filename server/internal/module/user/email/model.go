package email

import "time"

const changeLogTable = "user_email_change_log"

// ChangeLog is the append-only audit record for a verified email mutation.
type ChangeLog struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID       int64     `gorm:"column:user_id;not null"`
	PlatformID   int64     `gorm:"column:platform_id;not null"`
	Action       string    `gorm:"column:action;type:varchar(16);not null"`
	OldEmailHint string    `gorm:"column:old_email_hint;type:varchar(128);not null"`
	OldEmailHMAC string    `gorm:"column:old_email_hmac;type:varchar(128);not null"`
	NewEmailHint string    `gorm:"column:new_email_hint;type:varchar(128);not null"`
	NewEmailHMAC string    `gorm:"column:new_email_hmac;type:varchar(128);not null"`
	CreatedAt    time.Time `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;type:timestamptz;not null"`
}

func (ChangeLog) TableName() string { return changeLogTable }
