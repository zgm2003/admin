package run

import (
	pr "admin/server/internal/module/ai/providerRequest"
	"admin/server/internal/module/ai/usage"
	"encoding/json"
	"testing"
)

func TestTextUsageLinesSubtractCacheAndRoundOnce(t *testing.T) {
	s := fixtureSnapshot()
	c := RuntimeConfig{Rates: []RuntimeRate{
		{ID: 1, Operation: "text.generate", UsageKey: "input", Selector: json.RawMessage(`{}`), Unit: "token", UnitQuantity: "1000000", BillingUnitPriceUnits: 500000000},
		{ID: 2, Operation: "text.generate", UsageKey: "cacheInput", Selector: json.RawMessage(`{}`), Unit: "token", UnitQuantity: "1000000", BillingUnitPriceUnits: 100000000},
		{ID: 3, Operation: "text.generate", UsageKey: "output", Selector: json.RawMessage(`{}`), Unit: "token", UnitQuantity: "1000000", BillingUnitPriceUnits: 1000000000},
	}}
	s.Config, _ = json.Marshal(c)
	lines, e := textUsageLines(s, 9, pr.TokenUsage{Known: true, InputTokens: 100, CachedInputTokens: 80, OutputTokens: 3})
	if e != nil || len(lines) != 3 {
		t.Fatalf("lines=%+v err=%v", lines, e)
	}
	calc := make([]usage.LineInput, 0, len(lines))
	for _, l := range lines {
		calc = append(calc, l.Calculation)
	}
	total, e := usage.Calculate(calc)
	if e != nil || total.TotalUnits != 21000 {
		t.Fatalf("total=%+v err=%v", total, e)
	}
	if _, e = textUsageLines(s, 9, pr.TokenUsage{}); e == nil {
		t.Fatal("unknown usage accepted")
	}
	c.Rates = append(c.Rates, c.Rates[0])
	s.Config, _ = json.Marshal(c)
	if _, e = textUsageLines(s, 9, pr.TokenUsage{Known: true}); e == nil {
		t.Fatal("duplicate rates accepted")
	}
}

func TestTextUsageMissingCacheRateCannotBecomeFree(t *testing.T) {
	s := fixtureSnapshot()
	s.Config = json.RawMessage(`{"rates":[{"id":1,"operation":"text.generate","usageKey":"input","selector":{},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":1},{"id":2,"operation":"text.generate","usageKey":"output","selector":{},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":1}]}`)
	if _, e := textUsageLines(s, 1, pr.TokenUsage{Known: true, InputTokens: 1, CachedInputTokens: 1}); e == nil {
		t.Fatal("missing cache price accepted")
	}
}
