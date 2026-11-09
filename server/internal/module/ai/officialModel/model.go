package officialmodel

import "time"

type Model struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Vendor          Vendor    `gorm:"column:vendor;type:smallint;not null"`
	ModelKey        string    `gorm:"column:model_key;type:varchar(200);not null"`
	DisplayName     string    `gorm:"column:display_name;type:varchar(200);not null"`
	InputUSDMicros  int64     `gorm:"column:input_usd_micros;not null"`
	OutputUSDMicros int64     `gorm:"column:output_usd_micros;not null"`
	InputCNYMicros  int64     `gorm:"column:input_cny_micros;not null"`
	OutputCNYMicros int64     `gorm:"column:output_cny_micros;not null"`
	ModelKind       int16     `gorm:"column:model_kind;type:smallint;not null"`
	CacheReadCNY    *int64    `gorm:"column:cache_read_cny_micros"`
	CacheWriteCNY   *int64    `gorm:"column:cache_write_cny_micros"`
	CacheReadUSD    *int64    `gorm:"column:cache_read_usd_micros"`
	CacheWriteUSD   *int64    `gorm:"column:cache_write_usd_micros"`
	CatalogVersion  string    `gorm:"column:catalog_version;type:varchar(64);not null"`
	SourceURL       string    `gorm:"column:source_url;type:varchar(512);not null"`
	VerifiedAt      time.Time `gorm:"column:verified_at;type:timestamptz;not null"`
	SortOrder       int       `gorm:"column:sort_order;not null"`
	Version         int64     `gorm:"column:version;not null"`
	CreatedAt       time.Time `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt       time.Time `gorm:"column:updated_at;type:timestamptz;not null"`
}

func (Model) TableName() string { return "ai_official_model" }
