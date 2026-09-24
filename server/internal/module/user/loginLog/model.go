package loginlog

import (
	"time"

	"admin/server/internal/shared/yesno"
)

type LoginLog struct {
	ID         int64       `gorm:"column:id;primaryKey;autoIncrement"`
	UserID     *int64      `gorm:"column:user_id"`
	PlatformID int64       `gorm:"column:platform_id;not null"`
	Account    string      `gorm:"column:account;type:varchar(254);not null"`
	EventType  EventType   `gorm:"column:event_type;type:smallint;not null"`
	LoginType  *LoginType  `gorm:"column:login_type;type:smallint"`
	IsSuccess  yesno.Value `gorm:"column:is_success;type:smallint;not null"`
	ReasonCode string      `gorm:"column:reason_code;type:varchar(64);not null"`
	ClientIP   string      `gorm:"column:client_ip;type:varchar(64);not null"`
	UserAgent  string      `gorm:"column:user_agent;type:varchar(512);not null"`
	CreatedAt  time.Time   `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt  time.Time   `gorm:"column:updated_at;type:timestamptz;not null"`
}

func (LoginLog) TableName() string { return "user_login_log" }
