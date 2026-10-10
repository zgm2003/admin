package recipientRule

import (
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

const Table = "message_sms_recipient_rule"

// Model stores the normalized phone or prefix as plaintext for administration
// and matching. Provider credentials and verification codes are separate data.
type Model struct {
	ID        int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Scope     Scope          `gorm:"column:scope;type:smallint;not null"`
	Pattern   string         `gorm:"column:pattern;type:varchar(32);not null"`
	Action    Action         `gorm:"column:action;type:smallint;not null"`
	Name      string         `gorm:"column:name;type:varchar(128);not null"`
	Remark    string         `gorm:"column:remark;type:varchar(512);not null"`
	IsEnabled yesno.Value    `gorm:"column:is_enabled;type:smallint;not null"`
	CreatedAt time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (Model) TableName() string { return Table }
