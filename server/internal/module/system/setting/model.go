package setting

import (
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type Model struct {
	ID          int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Key         string         `gorm:"column:setting_key;type:varchar(128);not null"`
	Value       string         `gorm:"column:value;type:text;not null"`
	ValueType   int            `gorm:"column:value_type;not null"`
	Description string         `gorm:"column:description;type:varchar(512);not null"`
	IsEnabled   yesno.Value    `gorm:"column:is_enabled;type:smallint;not null"`
	IsBuiltin   yesno.Value    `gorm:"column:is_builtin;type:smallint;not null"`
	CreatedAt   time.Time      `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt   time.Time      `gorm:"column:updated_at;type:timestamptz;not null"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

func (Model) TableName() string { return "system_setting" }

type Record struct {
	ID          int64       `json:"id"`
	Key         string      `json:"key"`
	Value       string      `json:"value"`
	ValueType   int         `json:"valueType"`
	Description string      `json:"description"`
	IsEnabled   yesno.Value `json:"isEnabled"`
	IsBuiltin   yesno.Value `json:"isBuiltin"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
}

func recordFromModel(row Model) Record {
	return Record{ID: row.ID, Key: row.Key, Value: row.Value, ValueType: row.ValueType, Description: row.Description, IsEnabled: row.IsEnabled, IsBuiltin: row.IsBuiltin, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}
