package callbackevent

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"admin/server/internal/payment/alipay"
	"admin/server/internal/shared/money"
)

type Payload struct {
	NotifyID    *string    `json:"notifyId"`
	OrderNo     string     `json:"orderNo"`
	TradeNo     *string    `json:"tradeNo"`
	TradeStatus string     `json:"tradeStatus"`
	AppID       string     `json:"appId"`
	SellerID    *string    `json:"sellerId"`
	TotalAmount string     `json:"totalAmount"`
	PaidAt      *time.Time `json:"paidAt"`
}

func payloadFromNotify(notify alipay.Notify) Payload {
	var notifyID, tradeNo *string
	if notify.NotifyID != "" {
		value := notify.NotifyID
		notifyID = &value
	}
	if notify.TradeNo != "" {
		value := notify.TradeNo
		tradeNo = &value
	}
	return Payload{NotifyID: notifyID, OrderNo: notify.OrderNo, TradeNo: tradeNo, TradeStatus: notify.Status, AppID: notify.AppID, SellerID: notify.SellerID, TotalAmount: fmt.Sprintf("%d.%02d", notify.AmountCents/100, notify.AmountCents%100), PaidAt: notify.PaidAt}
}

func marshalPayload(payload Payload) ([]byte, error) {
	if err := validatePayload(payload); err != nil {
		return nil, err
	}
	return json.Marshal(payload)
}

func dedupeKey(configID, version int64, payload Payload) []byte {
	coordinate := fmt.Sprintf("%d\x00%d\x00", configID, version)
	var seed []byte
	if payload.NotifyID != nil {
		seed = []byte(coordinate + "notify\x00" + *payload.NotifyID)
	} else {
		seed = []byte(coordinate + "facts\x00" + payloadFacts(payload))
	}
	digest := sha256.Sum256(seed)
	return digest[:]
}

// All fields have already passed validatePayload: identifiers and amounts may
// not contain NUL, nullable identifiers cannot be empty, and times are UTC.
func payloadFacts(payload Payload) string {
	optional := func(value *string) string {
		if value == nil {
			return ""
		}
		return *value
	}
	paidAt := ""
	if payload.PaidAt != nil {
		paidAt = payload.PaidAt.UTC().Format(time.RFC3339Nano)
	}
	return fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s", optional(payload.NotifyID), payload.OrderNo, optional(payload.TradeNo), payload.TradeStatus, payload.AppID, optional(payload.SellerID), payload.TotalAmount, paidAt)
}

func decodePayload(raw []byte) (Payload, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return Payload{}, fmt.Errorf("stored callback payload is invalid")
	}
	allowed := map[string]bool{"notifyId": true, "orderNo": true, "tradeNo": true, "tradeStatus": true, "appId": true, "sellerId": true, "totalAmount": true, "paidAt": true}
	seen := make(map[string]bool, len(allowed))
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || !allowed[key] || seen[key] {
			return Payload{}, fmt.Errorf("stored callback payload fields are invalid")
		}
		seen[key] = true
		var field json.RawMessage
		if err := decoder.Decode(&field); err != nil {
			return Payload{}, fmt.Errorf("stored callback payload is invalid")
		}
	}
	if closing, err := decoder.Token(); err != nil || closing != json.Delim('}') || len(seen) != len(allowed) {
		return Payload{}, fmt.Errorf("stored callback payload fields are incomplete")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return Payload{}, fmt.Errorf("stored callback payload has trailing data")
	}
	var payload Payload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Payload{}, fmt.Errorf("stored callback payload is invalid")
	}
	if err := validatePayload(payload); err != nil {
		return Payload{}, err
	}
	if payload.PaidAt != nil {
		utc := payload.PaidAt.UTC()
		payload.PaidAt = &utc
	}
	return payload, nil
}

func validatePayload(payload Payload) error {
	if !validFact(payload.OrderNo, 32) || !validFact(payload.AppID, 64) || !validFact(payload.TradeStatus, 64) || payload.SellerID == nil || !validFact(*payload.SellerID, 64) ||
		(payload.NotifyID != nil && !validFact(*payload.NotifyID, 128)) || (payload.TradeNo != nil && !validFact(*payload.TradeNo, 64)) ||
		(isPaidStatus(payload.TradeStatus) && payload.TradeNo == nil) || (payload.PaidAt != nil && payload.PaidAt.IsZero()) {
		return fmt.Errorf("stored callback payment facts are invalid")
	}
	cents, err := money.ParseRMBCents(payload.TotalAmount)
	if err != nil || cents <= 0 {
		return fmt.Errorf("stored callback amount is invalid")
	}
	if _, err := money.CentsToUnits(cents); err != nil {
		return fmt.Errorf("stored callback amount exceeds wallet range")
	}
	canonical, err := money.FormatRMBCents(cents)
	if err != nil || canonical != payload.TotalAmount {
		return fmt.Errorf("stored callback amount is not canonical")
	}
	return nil
}

func validFact(value string, maximum int) bool {
	if len(value) == 0 || len(value) > maximum {
		return false
	}
	for _, character := range []byte(value) {
		if character < 0x21 || character > 0x7e || character == '&' || character == '=' {
			return false
		}
	}
	return true
}
