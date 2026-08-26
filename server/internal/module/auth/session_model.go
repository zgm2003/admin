package auth

import "time"

type Session struct {
	ID               int64      `gorm:"column:id;primaryKey;autoIncrement"`
	UserID           int64      `gorm:"column:user_id;not null"`
	Platform         string     `gorm:"column:platform;type:varchar(49);not null"`
	DeviceID         string     `gorm:"column:device_id;type:varchar(36);not null"`
	RefreshTokenHash string     `gorm:"column:refresh_token_hash;type:char(64);not null"`
	Version          int64      `gorm:"column:version;not null;default:1;check:ck_auth_session_version,version >= 1"`
	ClientIP         string     `gorm:"column:client_ip;type:varchar(64);not null"`
	UserAgent        string     `gorm:"column:user_agent;type:varchar(512);not null"`
	RefreshExpiresAt time.Time  `gorm:"column:refresh_expires_at;type:timestamptz;not null"`
	RevokedAt        *time.Time `gorm:"column:revoked_at;type:timestamptz"`
	CreatedAt        time.Time  `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
}

func (Session) TableName() string {
	return "auth_session"
}
