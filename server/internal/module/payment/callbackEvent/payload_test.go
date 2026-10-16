package callbackevent

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func validEventRow(t *testing.T) Model {
	t.Helper()
	notifyID, tradeNo, seller := "NOTIFY-1", "TRADE-1", "20880001"
	payload := Payload{NotifyID: &notifyID, OrderNo: "ORDER-1", TradeNo: &tradeNo, TradeStatus: "TRADE_SUCCESS", AppID: "20260001", SellerID: &seller, TotalAmount: "3.00"}
	body, err := marshalPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	return Model{ID: 1, ConfigID: 3, ConfigVersion: 4, DedupeKey: dedupeKey(3, 4, payload), Payload: body, ProcessStatus: ProcessPending, CreatedAt: now, UpdatedAt: now}
}

func TestStoredEventMetadataCannotAdvertiseFalseTerminalSuccess(t *testing.T) {
	if _, err := value(context.Background(), validEventRow(t)); err != nil {
		t.Fatalf("valid baseline failed: %v", err)
	}
	for name, change := range map[string]func(*Model){
		"missing timestamp": func(row *Model) { row.CreatedAt = time.Time{} },
		"invalid parent":    func(row *Model) { zero := int64(0); row.OrderID = &zero },
		"wrong digest":      func(row *Model) { row.DedupeKey[0] ^= 1 },
		"success with error": func(row *Model) {
			id := int64(11)
			code := ErrorDependency
			row.OrderID = &id
			row.ProcessStatus = ProcessSuccess
			row.ErrorCode = &code
		},
		"success missing parent":   func(row *Model) { row.ProcessStatus = ProcessSuccess },
		"rejection without reason": func(row *Model) { row.ProcessStatus = ProcessRejected },
		"paid facts ignored":       func(row *Model) { row.ProcessStatus = ProcessIgnored },
	} {
		t.Run(name, func(t *testing.T) {
			row := validEventRow(t)
			change(&row)
			if _, err := value(context.Background(), row); err == nil {
				t.Fatal("corrupt stored event accepted")
			}
		})
	}
}

func TestDedupeUsesNotifyIdentityOrEntireClosedFactDigest(t *testing.T) {
	row := validEventRow(t)
	var payload Payload
	if err := json.Unmarshal(row.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	key := dedupeKey(3, 4, payload)
	payload.TotalAmount = "4.00"
	if string(dedupeKey(3, 4, payload)) != string(key) {
		t.Fatal("same notify ID changes key when amount changes")
	}
	payload.NotifyID = nil
	key = dedupeKey(3, 4, payload)
	payload.TradeStatus = "TRADE_FINISHED"
	if string(dedupeKey(3, 4, payload)) == string(key) {
		t.Fatal("fallback key ignores closed provider status")
	}
	if string(dedupeKey(3, 5, payload)) == string(dedupeKey(3, 4, payload)) {
		t.Fatal("key omits credential coordinates")
	}
}

func TestStoredPayloadRejectsUnclosedOrMalformedEvidence(t *testing.T) {
	for name, change := range map[string]func(map[string]json.RawMessage){
		"unknown secret":       func(fields map[string]json.RawMessage) { fields["privateKey"] = json.RawMessage(`"SECRET"`) },
		"missing optional key": func(fields map[string]json.RawMessage) { delete(fields, "notifyId") },
		"invalid amount":       func(fields map[string]json.RawMessage) { fields["totalAmount"] = json.RawMessage(`"3.001"`) },
		"zero amount":          func(fields map[string]json.RawMessage) { fields["totalAmount"] = json.RawMessage(`"0.00"`) },
		"noncanonical amount":  func(fields map[string]json.RawMessage) { fields["totalAmount"] = json.RawMessage(`"03.0"`) },
		"missing merchant":     func(fields map[string]json.RawMessage) { fields["sellerId"] = json.RawMessage(`null`) },
		"missing paid trade":   func(fields map[string]json.RawMessage) { fields["tradeNo"] = json.RawMessage(`null`) },
		"invalid time":         func(fields map[string]json.RawMessage) { fields["paidAt"] = json.RawMessage(`"0001-01-01T00:00:00Z"`) },
	} {
		t.Run(name, func(t *testing.T) {
			row := validEventRow(t)
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(row.Payload, &fields); err != nil {
				t.Fatal(err)
			}
			change(fields)
			body, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			row.Payload = body
			if _, err := value(context.Background(), row); err == nil {
				t.Fatal("corrupt stored evidence accepted")
			}
		})
	}
	row := validEventRow(t)
	row.Payload = append(row.Payload[:len(row.Payload)-1], []byte(`,"orderNo":"OTHER"}`)...)
	if _, err := value(context.Background(), row); err == nil {
		t.Fatal("duplicate payload key accepted")
	}
}
