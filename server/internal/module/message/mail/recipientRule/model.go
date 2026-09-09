package recipientrule

import (
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

const Table = "message_mail_recipient_rule"

type Model struct {
	ID        int64          `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Scope     string         `gorm:"column:scope;type:varchar(16);not null" json:"scope"`
	Pattern   string         `gorm:"column:pattern;type:varchar(254);not null" json:"pattern"`
	Action    string         `gorm:"column:action;type:varchar(16);not null" json:"action"`
	Name      string         `gorm:"column:name;type:varchar(128);not null" json:"name"`
	Remark    string         `gorm:"column:remark;type:varchar(512);not null" json:"remark"`
	IsEnabled yesno.Value    `gorm:"column:is_enabled;type:smallint;not null" json:"isEnabled"`
	CreatedAt time.Time      `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"column:updated_at;not null" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz" json:"-"`
}

func (Model) TableName() string { return "message_mail_recipient_rule" }
