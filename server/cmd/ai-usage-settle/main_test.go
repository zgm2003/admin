package main

import (
	"admin/server/internal/module/ai/usage"
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCLIOnlyAcceptsExistingChargeIDAndExplicitApply(t *testing.T) {
	v, err := parseArgs([]string{"--charge-id", "12"}, &bytes.Buffer{})
	if err != nil || v.Apply || v.ChargeID != 12 {
		t.Fatalf("%+v %v", v, err)
	}
	for _, args := range [][]string{nil, {"--charge-id", "0"}, {"--charge-id", "12", "--amount", "1"}, {"--charge-id", "12", "--user-id", "1"}, {"--charge-id", "12", "--price", "1"}, {"--charge-id", "12", "extra"}} {
		if _, err := parseArgs(args, &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	v, err = parseArgs([]string{"--charge-id", "12", "--apply"}, &bytes.Buffer{})
	if err != nil || !v.Apply {
		t.Fatal(v, err)
	}
}

type serviceProbe struct {
	previews, settles int
	fail              error
}

func (p *serviceProbe) Preview(context.Context, int64) (usage.SettlementPreview, error) {
	p.previews++
	return usage.SettlementPreview{ChargeID: "12", RunID: "7", ActualUnits: 10}, p.fail
}
func (p *serviceProbe) Settle(context.Context, int64) error { p.settles++; return p.fail }
func TestCLIDryRunDoesNotSettleAndApplyUsesExistingService(t *testing.T) {
	p := &serviceProbe{}
	var out bytes.Buffer
	if err := execute(context.Background(), p, settings{ChargeID: 12}, &out); err != nil || p.settles != 0 || !strings.Contains(out.String(), `"dryRun": true`) {
		t.Fatal(out.String(), err)
	}
	if err := execute(context.Background(), p, settings{ChargeID: 12, Apply: true}, &out); err != nil || p.settles != 1 || p.previews != 3 {
		t.Fatal(p, err)
	}
	p.fail = errors.New("secret DSN")
	if err := execute(context.Background(), p, settings{ChargeID: 12, Apply: true}, &out); err == nil || strings.Contains(err.Error(), "secret") || p.settles != 1 {
		t.Fatal(err)
	}
}
