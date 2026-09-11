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
	"admin/server/internal/shared/phone"
	"admin/server/internal/shared/yesno"
)

const (
	ScopePhone  = "phone"
	ScopePrefix = "prefix"

	ActionAllow = "allow"
	ActionDeny  = "deny"

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
	prefixMask      = "****"
)

// ErrConflict is produced by the repository when the active partial unique index
// rejects a duplicate (scope, pattern hmac, action) combination.
var ErrConflict = errors.New("sms recipient rule conflicts with an existing record")

var prefixNationalPattern = regexp.MustCompile(`^[0-9]{3,10}$`)

type Decision struct {
	Allowed bool
	RuleID  int64
	Reason  string
}

type Safe struct {
	ID          int64       `json:"id"`
	Scope       string      `json:"scope"`
	PatternHint string      `json:"patternHint"`
	Action      string      `json:"action"`
	Name        string      `json:"name"`
	Remark      string      `json:"remark"`
	IsEnabled   yesno.Value `json:"isEnabled"`
	CreatedAt   string      `json:"createdAt"`
	UpdatedAt   string      `json:"updatedAt"`
}

type ListResponse struct {
	List []Safe `json:"list"`
}

type CreateInput struct {
	Scope     string
	Pattern   string
	Action    string
	Name      string
	Remark    string
	IsEnabled yesno.Value
}

// UpdateInput keeps the pattern optional: omitting it preserves the stored
// pattern and scope, while a scope change requires a complete new pattern.
type UpdateInput struct {
	Scope     string
	Pattern   *string
	Action    string
	Name      string
	Remark    string
	IsEnabled yesno.Value
}

type repository interface {
	List(context.Context) ([]Model, error)
	FindByID(context.Context, int64) (Model, error)
	Create(context.Context, *Model) error
	Update(context.Context, *Model, time.Time) error
	UpdateStatus(context.Context, int64, int16, time.Time) error
	Delete(context.Context, int64) error
}

// RuntimeCoordinator invalidates the SMS runtime snapshot after a write.
type RuntimeCoordinator interface {
	Mutate(context.Context, func(context.Context) error) error
}

func safeOf(value Model) Safe {
	return Safe{
		ID:          value.ID,
		Scope:       value.Scope,
		PatternHint: value.PatternHint,
		Action:      value.Action,
		Name:        value.Name,
		Remark:      value.Remark,
		IsEnabled:   value.IsEnabled,
		CreatedAt:   value.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:   value.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

// HMACValue is the deterministic HMAC shared by rule patterns, log phone
// filters and quota keys. The plaintext never appears in any key or index.
func HMACValue(keys *secretkey.KeyRing, value string) string {
	mac := hmac.New(sha256.New, keys.SMSRecipientHMACKey())
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

// patternHMAC is the deterministic lookup value; it never leaves the server.
func patternHMAC(keys *secretkey.KeyRing, pattern string) string {
	return HMACValue(keys, pattern)
}

// patternHint is the only representation shown to administrators. Full numbers
// keep the mainland masking, prefixes keep the configured prefix.
func patternHint(scope, pattern string) string {
	if scope == ScopePhone {
		return phone.Hint(pattern)
	}
	return pattern + prefixMask
}

func normalizePattern(scope, value string) (string, error) {
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

func validAction(action string) bool {
	return action == ActionAllow || action == ActionDeny
}
