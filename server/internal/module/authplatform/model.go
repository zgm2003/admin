package authplatform

import (
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type Platform struct {
	ID                     int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Code                   string         `gorm:"column:code;type:varchar(49);not null"`
	Name                   string         `gorm:"column:name;type:varchar(64);not null"`
	PolicyVersion          int64          `gorm:"column:policy_version;not null;default:1"`
	AccessTTLSeconds       int            `gorm:"column:access_ttl_seconds;type:integer;not null"`
	RefreshTTLSeconds      int            `gorm:"column:refresh_ttl_seconds;type:integer;not null"`
	SessionCacheTTLSeconds int            `gorm:"column:session_cache_ttl_seconds;type:integer;not null"`
	AccessCacheTTLSeconds  int            `gorm:"column:access_cache_ttl_seconds;type:integer;not null"`
	BindDevice             yesno.Value    `gorm:"column:bind_device;type:smallint;not null"`
	BindIP                 yesno.Value    `gorm:"column:bind_ip;type:smallint;not null"`
	MaxSessions            int16          `gorm:"column:max_sessions;type:smallint;not null"`
	AllowRegister          yesno.Value    `gorm:"column:allow_register;type:smallint;not null"`
	IsEnabled              yesno.Value    `gorm:"column:is_enabled;type:smallint;not null"`
	IsBuiltin              yesno.Value    `gorm:"column:is_builtin;type:smallint;not null"`
	CreatedAt              time.Time      `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt              time.Time      `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt              gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

func (Platform) TableName() string {
	return "sys_auth_platform"
}
