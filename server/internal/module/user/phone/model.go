package phone

import "time"

const changeLogTable = "user_phone_change_log"

// ChangeLog is the append-only audit record for a verified phone mutation.
type ChangeLog struct {
	ID         int64        `gorm:"column:id;primaryKey;autoIncrement"`
	UserID     int64        `gorm:"column:user_id;not null"`
	PlatformID int64        `gorm:"column:platform_id;not null"`
	Action     ChangeAction `gorm:"column:action;type:smallint;not null"`
	OldPhone   *string      `gorm:"column:old_phone;type:varchar(32)"`
	NewPhone   string       `gorm:"column:new_phone;type:varchar(32);not null"`
	CreatedAt  time.Time    `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt  time.Time    `gorm:"column:updated_at;type:timestamptz;not null"`
}

func (ChangeLog) TableName() string { return changeLogTable }
