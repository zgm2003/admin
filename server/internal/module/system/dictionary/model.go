package dictionary

import (
	"time"

	"admin/server/internal/shared/yesno"

	"gorm.io/gorm"
)

type Dictionary struct {
	ID          int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Code        string         `gorm:"column:code;type:varchar(128);not null"`
	NameZH      string         `gorm:"column:name_zh;type:varchar(128);not null"`
	NameEN      string         `gorm:"column:name_en;type:varchar(128);not null"`
	Description string         `gorm:"column:description;type:varchar(512);not null"`
	IsEnabled   yesno.Value    `gorm:"column:is_enabled;type:smallint;not null"`
	IsBuiltin   yesno.Value    `gorm:"column:is_builtin;type:smallint;not null"`
	CreatedAt   time.Time      `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt   time.Time      `gorm:"column:updated_at;type:timestamptz;not null"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

func (Dictionary) TableName() string { return "system_dictionary" }

type Item struct {
	ID           int64          `gorm:"column:id;primaryKey;autoIncrement"`
	DictionaryID int64          `gorm:"column:dictionary_id;not null"`
	Value        string         `gorm:"column:value;type:varchar(128);not null"`
	LabelZH      string         `gorm:"column:label_zh;type:varchar(256);not null"`
	LabelEN      string         `gorm:"column:label_en;type:varchar(256);not null"`
	Sort         int            `gorm:"column:sort;not null"`
	IsEnabled    yesno.Value    `gorm:"column:is_enabled;type:smallint;not null"`
	IsBuiltin    yesno.Value    `gorm:"column:is_builtin;type:smallint;not null"`
	CreatedAt    time.Time      `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt    time.Time      `gorm:"column:updated_at;type:timestamptz;not null"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

func (Item) TableName() string { return "system_dictionary_item" }
