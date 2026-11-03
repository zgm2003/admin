package charge

import "time"

type Model struct {
	ID                 int64      `gorm:"column:id;primaryKey;autoIncrement"`
	RunID              int64      `gorm:"column:run_id;not null"`
	UserID             int64      `gorm:"column:user_id;not null"`
	PlatformID         int64      `gorm:"column:platform_id;not null"`
	Status             int16      `gorm:"column:status;not null"`
	Currency           string     `gorm:"column:currency;not null"`
	PriceVersionID     int64      `gorm:"column:price_version_id;not null"`
	MultiplierPPM      int64      `gorm:"column:multiplier_ppm;not null"`
	ActualUnits        *int64     `gorm:"column:actual_units"`
	UsageComplete      int16      `gorm:"column:usage_complete;not null"`
	SnapshotDigest     string     `gorm:"column:snapshot_digest;not null"`
	SettlementAttempts int        `gorm:"column:settlement_attempts;not null"`
	NextSettleAt       *time.Time `gorm:"column:next_settle_at"`
	SettledAt          *time.Time `gorm:"column:settled_at"`
	ErrorCode          *string    `gorm:"column:error_code"`
	ErrorMessage       *string    `gorm:"column:error_message"`
	CreatedAt          time.Time  `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;type:timestamptz;not null"`
}

func (Model) TableName() string { return "ai_usage_charge" }
