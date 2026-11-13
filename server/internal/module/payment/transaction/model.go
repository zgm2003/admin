package transaction

import "time"

type Model struct {
	ID                int64     `gorm:"column:id;primaryKey;autoIncrement"`
	TransactionNo     string    `gorm:"column:transaction_no;type:varchar(32);not null"`
	UserID            int64     `gorm:"column:user_id;not null"`
	PlatformID        int64     `gorm:"column:platform_id;not null"`
	AmountUnits       int64     `gorm:"column:amount_units;not null"`
	BalanceAfterUnits int64     `gorm:"column:balance_after_units;not null"`
	OrderID           *int64    `gorm:"column:order_id"`
	RedeemCodeID      *int64    `gorm:"column:redeem_code_id"`
	CreatedAt         time.Time `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt         time.Time `gorm:"column:updated_at;type:timestamptz;not null"`
	Username          string    `gorm:"->;column:username"`
	PlatformCode      string    `gorm:"->;column:platform_code"`

	// Personal ledger projections. AI settlement stays in ai_usage_charge; the
	// personal wallet statement reads both sources without weakening the
	// positive-amount constraints on payment_transaction.
	SourceType         SourceType `gorm:"->;column:source_type"`
	BalanceBeforeUnits *int64     `gorm:"->;column:balance_before_units"`
	AIUsageChargeID    *int64     `gorm:"->;column:ai_usage_charge_id"`
	RunID              *int64     `gorm:"->;column:run_id"`
	AgentName          string     `gorm:"->;column:agent_name"`
	ModelName          string     `gorm:"->;column:model_name"`
	InputTokens        int64      `gorm:"->;column:input_tokens"`
	OutputTokens       int64      `gorm:"->;column:output_tokens"`
}

func (Model) TableName() string { return "payment_transaction" }
