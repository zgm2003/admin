package rateLimitPolicy

import (
	"time"
)

const Table = "message_sms_rate_limit_policy"

// Model is one of the two fixed platform scoped SMS sending policies. Policies
// are provisioned and removed together with the authentication platform, so the
// table has no soft delete column.
type Model struct {
	PlatformID    int64     `gorm:"column:platform_id;primaryKey"`
	Key           string    `gorm:"column:policy_key;primaryKey;type:varchar(64)"`
	Mode          string    `gorm:"column:mode;type:varchar(16);not null"`
	Dimension     string    `gorm:"column:dimension;type:varchar(64);not null"`
	Limit         int       `gorm:"column:limit_count;not null"`
	WindowSeconds int       `gorm:"column:window_seconds;not null"`
	Revision      int64     `gorm:"column:revision;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time `gorm:"column:updated_at;not null"`
}

func (Model) TableName() string { return Table }
