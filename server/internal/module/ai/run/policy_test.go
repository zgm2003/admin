package run

import (
	"context"
	"errors"
	"strconv"
	"testing"

	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

type policySettings map[string]sharedsetting.Record

func (s policySettings) FindByKey(ctx context.Context, key string) (sharedsetting.Record, error) {
	if err := ctx.Err(); err != nil {
		return sharedsetting.Record{}, err
	}
	row, ok := s[key]
	if !ok {
		return sharedsetting.Record{}, errors.New("missing setting")
	}
	return row, nil
}

func validPolicySettings() policySettings {
	values := map[string]int64{
		"ai.run.user_inflight_limit": 2, "ai.run.user_rate_per_second": 1, "ai.run.user_rate_burst": 5, "ai.run.global_rate_per_second": 200, "ai.run.global_rate_burst": 400,
		"ai.run.queued_backpressure_threshold": 1000, "ai.run.active_backpressure_threshold": 2000, "ai.run.max_queued_seconds": 60, "ai.run.text_timeout_seconds": 180, "ai.run.image_timeout_seconds": 300, "ai.run.audio_timeout_seconds": 300, "ai.run.video_timeout_seconds": 3600,
		"ai.run.max_model_rounds": 8, "ai.run.max_tool_calls": 16, "ai.run.max_bound_tools": 16, "ai.run.max_input_attachments": 4, "ai.run.max_output_tokens": 4096, "ai.run.max_text_input_bytes": 131072, "ai.run.max_images": 4,
		"ai.media.image_max_bytes": 10485760, "ai.media.audio_max_bytes": 104857600, "ai.media.video_max_bytes": 524288000, "ai.media.download_timeout_seconds": 300, "ai.media.transfer_timeout_seconds": 900, "ai.media.spool_max_bytes_per_worker": 2147483648, "ai.run.event_retention_days": 7,
	}
	rows := policySettings{}
	for key, value := range values {
		rows[key] = sharedsetting.Record{Key: key, Value: strconv.FormatInt(value, 10), ValueType: sharedsetting.ValueTypeNumber, Status: yesno.Yes, IsBuiltin: yesno.Yes}
	}
	return rows
}

func TestPolicyReaderLoadsEveryExplicitSettingWithoutDefaults(t *testing.T) {
	rows := validPolicySettings()
	policy, err := NewPolicyReader(rows).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if policy.UserInflightLimit != 2 || policy.GlobalRateBurst != 400 || policy.MaxModelRounds != 8 || policy.MaxTextInputBytes != 131072 || policy.SpoolMaxBytesPerWorker != 2147483648 || policy.TransferTimeoutSeconds != 900 || policy.EventRetentionDays != 7 {
		t.Fatalf("policy=%+v", policy)
	}
	if err := policy.Validate(); err != nil {
		t.Fatal(err)
	}
	for key := range rows {
		t.Run("missing_"+key, func(t *testing.T) {
			fixture := validPolicySettings()
			delete(fixture, key)
			if _, err := NewPolicyReader(fixture).Read(context.Background()); !errors.Is(err, ErrPolicyInvalid) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestPolicyReaderRejectsInvalidSettingFacts(t *testing.T) {
	for name, mutate := range map[string]func(*sharedsetting.Record){
		"wrong key": func(v *sharedsetting.Record) { v.Key = "ai.run.unknown" }, "disabled": func(v *sharedsetting.Record) { v.Status = yesno.No }, "not builtin": func(v *sharedsetting.Record) { v.IsBuiltin = yesno.No }, "wrong type": func(v *sharedsetting.Record) { v.ValueType = sharedsetting.ValueTypeString }, "zero": func(v *sharedsetting.Record) { v.Value = "0" }, "negative": func(v *sharedsetting.Record) { v.Value = "-1" }, "fraction": func(v *sharedsetting.Record) { v.Value = "1.5" }, "overflow": func(v *sharedsetting.Record) { v.Value = "9223372036854775808" }, "not exact JS integer": func(v *sharedsetting.Record) { v.Value = "9007199254740992" },
	} {
		t.Run(name, func(t *testing.T) {
			rows := validPolicySettings()
			row := rows["ai.run.user_inflight_limit"]
			mutate(&row)
			rows["ai.run.user_inflight_limit"] = row
			if _, err := NewPolicyReader(rows).Read(context.Background()); !errors.Is(err, ErrPolicyInvalid) {
				t.Fatalf("err=%v", err)
			}
		})
	}
	rows := validPolicySettings()
	row := rows["ai.run.video_timeout_seconds"]
	row.Value = "9223372037"
	rows[row.Key] = row
	if _, err := NewPolicyReader(rows).Read(context.Background()); !errors.Is(err, ErrPolicyInvalid) {
		t.Fatalf("duration overflow err=%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewPolicyReader(validPolicySettings()).Read(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation err=%v", err)
	}
}
