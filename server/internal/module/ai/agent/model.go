package agent

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"gorm.io/gorm"
	"time"
)

type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
	b, e := pgtype.NewMap().Encode(pgtype.TextArrayOID, pgtype.TextFormatCode, pgtype.FlatArray[string](a), nil)
	return string(b), e
}
func (a *StringArray) Scan(v any) error {
	var b []byte
	switch x := v.(type) {
	case string:
		b = []byte(x)
	case []byte:
		b = x
	case nil:
		*a = nil
		return nil
	default:
		return fmt.Errorf("invalid array %T", v)
	}
	var d pgtype.FlatArray[string]
	if e := pgtype.NewMap().Scan(pgtype.TextArrayOID, pgtype.TextFormatCode, b, &d); e != nil {
		return e
	}
	*a = StringArray(d)
	return nil
}

type Model struct {
	ID               int64           `json:"id,string" gorm:"column:id;primaryKey"`
	Code             string          `json:"code"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	AvatarFileID     *int64          `json:"avatarFileId,string"`
	Capabilities     StringArray     `json:"capabilities" gorm:"type:text[]"`
	SystemPrompt     string          `json:"systemPrompt"`
	MultiplierPPM    int64           `json:"multiplierPpm"`
	ActiveBindingID  *int64          `json:"activeBindingId,string"`
	GenerationParams json.RawMessage `json:"generationParams" gorm:"type:jsonb"`
	Status           int16           `json:"status"`
	CreatedBy        int64           `json:"createdBy,string"`
	UpdatedBy        int64           `json:"updatedBy,string"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
	DeletedAt        gorm.DeletedAt  `json:"-"`
}

func (Model) TableName() string { return "ai_agent" }

type Binding struct {
	ID              int64           `json:"id,string" gorm:"primaryKey"`
	AgentID         int64           `json:"agentId,string"`
	ProviderModelID int64           `json:"providerModelId,string"`
	TextProtocol    int16           `json:"textProtocol"`
	Parameters      json.RawMessage `json:"parameters" gorm:"type:jsonb"`
	Status          int16           `json:"status"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

func (Binding) TableName() string { return "ai_agent_model_binding" }

type ToolBinding struct {
	ID                   int64           `json:"id,string" gorm:"primaryKey"`
	AgentID              int64           `json:"agentId,string"`
	ToolID               int64           `json:"toolId,string"`
	ExposedName          string          `json:"exposedName"`
	ParameterConstraints json.RawMessage `json:"parameterConstraints" gorm:"type:jsonb"`
	Status               int16           `json:"status"`
	CreatedAt            time.Time       `json:"createdAt"`
	UpdatedAt            time.Time       `json:"updatedAt"`
}

func (ToolBinding) TableName() string { return "ai_agent_tool" }

type BindingValidator interface {
	ValidateBinding(context.Context, int64, int16, []string) error
}

var ErrConfiguration = errors.New("invalid agent configuration")
var ErrReferenced = errors.New("referenced agent can only be disabled")

func configurationError(message string) error { return fmt.Errorf("%w: %s", ErrConfiguration, message) }
