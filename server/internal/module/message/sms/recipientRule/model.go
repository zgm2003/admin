package recipientRule

import (
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

const Table = "message_sms_recipient_rule"

// Model stores the rule pattern as SMS-domain ciphertext. The plaintext pattern
// is never persisted, never returned by the list endpoint and never cached.
type Model struct {
	ID                int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Scope             string         `gorm:"column:scope;type:varchar(16);not null"`
	PatternCiphertext string         `gorm:"column:pattern_ciphertext;not null"`
	PatternHint       string         `gorm:"column:pattern_hint;type:varchar(64);not null"`
	PatternHMAC       string         `gorm:"column:pattern_hmac;type:varchar(128);not null"`
	Action            string         `gorm:"column:action;type:varchar(16);not null"`
	Name              string         `gorm:"column:name;type:varchar(128);not null"`
	Remark            string         `gorm:"column:remark;type:varchar(512);not null"`
	IsEnabled         yesno.Value    `gorm:"column:is_enabled;type:smallint;not null"`
	CreatedAt         time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt         gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (Model) TableName() string { return Table }
