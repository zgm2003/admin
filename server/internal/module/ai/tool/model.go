package tool

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"time"
)

type Model struct {
	ID             int64          `json:"id,string" gorm:"primaryKey"`
	Code           string         `json:"code"`
	Name           string         `json:"name"`
	Type           Type           `json:"type"`
	CurrentVersion int64          `json:"currentVersion"`
	Status         Status         `json:"status"`
	CreatedBy      int64          `json:"createdBy,string"`
	UpdatedBy      int64          `json:"updatedBy,string"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `json:"-"`
}

func (Model) TableName() string { return "ai_tool" }

type Version struct {
	ID               int64           `json:"id,string" gorm:"primaryKey"`
	ToolID           int64           `json:"toolId,string"`
	Version          int64           `json:"version"`
	Type             Type            `json:"type"`
	Description      string          `json:"description"`
	InputSchema      json.RawMessage `json:"inputSchema" gorm:"type:jsonb"`
	OutputSchema     json.RawMessage `json:"outputSchema" gorm:"type:jsonb"`
	BuiltinKey       *string         `json:"builtinKey"`
	HTTPDefinition   json.RawMessage `json:"httpDefinition" gorm:"type:jsonb"`
	SecretCiphertext *string         `json:"-"`
	EncryptionKeyID  *string         `json:"-"`
	TimeoutMS        int             `json:"timeoutMs"`
	OutputLimitBytes int             `json:"outputLimitBytes"`
	HasSideEffect    int16           `json:"hasSideEffect"`
	IdempotencyMode  int16           `json:"idempotencyMode"`
	CreatedBy        int64           `json:"createdBy,string"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

func (Version) TableName() string { return "ai_tool_version" }
func versionOf(in VersionInput, actor int64) Version {
	v := Version{Type: in.Type, Description: in.Description, InputSchema: in.InputSchema, OutputSchema: in.OutputSchema, HTTPDefinition: in.HTTPDefinition, TimeoutMS: in.TimeoutMS, OutputLimitBytes: in.OutputLimitBytes, HasSideEffect: in.HasSideEffect, IdempotencyMode: in.IdempotencyMode, CreatedBy: actor}
	if in.Type == TypeBuiltin {
		v.HTTPDefinition = nil
	}
	if in.BuiltinKey != "" {
		v.BuiltinKey = &in.BuiltinKey
	}
	if in.SecretCiphertext != "" {
		v.SecretCiphertext = &in.SecretCiphertext
		v.EncryptionKeyID = &in.EncryptionKeyID
	}
	return v
}

var ErrReferenced = errors.New("referenced tool can only be disabled")
