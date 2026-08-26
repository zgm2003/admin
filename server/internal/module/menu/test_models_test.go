package menu

import (
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type testRole struct {
	ID        int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Code      string         `gorm:"column:code;type:varchar(64);not null"`
	Name      string         `gorm:"column:name;type:varchar(64);not null"`
	IsDefault yesno.Value    `gorm:"column:is_default;type:smallint;not null;default:0"`
	IsEnabled yesno.Value    `gorm:"column:is_enabled;type:smallint;not null;default:1"`
	CreatedAt time.Time      `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time      `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

func (testRole) TableName() string { return "rbac_role" }

type testUser struct {
	ID        int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Username  string         `gorm:"column:username;type:varchar(64);not null"`
	Email     string         `gorm:"column:email;type:varchar(254);not null"`
	IsEnabled yesno.Value    `gorm:"column:is_enabled;type:smallint;not null;default:1"`
	CreatedAt time.Time      `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time      `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

func (testUser) TableName() string { return "user_account" }

type testAccessVersion struct {
	UserID    int64     `gorm:"column:user_id;primaryKey"`
	Version   int64     `gorm:"column:version;not null;default:1"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
}

func (testAccessVersion) TableName() string { return "rbac_access_version" }
