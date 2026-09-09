package ratelimitpolicy

import (
	"context"
	"time"
)

const PermissionUpdate = "message:mail:rate-limit:update"

type Input struct {
	Key           string
	Limit         int
	WindowSeconds int
}

type Catalog struct {
	Version  int64
	Policies []Model
}

type Store interface {
	Load(context.Context) (Catalog, error)
	Update(context.Context, Input) (Catalog, error)
}

type Snapshot struct {
	SchemaVersion int                       `json:"schemaVersion"`
	State         string                    `json:"state"`
	Version       int64                     `json:"version"`
	Policies      map[string]snapshotPolicy `json:"policies,omitempty"`
	MutationToken *string                   `json:"mutationToken"`
}

type snapshotPolicy struct {
	Limit         int       `json:"limit"`
	WindowSeconds int       `json:"windowSeconds"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type UpdateRequest struct {
	Limit         int `json:"limit"`
	WindowSeconds int `json:"windowSeconds"`
}

type ListResponse struct {
	Version  int64   `json:"version"`
	Policies []Model `json:"policies"`
}

type UpdateResponse struct {
	Version int64 `json:"version"`
	Policy  Model `json:"policy"`
}

type RateLimitPolicy = Model
type RateLimitPolicyInput = Input
type RateLimitCatalog = Catalog
type RateLimitSnapshot = Snapshot
type rateLimitPolicyValue = snapshotPolicy
