package loginlog

import (
	"time"

	"admin/server/internal/shared/yesno"
)

const (
	EventLogin    = "login"
	EventLogout   = "logout"
	LoginPassword = "password"
	LoginEmail    = "email"
	LoginPhone    = "phone"
)

type LoginLog struct {
	ID           int64       `gorm:"column:id;primaryKey;autoIncrement"`
	UserID       *int64      `gorm:"column:user_id"`
	SessionID    *int64      `gorm:"column:session_id"`
	PlatformID   int64       `gorm:"column:platform_id;not null"`
	LoginAccount string      `gorm:"column:login_account;type:varchar(254);not null"`
	EventType    string      `gorm:"column:event_type;type:varchar(16);not null"`
	LoginType    *string     `gorm:"column:login_type;type:varchar(32)"`
	IsSuccess    yesno.Value `gorm:"column:is_success;type:smallint;not null"`
	ReasonCode   string      `gorm:"column:reason_code;type:varchar(64);not null"`
	ClientIP     string      `gorm:"column:client_ip;type:varchar(64);not null"`
	UserAgent    string      `gorm:"column:user_agent;type:varchar(512);not null"`
	CreatedAt    time.Time   `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt    time.Time   `gorm:"column:updated_at;type:timestamptz;not null"`
}

func (LoginLog) TableName() string { return "user_login_log" }
