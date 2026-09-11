package rateLimitPolicy

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	PermissionList   = "message:sms:list"
	PermissionUpdate = "message:sms:rate-limit:update"

	ModeBusiness   = "business"
	DimensionPhone = "platform_phone"

	KeyMinute = "business_phone_minute"
	KeyTenMin = "business_phone_10m"

	minLimit         = 1
	maxLimit         = 100000
	minWindowSeconds = 1
	maxWindowSeconds = 86400
)

// ErrPlatformNotFound reports an unknown or soft deleted authentication platform.
var ErrPlatformNotFound = errors.New("sms rate limit platform not found")

type Fixed struct {
	Key           string
	Mode          string
	Dimension     string
	Limit         int
	WindowSeconds int
}

// FixedPolicies is the only legal policy set. Scenes, the administrator test
// mode and IP addresses never get their own policy.
func FixedPolicies() []Fixed {
	return []Fixed{
		{Key: KeyMinute, Mode: ModeBusiness, Dimension: DimensionPhone, Limit: 1, WindowSeconds: 60},
		{Key: KeyTenMin, Mode: ModeBusiness, Dimension: DimensionPhone, Limit: 5, WindowSeconds: 600},
	}
}

func FixedPolicyByKey(key string) (Fixed, bool) {
	for _, fixed := range FixedPolicies() {
		if fixed.Key == key {
			return fixed, true
		}
	}
	return Fixed{}, false
}

func ValidateInput(key string, limit, windowSeconds int) error {
	if _, found := FixedPolicyByKey(key); !found {
		return fmt.Errorf("unknown sms rate limit policy key")
	}
	if limit < minLimit || limit > maxLimit {
		return fmt.Errorf("sms rate limit count must be between %d and %d", minLimit, maxLimit)
	}
	if windowSeconds < minWindowSeconds || windowSeconds > maxWindowSeconds {
		return fmt.Errorf("sms rate limit window must be between %d and %d seconds", minWindowSeconds, maxWindowSeconds)
	}
	return nil
}

type Input struct {
	Key           string
	Limit         int
	WindowSeconds int
}

type Catalog struct {
	PlatformID   int64
	PlatformCode string
	PlatformName string
	Policies     []Model
}

type PolicyResponse struct {
	Key           string `json:"key"`
	Mode          string `json:"mode"`
	Dimension     string `json:"dimension"`
	Limit         int    `json:"limit"`
	WindowSeconds int    `json:"windowSeconds"`
	Revision      int64  `json:"revision"`
	UpdatedAt     string `json:"updatedAt"`
}

type PlatformResponse struct {
	PlatformID   int64            `json:"platformId"`
	PlatformCode string           `json:"platformCode"`
	PlatformName string           `json:"platformName"`
	Policies     []PolicyResponse `json:"policies"`
}

type ListResponse struct {
	Platforms []PlatformResponse `json:"platforms"`
}

type repository interface {
	ListPlatforms(context.Context) ([]Catalog, error)
	FindPlatform(context.Context, int64) (Catalog, error)
	UpdatePolicy(context.Context, int64, string, int, int, time.Time) (Model, error)
	ProvisionDefaults(context.Context, int64, time.Time) error
	DeleteForPlatform(context.Context, int64) error
}

// Store publishes and invalidates the platform scoped runtime snapshot that the
// sending path reads. The database stays the source of truth.
type Store interface {
	Load(context.Context, int64, func(context.Context) (Catalog, error)) (Catalog, error)
	Mutate(context.Context, int64, func(context.Context) error) error
}

func policyResponseOf(value Model) PolicyResponse {
	return PolicyResponse{
		Key: value.Key, Mode: value.Mode, Dimension: value.Dimension,
		Limit: value.Limit, WindowSeconds: value.WindowSeconds,
		Revision: value.Revision, UpdatedAt: value.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}
