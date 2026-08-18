package menu

import (
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type Type string

const (
	TypeDirectory Type = "directory"
	TypePage      Type = "page"
	TypeAction    Type = "action"
)

type Menu struct {
	ID        int64          `gorm:"column:id;primaryKey;autoIncrement"`
	ParentID  *int64         `gorm:"column:parent_id"`
	MenuType  Type           `gorm:"column:menu_type;type:varchar(16);not null"`
	Code      string         `gorm:"column:code;type:varchar(128);not null"`
	I18nKey   string         `gorm:"column:i18n_key;type:varchar(128);not null"`
	Path      *string        `gorm:"column:path;type:varchar(255)"`
	ViewKey   *string        `gorm:"column:view_key;type:varchar(128)"`
	Icon      *string        `gorm:"column:icon;type:varchar(64)"`
	SortOrder int            `gorm:"column:sort_order;type:integer;not null;default:0"`
	IsEnabled yesno.Value    `gorm:"column:is_enabled;type:smallint;not null;default:1"`
	CreatedAt time.Time      `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time      `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

func (Menu) TableName() string {
	return "sys_menu"
}

type RoleMenu struct {
	ID        int64          `gorm:"column:id;primaryKey;autoIncrement"`
	RoleID    int64          `gorm:"column:role_id;not null"`
	MenuID    int64          `gorm:"column:menu_id;not null"`
	CreatedAt time.Time      `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time      `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

func (RoleMenu) TableName() string {
	return "sys_role_menu"
}
