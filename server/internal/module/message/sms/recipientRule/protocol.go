package recipientRule

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"admin/server/internal/secretkey"
	"admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/phone"
	"admin/server/internal/shared/yesno"
)

type Scope int16

const (
	ScopePhone  Scope = 0
	ScopePrefix Scope = 1
)

func (s Scope) Valid() bool { return s == ScopePhone || s == ScopePrefix }

type Action int16

const (
	ActionDeny  Action = 0
	ActionAllow Action = 1
)

func (a Action) Valid() bool { return a == ActionDeny || a == ActionAllow }

const (
	PermissionList   = "message:sms:list"
	PermissionCreate = "message:sms:rule:create"
	PermissionUpdate = "message:sms:rule:update"
	PermissionStatus = "message:sms:rule:status"
	PermissionDelete = "message:sms:rule:delete"

	ReasonDefaultAllow = "default_allow"
	ReasonPhoneExact   = "phone_exact"
	ReasonPrefixMatch  = "prefix_match"

	maxNameLength   = 128
	maxRemarkLength = 512
	countryCode     = "+86"
)

// ErrConflict is produced by the repository when the active partial unique index
// rejects a duplicate (scope, pattern, action) combination.
var ErrConflict = errors.New("sms recipient rule conflicts with an existing record")

var prefixNationalPattern = regexp.MustCompile(`^[0-9]{3,10}$`)

type Decision struct {
	Allowed bool
	RuleID  int64
	Reason  string
}

type Safe struct {
	ID        int64       `json:"id"`
	Scope     Scope       `json:"scope"`
	Pattern   string      `json:"pattern"`
	Action    Action      `json:"action"`
	Name      string      `json:"name"`
	Remark    string      `json:"remark"`
	IsEnabled yesno.Value `json:"isEnabled"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
}

type ListResponse struct {
	List []Safe `json:"list"`
}

type CreateInput struct {
	Scope     Scope
	Pattern   string
	Action    Action
	Name      string
	Remark    string
	IsEnabled yesno.Value
}

// UpdateInput keeps the pattern optional: omitting it preserves the stored
// pattern and scope, while a scope change requires a complete new pattern.
type UpdateInput struct {
	Scope     Scope
	Pattern   *string
	Action    Action
	Name      string
	Remark    string
	IsEnabled yesno.Value
}

type repository interface {
	List(context.Context) ([]Model, error)
	FindByID(context.Context, int64) (Model, error)
	Create(context.Context, *Model, int64, time.Time) (cachegeneration.MutationResult, error)
	Update(context.Context, *Model, int64, time.Time) (cachegeneration.MutationResult, error)
	UpdateStatus(context.Context, int64, int16, int64, time.Time) (cachegeneration.MutationResult, error)
	Delete(context.Context, int64, int64, time.Time) (cachegeneration.MutationResult, error)
}

// RuntimeCoordinator invalidates the SMS runtime snapshot after a write.
type RuntimeCoordinator interface {
	Mutate(context.Context, func(context.Context, int64) (cachegeneration.MutationResult, error)) error
}

func safeOf(value Model) Safe {
	return Safe{
		ID:        value.ID,
		Scope:     value.Scope,
		Pattern:   value.Pattern,
		Action:    value.Action,
		Name:      value.Name,
		Remark:    value.Remark,
		IsEnabled: value.IsEnabled,
		CreatedAt: value.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt: value.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

// HMACValue is retained only for internal rate-limit Redis keys. It is not
// persisted as a business field and never leaves the server API.
func HMACValue(keys *secretkey.KeyRing, value string) string {
	mac := hmac.New(sha256.New, keys.SMSRecipientHMACKey())
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func normalizePattern(scope Scope, value string) (string, error) {
	switch scope {
	case ScopePhone:
		return phone.Normalize(value)
	case ScopePrefix:
		trimmed := strings.TrimSpace(value)
		if !strings.HasPrefix(trimmed, countryCode) {
			return "", fmt.Errorf("sms prefix rule must start with the +86 country code")
		}
		national := strings.TrimPrefix(trimmed, countryCode)
		if !prefixNationalPattern.MatchString(national) {
			return "", fmt.Errorf("sms prefix rule must contain 3 to 10 national digits")
		}
		return countryCode + national, nil
	default:
		return "", fmt.Errorf("sms recipient rule scope is invalid")
	}
}

// ValidatePattern enforces the canonical stored SMS rule representation.
func ValidatePattern(scope Scope, value string) error {
	normalized, err := normalizePattern(scope, value)
	if err != nil || normalized != value {
		if err != nil {
			return err
		}
		return fmt.Errorf("sms pattern is not canonical")
	}
	return nil
}

func validAction(action Action) bool { return action.Valid() }
