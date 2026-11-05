package run

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	cachefill "admin/server/internal/shared/cacheFill"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

var ErrPolicyInvalid = errors.New("AI engineering policy is unavailable or invalid")

// Policy is the admin-editable admission policy frozen for a Run. Model
// capability, agent behavior and engineering limits are intentionally not part
// of it; they live in ai_provider_model.limits, ai_agent and ai/limits.go.
type Policy struct {
	UserInflightLimit int64 `json:"userInflightLimit"`
	UserRatePerSecond int64 `json:"userRatePerSecond"`
	UserRateBurst     int64 `json:"userRateBurst"`
	MaxQueuedSeconds  int64 `json:"maxQueuedSeconds"`
}

type policySetting struct {
	key   string
	value *int64
}

func policySettingsFor(p *Policy) []policySetting {
	return []policySetting{
		{"ai.run.user_inflight_limit", &p.UserInflightLimit},
		{"ai.run.user_rate_per_second", &p.UserRatePerSecond},
		{"ai.run.user_rate_burst", &p.UserRateBurst},
		{"ai.run.max_queued_seconds", &p.MaxQueuedSeconds},
	}
}

func (p Policy) Validate() error {
	// JSON and Redis Lua both use IEEE-754 numbers. Reject a policy that would
	// silently round rather than freezing different effective limits.
	for _, setting := range policySettingsFor(&p) {
		if *setting.value <= 0 || *setting.value > 1<<53-1 {
			return fmt.Errorf("%w: %s", ErrPolicyInvalid, setting.key)
		}
	}
	if p.MaxQueuedSeconds > int64(time.Duration(1<<63-1)/time.Second) {
		return ErrPolicyInvalid
	}
	return nil
}

type PolicyReader struct{ settings sharedsetting.Reader }

func NewPolicyReader(settings sharedsetting.Reader) *PolicyReader {
	return &PolicyReader{settings: settings}
}

func (r *PolicyReader) Read(ctx context.Context) (Policy, error) {
	if r == nil || r.settings == nil {
		return Policy{}, ErrPolicyInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, cachefill.WorkTimeout)
	defer cancel()
	var policy Policy
	for _, setting := range policySettingsFor(&policy) {
		if err := ctx.Err(); err != nil {
			return Policy{}, errors.Join(ErrPolicyInvalid, err)
		}
		row, err := r.settings.FindByKey(ctx, setting.key)
		if err != nil {
			return Policy{}, errors.Join(ErrPolicyInvalid, err)
		}
		if row.Key != setting.key || row.ValueType != sharedsetting.ValueTypeNumber || row.Status != yesno.Yes || row.IsBuiltin != yesno.Yes || len(row.Value) == 0 || row.Value[0] < '1' || row.Value[0] > '9' {
			return Policy{}, fmt.Errorf("%w: %s", ErrPolicyInvalid, setting.key)
		}
		for _, char := range row.Value {
			if char < '0' || char > '9' {
				return Policy{}, fmt.Errorf("%w: %s", ErrPolicyInvalid, setting.key)
			}
		}
		value, err := strconv.ParseInt(row.Value, 10, 64)
		if err != nil {
			return Policy{}, fmt.Errorf("%w: %s", ErrPolicyInvalid, setting.key)
		}
		*setting.value = value
	}
	if err := policy.Validate(); err != nil {
		return Policy{}, err
	}
	return policy, nil
}
