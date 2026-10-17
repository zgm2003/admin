package rechargepackage

import (
	"admin/server/internal/shared/yesno"
	"context"
	"testing"
)

func TestOptionsAreStaticAndExposeExactLimits(t *testing.T) {
	options, err := (&Service{}).Options(context.Background())
	if err != nil || len(options.Statuses) != 2 || options.Statuses[0].Value != yesno.No || options.Statuses[1].Value != yesno.Yes || options.Defaults.IsEnabled != yesno.No || options.Limits.Name != 128 || options.Limits.Badge != 32 || options.Limits.AmountScale != 2 {
		t.Fatalf("options=%+v err=%v", options, err)
	}
	if options.Limits.MaximumAmount == "" || options.Limits.MinimumAmount != "0.01" {
		t.Fatalf("amount limits=%+v", options.Limits)
	}
}
