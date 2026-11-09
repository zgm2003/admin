package provider

import (
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type Model struct {
	ID               int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Name             string         `gorm:"column:name;type:varchar(128);not null"`
	Vendor           Vendor         `gorm:"column:vendor;type:smallint;not null"`
	Protocol         Protocol       `gorm:"column:protocol;type:smallint;not null"`
	BaseURL          string         `gorm:"column:base_url;type:varchar(2048);not null"`
	APIKeyCiphertext string         `gorm:"column:api_key_ciphertext;type:text;not null"`
	EncryptionKeyID  string         `gorm:"column:encryption_key_id;type:varchar(64);not null"`
	TimeoutMS        int32          `gorm:"column:timeout_ms;not null"`
	SortOrder        int32          `gorm:"column:sort_order;not null"`
	TokenRatePPM     int64          `gorm:"column:token_rate_multiplier_ppm;not null"`
	Status           yesno.Value    `gorm:"column:status;type:smallint;not null"`
	CreatedAt        time.Time      `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt        time.Time      `gorm:"column:updated_at;type:timestamptz;not null"`
	DeletedAt        gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz;index"`
}

func (Model) TableName() string { return "ai_provider" }
