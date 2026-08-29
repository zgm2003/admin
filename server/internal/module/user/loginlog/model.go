package loginlog

import (
	"time"

	"admin/server/internal/shared/yesno"
)

const (
	EventLogin    = "login"
	EventLogout   = "logout"
	LoginPassword = "password"
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

type Event struct {
	UserID       *int64
	SessionID    *int64
	PlatformID   int64
	LoginAccount string
	EventType    string
	LoginType    *string
	IsSuccess    yesno.Value
	ReasonCode   string
	ClientIP     string
	UserAgent    string
}

func ValidateEvent(event Event) error {
	if event.PlatformID < 1 || len(event.LoginAccount) > 254 || (event.EventType == EventLogin && event.LoginAccount == "") {
		return errInvalidEvent
	}
	if event.EventType != EventLogin && event.EventType != EventLogout {
		return errInvalidEvent
	}
	if event.EventType == EventLogin {
		if event.LoginType == nil || *event.LoginType == "" || len(*event.LoginType) > 32 {
			return errInvalidEvent
		}
	} else if event.LoginType != nil {
		return errInvalidEvent
	}
	if !yesno.IsValid(event.IsSuccess) || event.ReasonCode == "" || len(event.ReasonCode) > 64 || len(event.ClientIP) > 64 || len(event.UserAgent) > 512 {
		return errInvalidEvent
	}
	return nil
}
