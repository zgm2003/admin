package template

import (
	"encoding/json"
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

const Table = "message_mail_template"

type Model struct {
	ID                int64           `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Scene             string          `gorm:"column:scene;type:varchar(32);not null" json:"scene"`
	Name              string          `gorm:"column:name;type:varchar(128);not null" json:"name"`
	Subject           string          `gorm:"column:subject;type:varchar(255);not null" json:"subject"`
	TencentTemplateID int             `gorm:"column:tencent_template_id;not null" json:"tencentTemplateId"`
	Variables         json.RawMessage `gorm:"column:variables;type:jsonb;not null" json:"variables"`
	ExampleVariables  json.RawMessage `gorm:"column:example_variables;type:jsonb;not null" json:"exampleVariables"`
	IsEnabled         yesno.Value     `gorm:"column:is_enabled;type:smallint;not null" json:"isEnabled"`
	CreatedAt         time.Time       `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt         time.Time       `gorm:"column:updated_at;not null" json:"updatedAt"`
	DeletedAt         gorm.DeletedAt  `gorm:"column:deleted_at;type:timestamptz" json:"-"`
}

func (Model) TableName() string { return "message_mail_template" }
