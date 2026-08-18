package user

import (
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type User struct {
	ID           int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Username     string         `gorm:"column:username;type:varchar(64);not null"`
	Email        string         `gorm:"column:email;type:varchar(254);not null"`
	PasswordHash string         `gorm:"column:password_hash;type:varchar(255);not null"`
	IsEnabled    yesno.Value    `gorm:"column:is_enabled;type:smallint;not null;default:1;check:ck_sys_user_is_enabled,is_enabled IN (0,1)"`
	CreatedAt    time.Time      `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt    time.Time      `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

func (User) TableName() string {
	return "sys_user"
}
