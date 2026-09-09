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
	PlatformID   int64
	PlatformCode string
	PlatformName string
	Version      int64
	Policies     []Model
}

type Store interface {
	Load(context.Context, int64) (Catalog, error)
	Update(context.Context, int64, Input) (Catalog, error)
}

type Snapshot struct {
	SchemaVersion int                       `json:"schemaVersion"`
	State         string                    `json:"state"`
	PlatformID    int64                     `json:"platformId"`
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

// PolicyResponse intentionally omits PlatformID. Platform scope belongs to
// the catalog envelope; repeating it on every policy makes the public DTO
// inconsistent with the frontend contract and invites scope mismatches.
type PolicyResponse struct {
	Key           string    `json:"key"`
	Mode          string    `json:"mode"`
	Dimension     string    `json:"dimension"`
	Limit         int       `json:"limit"`
	WindowSeconds int       `json:"windowSeconds"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func newPolicyResponse(value Model) PolicyResponse {
	return PolicyResponse{
		Key: value.Key, Mode: value.Mode, Dimension: value.Dimension,
		Limit: value.Limit, WindowSeconds: value.WindowSeconds, UpdatedAt: value.UpdatedAt,
	}
}

type PlatformResponse struct {
	PlatformID   int64            `json:"platformId"`
	PlatformCode string           `json:"platformCode"`
	PlatformName string           `json:"platformName"`
	Version      int64            `json:"version"`
	Policies     []PolicyResponse `json:"policies"`
}

type ListResponse struct {
	Platforms []PlatformResponse `json:"platforms"`
}

type UpdateResponse struct {
	PlatformID int64          `json:"platformId"`
	Version    int64          `json:"version"`
	Policy     PolicyResponse `json:"policy"`
}

type RateLimitPolicy = Model
type RateLimitPolicyInput = Input
type RateLimitCatalog = Catalog
type RateLimitSnapshot = Snapshot
type rateLimitPolicyValue = snapshotPolicy
