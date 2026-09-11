package template

import (
	"encoding/json"
	"time"

	"admin/server/internal/shared/yesno"
)

const Table = "message_sms_template"

// Model is one of the four fixed SMS templates. Templates are never deleted, so
// there is deliberately no deleted_at column.
type Model struct {
	ID                int64           `gorm:"column:id;primaryKey;autoIncrement"`
	Scene             string          `gorm:"column:scene;type:varchar(32);not null"`
	Name              string          `gorm:"column:name;type:varchar(128);not null"`
	TencentTemplateID string          `gorm:"column:tencent_template_id;type:varchar(64);not null"`
	ParameterKeys     json.RawMessage `gorm:"column:parameter_keys;type:jsonb;not null"`
	ExampleVariables  json.RawMessage `gorm:"column:example_variables;type:jsonb;not null"`
	IsEnabled         yesno.Value     `gorm:"column:is_enabled;type:smallint;not null"`
	CreatedAt         time.Time       `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time       `gorm:"column:updated_at;not null"`
}

func (Model) TableName() string { return Table }

func jsonOf(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`null`)
	}
	return json.RawMessage(payload)
}
