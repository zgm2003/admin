package wallet

import "time"

type Model struct {
	UserID       int64     `gorm:"column:user_id;primaryKey"`
	BalanceUnits int64     `gorm:"column:balance_units;not null"`
	CreatedAt    time.Time `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;type:timestamptz;not null"`
	Username     string    `gorm:"->;column:username"`
}

func (Model) TableName() string { return "payment_wallet" }
