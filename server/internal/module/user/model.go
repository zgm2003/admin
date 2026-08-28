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
	Phone        *string        `gorm:"column:phone;type:varchar(32)"`
	PasswordHash string         `gorm:"column:password_hash;type:varchar(255);not null"`
	IsEnabled    yesno.Value    `gorm:"column:is_enabled;type:smallint;not null;default:1;check:ck_user_account_is_enabled,is_enabled IN (0,1)"`
	CreatedAt    time.Time      `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt    time.Time      `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

type Profile struct {
	UserID    int64      `gorm:"column:user_id;primaryKey"`
	Birthday  *time.Time `gorm:"column:birthday;type:date"`
	Gender    int16      `gorm:"column:gender;type:smallint;not null;default:0"`
	CreatedAt time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt time.Time  `gorm:"column:updated_at;not null"`
}

func (Profile) TableName() string { return "user_profile" }

func (User) TableName() string {
	return "user_account"
}
