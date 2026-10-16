package order

import "time"

type Model struct {
	ID                 int64      `gorm:"column:id;primaryKey;autoIncrement"`
	OrderNo            string     `gorm:"column:order_no;type:varchar(32);not null"`
	UserID             int64      `gorm:"column:user_id;not null"`
	PlatformID         int64      `gorm:"column:platform_id;not null"`
	PackageID          int64      `gorm:"column:package_id;not null"`
	PackageName        string     `gorm:"column:package_name;type:varchar(128);not null"`
	AmountCents        int64      `gorm:"column:amount_cents;not null"`
	ConfigID           int64      `gorm:"column:config_id;not null"`
	ConfigVersion      int64      `gorm:"column:config_version;not null"`
	PayMethod          int16      `gorm:"column:pay_method;not null"`
	RequestID          string     `gorm:"column:request_id;type:uuid;not null"`
	RequestFingerprint []byte     `gorm:"column:request_fingerprint;type:bytea;not null"`
	ReturnURL          string     `gorm:"column:return_url;type:varchar(2048);not null"`
	Status             Status     `gorm:"column:status;not null"`
	ProviderTradeNo    *string    `gorm:"column:provider_trade_no;type:varchar(64)"`
	ExpiresAt          time.Time  `gorm:"column:expires_at;type:timestamptz;not null"`
	PaidAt             *time.Time `gorm:"column:paid_at;type:timestamptz"`
	CreditedAt         *time.Time `gorm:"column:credited_at;type:timestamptz"`
	ClosedAt           *time.Time `gorm:"column:closed_at;type:timestamptz"`
	NextCheckAt        *time.Time `gorm:"column:next_check_at;type:timestamptz"`
	CheckToken         *string    `gorm:"column:check_token;type:uuid"`
	CheckLeaseUntil    *time.Time `gorm:"column:check_lease_until;type:timestamptz"`
	CreatedAt          time.Time  `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;type:timestamptz;not null"`
}

func (Model) TableName() string { return "payment_order" }
