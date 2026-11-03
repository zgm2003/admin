package run

import (
	"encoding/json"
	"math"
	"testing"
)

func TestPublicAmountsPreserveIntegerPrecisionInJSON(t *testing.T) {
	amount := int64(math.MaxInt64)
	for _, tc := range []struct {
		name  string
		value any
		keys  []string
	}{
		{"run", PublicResponse(Model{EstimatedUnits: amount}), []string{"estimatedUnits"}},
		{"charge", ChargeResponse{ActualUnits: &amount}, []string{"actualUnits"}},
		{"item", ChargeItemResponse{BillingUnitPriceUnits: amount, AmountUnits: amount}, []string{"billingUnitPriceUnits", "amountUnits"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			for _, key := range tc.keys {
				if string(fields[key]) != `"9223372036854775807"` {
					t.Fatalf("%s=%s; amounts must remain exact strings in browsers", key, fields[key])
				}
			}
		})
	}
}

func TestUnknownChargeAmountRemainsNull(t *testing.T) {
	raw, err := json.Marshal(ChargeResponse{})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["actualUnits"]) != "null" {
		t.Fatalf("unknown amount=%s", fields["actualUnits"])
	}
}
