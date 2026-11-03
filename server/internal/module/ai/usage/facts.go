package usage

import (
	"admin/server/internal/module/ai/usage/charge"
	chargeItem "admin/server/internal/module/ai/usage/chargeItem"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrFrozenSnapshotMismatch = errors.New("AI frozen usage snapshot mismatch")

type FrozenRate struct {
	ID                    int64
	Operation             string
	UsageKey              string
	Selector              json.RawMessage
	Unit                  string
	UnitQuantity          string
	BillingUnitPriceUnits int64
}
type SettlementFacts struct {
	RunID              int64
	UserID             int64
	PlatformID         int64
	PriceVersionID     int64
	MultiplierPPM      int64
	SnapshotDigest     string
	Terminal           bool
	ProviderRequestIDs []int64
	UsageDigests       map[int64]string
	RequiredItemKeys   []string
	Quantities         map[string]string
	ExpectedRates      map[string]int64
	Rates              []FrozenRate
}

func ValidateSettlementFacts(ch charge.Model, items []chargeItem.Model, facts SettlementFacts) error {
	if !facts.Terminal || facts.RunID != ch.RunID || facts.UserID != ch.UserID || facts.PlatformID != ch.PlatformID || facts.PriceVersionID != ch.PriceVersionID || facts.MultiplierPPM != ch.MultiplierPPM || facts.SnapshotDigest == "" || facts.SnapshotDigest != ch.SnapshotDigest || ch.ActualUnits == nil {
		return ErrFrozenSnapshotMismatch
	}
	if len(items) == 0 && *ch.ActualUnits != 0 {
		return ErrFrozenSnapshotMismatch
	}
	req := map[int64]bool{}
	for _, id := range facts.ProviderRequestIDs {
		if id < 1 || req[id] || len(facts.UsageDigests[id]) != 64 {
			return ErrFrozenSnapshotMismatch
		}
		req[id] = true
	}
	rates := map[int64]FrozenRate{}
	for _, r := range facts.Rates {
		if r.ID < 1 || r.BillingUnitPriceUnits < 0 || r.Operation == "" || r.UsageKey == "" || r.UnitQuantity == "" {
			return ErrFrozenSnapshotMismatch
		}
		rates[r.ID] = r
	}
	inputs := make([]LineInput, 0, len(items))
	amounts := make(map[string]int64, len(items))
	seen := map[string]bool{}
	for _, it := range items {
		if it.ChargeID != ch.ID || !req[it.ProviderRequestID] || len(it.RawUsageDigest) != 64 {
			return ErrFrozenSnapshotMismatch
		}
		r, ok := rates[it.PriceRateID]
		if !ok || it.Operation != r.Operation || it.UsageKey != r.UsageKey || it.Unit != r.Unit || it.UnitQuantity != r.UnitQuantity || it.BillingUnitPriceUnits != r.BillingUnitPriceUnits || !bytes.Equal(bytes.TrimSpace(it.SelectorSnapshot), bytes.TrimSpace(r.Selector)) {
			return ErrFrozenSnapshotMismatch
		}
		if _, e := hex.DecodeString(it.RawUsageDigest); e != nil {
			return ErrFrozenSnapshotMismatch
		}
		if expected := facts.UsageDigests[it.ProviderRequestID]; expected == "" || !strings.EqualFold(expected, it.RawUsageDigest) {
			return ErrFrozenSnapshotMismatch
		}
		key := it.ItemKey
		if key != fmt.Sprintf("%d:%s", it.ProviderRequestID, it.UsageKey) {
			return ErrFrozenSnapshotMismatch
		}
		if facts.ExpectedRates[key] != it.PriceRateID {
			return ErrFrozenSnapshotMismatch
		}
		expectedQuantity, ok := facts.Quantities[key]
		want, err := parseDecimal(expectedQuantity)
		got, gotErr := parseDecimal(it.Quantity)
		if !ok || err != nil || gotErr != nil || want.Cmp(got) != 0 {
			return ErrFrozenSnapshotMismatch
		}
		if seen[key] {
			return ErrFrozenSnapshotMismatch
		}
		seen[key] = true
		if it.MultiplierPPM != ch.MultiplierPPM || it.AmountUnits < 0 {
			return ErrFrozenSnapshotMismatch
		}
		inputs = append(inputs, LineInput{ItemKey: key, PriceUnits: r.BillingUnitPriceUnits, Quantity: it.Quantity, UnitQuantity: it.UnitQuantity, MultiplierPPM: it.MultiplierPPM})
		amounts[key] = it.AmountUnits
	}
	for _, required := range facts.RequiredItemKeys {
		if !seen[required] {
			return ErrFrozenSnapshotMismatch
		}
	}
	if len(seen) != len(facts.RequiredItemKeys) || len(seen) != len(facts.Quantities) {
		return ErrFrozenSnapshotMismatch
	}
	if len(inputs) == 0 {
		if *ch.ActualUnits == 0 {
			return nil
		}
		return ErrFrozenSnapshotMismatch
	}
	calculated, err := Calculate(inputs)
	if err != nil || calculated.TotalUnits != *ch.ActualUnits {
		return ErrFrozenSnapshotMismatch
	}
	for _, item := range calculated.Items {
		if amounts[item.ItemKey] != item.AmountUnits {
			return ErrFrozenSnapshotMismatch
		}
	}
	return nil
}
func SelectorDigest(raw json.RawMessage) string {
	v := sha256.Sum256(bytes.TrimSpace(raw))
	return hex.EncodeToString(v[:])
}

// UsageDigest is independent of JSONB whitespace/key ordering and preserves
// integers exactly, including values beyond JavaScript's safe integer range.
func UsageDigest(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	var canonical any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&canonical); err != nil {
		return "", err
	}
	normalized, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(normalized)
	return hex.EncodeToString(digest[:]), nil
}
