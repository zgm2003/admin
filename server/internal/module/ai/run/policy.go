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

// Policy is the complete, serializable engineering policy frozen for a Run.
// Values come from generation-checked setting.Reader records, never defaults.
type Policy struct {
	UserInflightLimit           int64 `json:"userInflightLimit"`
	UserRatePerSecond           int64 `json:"userRatePerSecond"`
	UserRateBurst               int64 `json:"userRateBurst"`
	GlobalRatePerSecond         int64 `json:"globalRatePerSecond"`
	GlobalRateBurst             int64 `json:"globalRateBurst"`
	QueuedBackpressureThreshold int64 `json:"queuedBackpressureThreshold"`
	ActiveBackpressureThreshold int64 `json:"activeBackpressureThreshold"`
	MaxQueuedSeconds            int64 `json:"maxQueuedSeconds"`
	TextTimeoutSeconds          int64 `json:"textTimeoutSeconds"`
	ImageTimeoutSeconds         int64 `json:"imageTimeoutSeconds"`
	AudioTimeoutSeconds         int64 `json:"audioTimeoutSeconds"`
	VideoTimeoutSeconds         int64 `json:"videoTimeoutSeconds"`
	MaxModelRounds              int64 `json:"maxModelRounds"`
	MaxToolCalls                int64 `json:"maxToolCalls"`
	MaxBoundTools               int64 `json:"maxBoundTools"`
	MaxInputAttachments         int64 `json:"maxInputAttachments"`
	MaxOutputTokens             int64 `json:"maxOutputTokens"`
	MaxTextInputBytes           int64 `json:"maxTextInputBytes"`
	MaxImages                   int64 `json:"maxImages"`
	ImageMaxBytes               int64 `json:"imageMaxBytes"`
	AudioMaxBytes               int64 `json:"audioMaxBytes"`
	VideoMaxBytes               int64 `json:"videoMaxBytes"`
	DownloadTimeoutSeconds      int64 `json:"downloadTimeoutSeconds"`
	TransferTimeoutSeconds      int64 `json:"transferTimeoutSeconds"`
	SpoolMaxBytesPerWorker      int64 `json:"spoolMaxBytesPerWorker"`
	EventRetentionDays          int64 `json:"eventRetentionDays"`
}

type policySetting struct {
	key   string
	value *int64
}

func policySettingsFor(p *Policy) []policySetting {
	return []policySetting{
		{"ai.run.user_inflight_limit", &p.UserInflightLimit}, {"ai.run.user_rate_per_second", &p.UserRatePerSecond}, {"ai.run.user_rate_burst", &p.UserRateBurst}, {"ai.run.global_rate_per_second", &p.GlobalRatePerSecond}, {"ai.run.global_rate_burst", &p.GlobalRateBurst},
		{"ai.run.queued_backpressure_threshold", &p.QueuedBackpressureThreshold}, {"ai.run.active_backpressure_threshold", &p.ActiveBackpressureThreshold}, {"ai.run.max_queued_seconds", &p.MaxQueuedSeconds}, {"ai.run.text_timeout_seconds", &p.TextTimeoutSeconds}, {"ai.run.image_timeout_seconds", &p.ImageTimeoutSeconds}, {"ai.run.audio_timeout_seconds", &p.AudioTimeoutSeconds}, {"ai.run.video_timeout_seconds", &p.VideoTimeoutSeconds},
		{"ai.run.max_model_rounds", &p.MaxModelRounds}, {"ai.run.max_tool_calls", &p.MaxToolCalls}, {"ai.run.max_bound_tools", &p.MaxBoundTools}, {"ai.run.max_input_attachments", &p.MaxInputAttachments}, {"ai.run.max_output_tokens", &p.MaxOutputTokens}, {"ai.run.max_text_input_bytes", &p.MaxTextInputBytes}, {"ai.run.max_images", &p.MaxImages},
		{"ai.media.image_max_bytes", &p.ImageMaxBytes}, {"ai.media.audio_max_bytes", &p.AudioMaxBytes}, {"ai.media.video_max_bytes", &p.VideoMaxBytes}, {"ai.media.download_timeout_seconds", &p.DownloadTimeoutSeconds}, {"ai.media.transfer_timeout_seconds", &p.TransferTimeoutSeconds}, {"ai.media.spool_max_bytes_per_worker", &p.SpoolMaxBytesPerWorker}, {"ai.run.event_retention_days", &p.EventRetentionDays},
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
	const maxSeconds = int64(time.Duration(1<<63-1) / time.Second)
	for _, seconds := range []int64{p.MaxQueuedSeconds, p.TextTimeoutSeconds, p.ImageTimeoutSeconds, p.AudioTimeoutSeconds, p.VideoTimeoutSeconds, p.DownloadTimeoutSeconds, p.TransferTimeoutSeconds} {
		if seconds > maxSeconds {
			return ErrPolicyInvalid
		}
	}
	if p.EventRetentionDays > maxSeconds/(24*60*60) {
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
