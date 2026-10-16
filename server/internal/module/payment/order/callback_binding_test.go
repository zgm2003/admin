package order

import (
	"encoding/json"
	"errors"
	"testing"

	"admin/server/internal/module/payment"
)

func validCallbackEvidence(t *testing.T) callbackEvidence {
	t.Helper()
	proof := validConfirmation()
	fields := map[string]any{"notifyId": "NOTIFY-1", "orderNo": proof.OrderNo, "tradeNo": proof.TradeNo, "tradeStatus": proof.Status, "appId": proof.AppID, "sellerId": proof.SellerID, "totalAmount": "3.00", "paidAt": proof.PaidAt}
	body, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return callbackEvidence{ID: 1, ConfigID: proof.ConfigID, ConfigVersion: proof.ConfigVersion, ProcessStatus: 1, Payload: body}
}

func TestSettlementRequiresExactPersistedCallbackFacts(t *testing.T) {
	proof := validConfirmation()
	if err := matchCallbackEvidence(validCallbackEvidence(t), proof); err != nil {
		t.Fatalf("valid baseline rejected: %v", err)
	}
	processed := validCallbackEvidence(t)
	processed.ProcessStatus = 2
	if err := matchCallbackEvidence(processed, proof); err != nil {
		t.Fatalf("committed replay baseline rejected: %v", err)
	}
	for _, field := range []string{"orderNo", "tradeNo", "tradeStatus", "appId", "sellerId", "totalAmount", "paidAt"} {
		t.Run(field, func(t *testing.T) {
			row := validCallbackEvidence(t)
			var fields map[string]any
			if err := json.Unmarshal(row.Payload, &fields); err != nil {
				t.Fatal(err)
			}
			fields[field] = "OTHER"
			if field == "totalAmount" {
				fields[field] = "4.00"
			}
			if field == "paidAt" {
				fields[field] = nil
			}
			body, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			row.Payload = body
			if err := matchCallbackEvidence(row, proof); !errors.Is(err, ErrCallbackMismatch) {
				t.Fatalf("changed callback %s error=%v", field, err)
			}
		})
	}
	for _, change := range []func(*callbackEvidence){func(row *callbackEvidence) { row.ConfigID++ }, func(row *callbackEvidence) { row.ConfigVersion++ }, func(row *callbackEvidence) { row.ProcessStatus = 4 }} {
		row := validCallbackEvidence(t)
		change(&row)
		if err := matchCallbackEvidence(row, proof); !errors.Is(err, ErrCallbackMismatch) {
			t.Fatalf("wrong evidence coordinate/status error=%v", err)
		}
	}
	proof.Environment = payment.EnvironmentProduction
	// The environment is independently enforced against the locked channel row;
	// callback payload deliberately contains no redundant environment field.
	if err := matchCallbackEvidence(validCallbackEvidence(t), proof); err != nil {
		t.Fatalf("payload matching invented environment field: %v", err)
	}
}
