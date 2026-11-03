package chargeitem

import (
	"encoding/json"
	"time"
)

type Model struct {
	ID                    int64           `gorm:"column:id;primaryKey;autoIncrement"`
	ChargeID              int64           `gorm:"column:charge_id;not null"`
	ProviderRequestID     int64           `gorm:"column:provider_request_id;not null"`
	PriceRateID           int64           `gorm:"column:price_rate_id;not null"`
	ItemKey               string          `gorm:"column:item_key;not null"`
	Operation             string          `gorm:"column:operation;not null"`
	UsageKey              string          `gorm:"column:usage_key;not null"`
	SelectorSnapshot      json.RawMessage `gorm:"column:selector_snapshot;type:jsonb;not null"`
	Quantity              string          `gorm:"column:quantity;type:numeric(24,8);not null"`
	UnitQuantity          string          `gorm:"column:unit_quantity;type:numeric(24,8);not null"`
	Unit                  string          `gorm:"column:unit;not null"`
	BillingUnitPriceUnits int64           `gorm:"column:billing_unit_price_units;not null"`
	MultiplierPPM         int64           `gorm:"column:multiplier_ppm;not null"`
	AmountUnits           int64           `gorm:"column:amount_units;not null"`
	RawUsageDigest        string          `gorm:"column:raw_usage_digest;not null"`
	CreatedAt             time.Time       `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt             time.Time       `gorm:"column:updated_at;type:timestamptz;not null"`
}

func (Model) TableName() string { return "ai_usage_charge_item" }
