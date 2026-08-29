package rolemenu

import (
	"gorm.io/gorm"
	"time"
)

type RoleMenu struct {
	ID        int64          `gorm:"column:id;primaryKey;autoIncrement"`
	RoleID    int64          `gorm:"column:role_id;not null"`
	MenuID    int64          `gorm:"column:menu_id;not null"`
	CreatedAt time.Time      `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time      `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

func (RoleMenu) TableName() string { return "rbac_role_menu" }
