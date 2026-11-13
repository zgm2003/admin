package agent

import (
	"time"

	"admin/server/internal/module/ai/pricing"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type Model struct {
	ID              int64             `gorm:"column:id;primaryKey;autoIncrement"`
	Name            string            `gorm:"column:name;type:varchar(128);not null"`
	Description     string            `gorm:"column:description;type:varchar(500);not null"`
	Avatar          string            `gorm:"column:avatar;type:varchar(512);not null"`
	SystemPrompt    string            `gorm:"column:system_prompt;type:text;not null"`
	AgentType       pricing.ModelKind `gorm:"column:agent_type;type:smallint;not null"`
	Scene           Scene             `gorm:"column:scene;type:smallint;not null"`
	ProviderModelID int64             `gorm:"column:provider_model_id;not null"`
	TimeoutMS       int32             `gorm:"column:timeout_ms;type:integer;not null"`
	Status          yesno.Value       `gorm:"column:status;type:smallint;not null"`
	Version         int64             `gorm:"column:version;not null"`
	CreatedAt       time.Time         `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt       time.Time         `gorm:"column:updated_at;type:timestamptz;not null"`
	DeletedAt       gorm.DeletedAt    `gorm:"column:deleted_at;type:timestamptz;index"`
}

func (Model) TableName() string { return "ai_agent" }

type ModelRef struct {
	ID              int64               `gorm:"column:id"`
	ProviderID      int64               `gorm:"column:provider_id"`
	ProviderName    string              `gorm:"column:provider_name"`
	ProviderStatus  yesno.Value         `gorm:"column:provider_status"`
	ModelKey        string              `gorm:"column:model_key"`
	DisplayName     string              `gorm:"column:display_name"`
	ModelKind       pricing.ModelKind   `gorm:"column:model_kind"`
	PricingMode     pricing.PricingMode `gorm:"column:pricing_mode"`
	OfficialModelID *int64              `gorm:"column:official_model_id"`
	Status          yesno.Value         `gorm:"column:status"`
	Available       bool                `gorm:"column:available"`
}
