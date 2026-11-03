package usage

import (
	"admin/server/internal/module/ai/usage/charge"
	chargeItem "admin/server/internal/module/ai/usage/chargeItem"
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateSettlementFactsRejectsUnknownRequestAndStaleFrozenRate(t *testing.T) {
	amount := int64(2)
	ch := charge.Model{ID: 1, RunID: 9, UserID: 3, PlatformID: 4, PriceVersionID: 8, MultiplierPPM: 1000000, SnapshotDigest: "digest", ActualUnits: &amount, UsageComplete: 1, Status: 1}
	facts := SettlementFacts{RunID: 9, UserID: 3, PlatformID: 4, PriceVersionID: 8, MultiplierPPM: 1000000, SnapshotDigest: "digest", Terminal: true, ProviderRequestIDs: []int64{11}, Rates: []FrozenRate{{ID: 21, Operation: "text.generate", UsageKey: "output", Selector: json.RawMessage(`{}`), Unit: "token", UnitQuantity: "1", BillingUnitPriceUnits: 2}}}
	item := chargeItem.Model{ChargeID: 1, ProviderRequestID: 99, PriceRateID: 21, Operation: "text.generate", UsageKey: "output", SelectorSnapshot: json.RawMessage(`{}`), Quantity: "1", UnitQuantity: "1", Unit: "token", BillingUnitPriceUnits: 2, MultiplierPPM: 1000000, AmountUnits: 2}
	if e := ValidateSettlementFacts(ch, []chargeItem.Model{item}, facts); e == nil {
		t.Fatal("unknown request accepted")
	}
	item.ProviderRequestID = 11
	item.BillingUnitPriceUnits = 3
	if e := ValidateSettlementFacts(ch, []chargeItem.Model{item}, facts); e == nil {
		t.Fatal("stale frozen rate accepted")
	}
}

func TestValidateSettlementFactsRoundsOnceAcrossAllItems(t *testing.T) {
	amount := int64(1)
	ch := charge.Model{ID: 1, RunID: 9, UserID: 3, PlatformID: 4, PriceVersionID: 8, MultiplierPPM: 1000000, SnapshotDigest: "digest", ActualUnits: &amount, UsageComplete: 1, Status: 1}
	facts := SettlementFacts{RunID: 9, UserID: 3, PlatformID: 4, PriceVersionID: 8, MultiplierPPM: 1000000, SnapshotDigest: "digest", Terminal: true, ProviderRequestIDs: []int64{11}, Rates: []FrozenRate{{ID: 21, Operation: "text.generate", UsageKey: "input", Selector: json.RawMessage(`{}`), Unit: "token", UnitQuantity: "10", BillingUnitPriceUnits: 1}, {ID: 22, Operation: "text.generate", UsageKey: "output", Selector: json.RawMessage(`{}`), Unit: "token", UnitQuantity: "10", BillingUnitPriceUnits: 1}}}
	items := []chargeItem.Model{
		{ChargeID: 1, ProviderRequestID: 11, PriceRateID: 21, ItemKey: "11:input", Operation: "text.generate", UsageKey: "input", SelectorSnapshot: json.RawMessage(`{}`), Quantity: "1", UnitQuantity: "10", Unit: "token", BillingUnitPriceUnits: 1, MultiplierPPM: 1000000, AmountUnits: 1, RawUsageDigest: strings.Repeat("a", 64)},
		{ChargeID: 1, ProviderRequestID: 11, PriceRateID: 22, ItemKey: "11:output", Operation: "text.generate", UsageKey: "output", SelectorSnapshot: json.RawMessage(`{}`), Quantity: "1", UnitQuantity: "10", Unit: "token", BillingUnitPriceUnits: 1, MultiplierPPM: 1000000, AmountUnits: 0, RawUsageDigest: strings.Repeat("a", 64)},
	}
	facts.UsageDigests = map[int64]string{11: strings.Repeat("a", 64)}
	facts.RequiredItemKeys = []string{"11:input", "11:output"}
	facts.Quantities = map[string]string{"11:input": "1", "11:output": "1"}
	facts.ExpectedRates = map[string]int64{"11:input": 21, "11:output": 22}
	if err := ValidateSettlementFacts(ch, items, facts); err != nil {
		t.Fatalf("valid apportioned items rejected: %v", err)
	}
	items[0].AmountUnits = 0
	items[1].AmountUnits = 1
	if err := ValidateSettlementFacts(ch, items, facts); err == nil {
		t.Fatal("noncanonical allocation accepted")
	}
}
func TestValidateSettlementFactsRequiresTerminalSnapshotAndExactTotal(t *testing.T) {
	amount := int64(2)
	ch := charge.Model{ID: 1, RunID: 9, UserID: 3, PlatformID: 4, PriceVersionID: 8, MultiplierPPM: 1000000, SnapshotDigest: "digest", ActualUnits: &amount, UsageComplete: 1, Status: 1}
	facts := SettlementFacts{RunID: 9, UserID: 3, PlatformID: 4, PriceVersionID: 8, MultiplierPPM: 1000000, SnapshotDigest: "digest", Terminal: false, ProviderRequestIDs: []int64{11}, Rates: []FrozenRate{{ID: 21, Operation: "text.generate", UsageKey: "output", Selector: json.RawMessage(`{}`), Unit: "token", UnitQuantity: "1", BillingUnitPriceUnits: 2}}}
	item := chargeItem.Model{ChargeID: 1, ProviderRequestID: 11, PriceRateID: 21, Operation: "text.generate", UsageKey: "output", SelectorSnapshot: json.RawMessage(`{}`), Quantity: "1", UnitQuantity: "1", Unit: "token", BillingUnitPriceUnits: 2, MultiplierPPM: 1000000, AmountUnits: 2}
	if e := ValidateSettlementFacts(ch, []chargeItem.Model{item}, facts); e == nil {
		t.Fatal("nonterminal accepted")
	}
	facts.Terminal = true
	item.AmountUnits = 3
	if e := ValidateSettlementFacts(ch, []chargeItem.Model{item}, facts); e == nil {
		t.Fatal("mismatched total accepted")
	}
}

func TestSettlementRejectsQuantityTamperingAndMissingUsageFacts(t *testing.T) {
	zero := int64(0)
	ch := charge.Model{ID: 1, RunID: 9, UserID: 3, PlatformID: 4, PriceVersionID: 8, MultiplierPPM: 1000000, SnapshotDigest: "digest", ActualUnits: &zero, UsageComplete: 1, Status: 1}
	facts := SettlementFacts{RunID: 9, UserID: 3, PlatformID: 4, PriceVersionID: 8, MultiplierPPM: 1000000, SnapshotDigest: "digest", Terminal: true, ProviderRequestIDs: []int64{11}, UsageDigests: map[int64]string{11: strings.Repeat("a", 64)}, RequiredItemKeys: []string{"11:output"}, Quantities: map[string]string{"11:output": "2"}, Rates: []FrozenRate{{ID: 21, Operation: "text.generate", UsageKey: "output", Selector: json.RawMessage(`{}`), Unit: "token", UnitQuantity: "1", BillingUnitPriceUnits: 0}}}
	item := chargeItem.Model{ChargeID: 1, ProviderRequestID: 11, PriceRateID: 21, ItemKey: "11:output", Operation: "text.generate", UsageKey: "output", SelectorSnapshot: json.RawMessage(`{}`), Quantity: "2.00000000", UnitQuantity: "1", Unit: "token", BillingUnitPriceUnits: 0, MultiplierPPM: 1000000, RawUsageDigest: strings.Repeat("a", 64)}
	facts.ExpectedRates = map[string]int64{"11:output": 21}
	if err := ValidateSettlementFacts(ch, []chargeItem.Model{item}, facts); err != nil {
		t.Fatal(err)
	}
	item.Quantity = "999"
	if err := ValidateSettlementFacts(ch, []chargeItem.Model{item}, facts); err == nil {
		t.Fatal("tampered zero-priced quantity accepted")
	}
	item.Quantity = "2"
	facts.UsageDigests = nil
	if err := ValidateSettlementFacts(ch, []chargeItem.Model{item}, facts); err == nil {
		t.Fatal("missing durable usage digest accepted")
	}
}

func TestSettlementRejectsAnotherImageTier(t *testing.T) {
	zero := int64(0)
	ch := charge.Model{ID: 1, RunID: 9, UserID: 3, PlatformID: 4, PriceVersionID: 8, MultiplierPPM: 1000000, SnapshotDigest: "digest", ActualUnits: &zero, UsageComplete: 1, Status: 1}
	low := FrozenRate{ID: 21, Operation: "image.generate", UsageKey: "output", Selector: json.RawMessage(`{"quality":"low","size":"1024x1024"}`), Unit: "image", UnitQuantity: "1", BillingUnitPriceUnits: 0}
	high := low
	high.ID = 22
	high.Selector = json.RawMessage(`{"quality":"high","size":"1024x1024"}`)
	facts := SettlementFacts{RunID: 9, UserID: 3, PlatformID: 4, PriceVersionID: 8, MultiplierPPM: 1000000, SnapshotDigest: "digest", Terminal: true, ProviderRequestIDs: []int64{11}, UsageDigests: map[int64]string{11: strings.Repeat("a", 64)}, RequiredItemKeys: []string{"11:output"}, Quantities: map[string]string{"11:output": "1"}, ExpectedRates: map[string]int64{"11:output": 21}, Rates: []FrozenRate{low, high}}
	item := chargeItem.Model{ChargeID: 1, ProviderRequestID: 11, PriceRateID: 22, ItemKey: "11:output", Operation: "image.generate", UsageKey: "output", SelectorSnapshot: high.Selector, Quantity: "1", UnitQuantity: "1", Unit: "image", BillingUnitPriceUnits: 0, MultiplierPPM: 1000000, RawUsageDigest: strings.Repeat("a", 64)}
	if err := ValidateSettlementFacts(ch, []chargeItem.Model{item}, facts); err == nil {
		t.Fatal("unselected image tier accepted")
	}
}
