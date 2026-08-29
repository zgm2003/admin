package session

import (
	"time"

	"admin/server/internal/shared/yesno"
)

type Session struct {
	ID               int64      `gorm:"column:id;primaryKey;autoIncrement"`
	UserID           int64      `gorm:"column:user_id;not null"`
	PlatformID       int64      `gorm:"column:platform_id;not null"`
	Platform         string     `gorm:"-"`
	DeviceID         string     `gorm:"column:device_id;type:varchar(36);not null"`
	RefreshTokenHash string     `gorm:"column:refresh_token_hash;type:char(64);not null"`
	Version          int64      `gorm:"column:version;not null;default:1;check:ck_user_session_version,version >= 1"`
	ClientIP         string     `gorm:"column:client_ip;type:varchar(64);not null"`
	UserAgent        string     `gorm:"column:user_agent;type:varchar(512);not null"`
	RefreshExpiresAt time.Time  `gorm:"column:refresh_expires_at;type:timestamptz;not null"`
	RevokedAt        *time.Time `gorm:"column:revoked_at;type:timestamptz"`
	CreatedAt        time.Time  `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
}

// Record combines the persisted session with its platform code resolved from
// auth_platform. Platform is not a user_session column.
type Record = Session

type Authority struct {
	Session       Record
	UserID        int64
	UserIsEnabled yesno.Value
	UserDeleted   bool
}

type CreateInput struct {
	UserID           int64
	Platform         string
	DeviceID         string
	RefreshTokenHash string
	ClientIP         string
	UserAgent        string
	RefreshExpiresAt time.Time
}

type TokenIdentity struct {
	UserID    int64
	SessionID int64
	Platform  string
	Version   int64
}

func (Session) TableName() string {
	return "user_session"
}
