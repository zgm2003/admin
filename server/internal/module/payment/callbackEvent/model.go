package callbackevent

import (
	"encoding/json"
	"time"
)

type Model struct {
	ID            int64           `gorm:"column:id;primaryKey;autoIncrement"`
	ConfigID      int64           `gorm:"column:config_id;not null"`
	ConfigVersion int64           `gorm:"column:config_version;not null"`
	DedupeKey     []byte          `gorm:"column:dedupe_key;type:bytea;not null"`
	OrderID       *int64          `gorm:"column:order_id"`
	Payload       json.RawMessage `gorm:"column:payload;type:jsonb;not null"`
	ProcessStatus ProcessStatus   `gorm:"column:process_status;not null"`
	ErrorCode     *ErrorCode      `gorm:"column:error_code"`
	CreatedAt     time.Time       `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt     time.Time       `gorm:"column:updated_at;type:timestamptz;not null"`
}

func (Model) TableName() string { return "payment_callback_event" }
