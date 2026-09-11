package phone

import "time"

const changeLogTable = "user_phone_change_log"

// ChangeLog is the append-only audit record for a verified phone mutation.
type ChangeLog struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID       int64     `gorm:"column:user_id;not null"`
	PlatformID   int64     `gorm:"column:platform_id;not null"`
	Action       string    `gorm:"column:action;type:varchar(16);not null"`
	OldPhoneHint string    `gorm:"column:old_phone_hint;type:varchar(32);not null"`
	OldPhoneHMAC string    `gorm:"column:old_phone_hmac;type:varchar(128);not null"`
	NewPhoneHint string    `gorm:"column:new_phone_hint;type:varchar(32);not null"`
	NewPhoneHMAC string    `gorm:"column:new_phone_hmac;type:varchar(128);not null"`
	CreatedAt    time.Time `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;type:timestamptz;not null"`
}

func (ChangeLog) TableName() string { return changeLogTable }
