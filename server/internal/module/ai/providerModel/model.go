package providermodel

import (
	"time"

	"admin/server/internal/module/ai/pricing"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type Source int16

const (
	SourceManual Source = 1
	SourceSync   Source = 2
)

func (source Source) Valid() bool { return source == SourceManual || source == SourceSync }

type Model struct {
	ID              int64               `gorm:"column:id;primaryKey;autoIncrement"`
	ProviderID      int64               `gorm:"column:provider_id;not null"`
	UpstreamModel   string              `gorm:"column:upstream_model;type:varchar(200);not null"`
	ModelKey        string              `gorm:"column:model_key;type:varchar(200);not null"`
	DisplayName     string              `gorm:"column:display_name;type:varchar(200);not null"`
	ModelKind       pricing.ModelKind   `gorm:"column:model_kind;type:smallint;not null"`
	PricingMode     pricing.PricingMode `gorm:"column:pricing_mode;type:smallint;not null"`
	PriceUnit       pricing.PriceUnit   `gorm:"column:price_unit;type:smallint;not null"`
	OfficialModelID *int64              `gorm:"column:official_model_id"`
	Source          Source              `gorm:"column:source;type:smallint;not null"`
	Status          yesno.Value         `gorm:"column:status;type:smallint;not null"`
	LastSeenAt      *time.Time          `gorm:"column:last_seen_at;type:timestamptz"`
	Version         int64               `gorm:"column:version;not null"`
	CreatedAt       time.Time           `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt       time.Time           `gorm:"column:updated_at;type:timestamptz;not null"`
	DeletedAt       gorm.DeletedAt      `gorm:"column:deleted_at;type:timestamptz;index"`
}

func (Model) TableName() string { return "ai_provider_model" }

type Price struct {
	ID              int64          `gorm:"column:id;primaryKey;autoIncrement"`
	ProviderModelID int64          `gorm:"column:provider_model_id;not null"`
	TierKey         string         `gorm:"column:tier_key;type:varchar(64);not null"`
	TierLabel       string         `gorm:"column:tier_label;type:varchar(64);not null"`
	PriceCNYMicros  int64          `gorm:"column:price_cny_micros;not null"`
	Status          yesno.Value    `gorm:"column:status;type:smallint;not null"`
	Version         int64          `gorm:"column:version;not null"`
	CreatedAt       time.Time      `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt       time.Time      `gorm:"column:updated_at;type:timestamptz;not null"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}

func (Price) TableName() string { return "ai_provider_model_price" }

type Official struct {
	ID          int64             `gorm:"column:id"`
	Vendor      int16             `gorm:"column:vendor"`
	ModelKey    string            `gorm:"column:model_key"`
	DisplayName string            `gorm:"column:display_name"`
	ModelKind   pricing.ModelKind `gorm:"column:model_kind"`
	InputCNY    int64             `gorm:"column:input_cny_micros"`
	OutputCNY   int64             `gorm:"column:output_cny_micros"`
	CacheRead   *int64            `gorm:"column:cache_read_cny_micros"`
	CacheWrite  *int64            `gorm:"column:cache_write_cny_micros"`
}

func (Official) TableName() string { return "ai_official_model" }
