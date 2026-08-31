package role

import (
	"time"

	"admin/server/internal/module/permission/userrole"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type Role struct {
	ID        int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Code      string         `gorm:"column:code;type:varchar(64);not null"`
	Name      string         `gorm:"column:name;type:varchar(64);not null"`
	IsDefault yesno.Value    `gorm:"column:is_default;type:smallint;not null;default:0;check:ck_permission_role_is_default,is_default IN (0,1)"`
	IsEnabled yesno.Value    `gorm:"column:is_enabled;type:smallint;not null;default:1;check:ck_permission_role_is_enabled,is_enabled IN (0,1)"`
	CreatedAt time.Time      `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time      `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

func (Role) TableName() string {
	return "permission_role"
}

type UserRole = userrole.UserRole
