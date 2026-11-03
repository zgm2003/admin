package provider

import (
	configcache "admin/server/internal/shared/configCache"
	"context"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"time"
)

var ErrNotFound = errors.New("AI provider not found")
var ErrInvalidConfiguration = errors.New("AI provider configuration is invalid")
var ErrRevisionConflict = errors.New("AI provider credential revision changed")

type Provider struct {
	ID               int64          `json:"id,string"`
	Code             string         `json:"code"`
	Name             string         `json:"name"`
	Description      string         `json:"description"`
	Status           int16          `json:"status"`
	CurrentVersion   int64          `json:"currentVersion,string"`
	APIKeyConfigured bool           `json:"apiKeyConfigured" gorm:"-"`
	CreatedBy        int64          `json:"-"`
	UpdatedBy        int64          `json:"-"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	DeletedAt        gorm.DeletedAt `json:"-"`
}

func (Provider) TableName() string { return "ai_provider" }

// ProviderVersion never crosses the public DTO boundary.
type ProviderVersion struct {
	ID, ProviderID, Version int64
	BaseURL                 string
	APIKeyCiphertext        string          `json:"-"`
	EncryptionKeyID         string          `json:"-"`
	HeadersCiphertext       *string         `json:"-"`
	MediaHostAllowlist      json.RawMessage `gorm:"type:jsonb"`
	RequestTimeoutMS        int
	CreatedBy               int64
	CreatedAt, UpdatedAt    time.Time
}

func (ProviderVersion) TableName() string { return "ai_provider_version" }

type ProviderModel struct {
	ID              int64           `json:"id,string"`
	ProviderID      int64           `json:"providerId,string"`
	UpstreamModelID string          `json:"upstreamModelId"`
	Name            string          `json:"name"`
	OfficialModelID *int64          `json:"officialModelId,omitempty,string"`
	Operations      []string        `json:"operations"`
	Limits          json.RawMessage `json:"limits"`
	Status          int16           `json:"status"`
}
type CreateRequest struct {
	Status             int16           `json:"status"`
	Code               string          `json:"code"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	BaseURL            string          `json:"baseUrl"`
	APIKey             string          `json:"apiKey"`
	Headers            json.RawMessage `json:"headers"`
	MediaHostAllowlist json.RawMessage `json:"mediaHostAllowlist"`
	RequestTimeoutMS   int             `json:"requestTimeoutMs"`
}
type UpdateRequest struct {
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	BaseURL            string          `json:"baseUrl"`
	APIKey             string          `json:"apiKey"`
	Headers            json.RawMessage `json:"headers"`
	MediaHostAllowlist json.RawMessage `json:"mediaHostAllowlist"`
	RequestTimeoutMS   int             `json:"requestTimeoutMs"`
}
type Repository interface {
	Mutate(context.Context, int64, func(Repository) error) (configcache.MutationResult, error)
	Create(context.Context, *Provider, *ProviderVersion) error
	Find(context.Context, int64) (*Provider, error)
	CurrentVersion(context.Context, int64) (*ProviderVersion, error)
	SaveRevision(context.Context, *Provider, *ProviderVersion) error
	WithProvider(context.Context, int64, func(Repository) error) error
	SetStatus(context.Context, int64, int16) error
	Models(context.Context, int64) ([]ProviderModel, error)
	SaveCatalog(context.Context, int64, []ProviderModel) error
}
