package userrole

import (
	"gorm.io/gorm"
	"time"
)

type UserRole struct {
	ID        int64          `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    int64          `gorm:"column:user_id;not null"`
	RoleID    int64          `gorm:"column:role_id;not null"`
	CreatedAt time.Time      `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time      `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

func (UserRole) TableName() string { return "permission_user_role" }
