package rechargepackage

import (
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type Model struct {
	ID          int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Code        string         `gorm:"column:code;type:varchar(64);not null"`
	Name        string         `gorm:"column:name;type:varchar(128);not null"`
	AmountCents int64          `gorm:"column:amount_cents;not null"`
	Badge       *string        `gorm:"column:badge;type:varchar(32)"`
	SortOrder   int32          `gorm:"column:sort_order;not null"`
	IsEnabled   yesno.Value    `gorm:"column:is_enabled;type:smallint;not null"`
	CreatedAt   time.Time      `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt   time.Time      `gorm:"column:updated_at;type:timestamptz;not null"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

func (Model) TableName() string { return "payment_recharge_package" }
