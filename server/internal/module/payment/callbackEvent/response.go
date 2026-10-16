package callbackevent

import (
	"context"
	"fmt"
	"time"
)

type Value struct {
	ID            int64         `json:"id"`
	ConfigID      int64         `json:"configId"`
	ConfigVersion int64         `json:"configVersion"`
	OrderID       *int64        `json:"orderId"`
	Payload       Payload       `json:"payload"`
	ProcessStatus ProcessStatus `json:"processStatus"`
	ErrorCode     *ErrorCode    `json:"errorCode"`
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
}

func value(_ context.Context, row Model) (Value, error) {
	if row.ID < 1 || row.ConfigID < 1 || row.ConfigVersion < 1 || len(row.DedupeKey) != 32 || len(row.Payload) == 0 || !row.ProcessStatus.Valid() || row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() {
		return Value{}, fmt.Errorf("stored callback event is invalid")
	}
	payload, err := decodePayload(row.Payload)
	if err != nil {
		return Value{}, err
	}
	if row.ErrorCode != nil && !row.ErrorCode.Valid() {
		return Value{}, fmt.Errorf("stored callback error code is invalid")
	}
	if string(row.DedupeKey) != string(dedupeKey(row.ConfigID, row.ConfigVersion, payload)) {
		return Value{}, fmt.Errorf("stored callback dedupe fact is invalid")
	}
	if row.OrderID != nil && *row.OrderID < 1 {
		return Value{}, fmt.Errorf("stored callback order id is invalid")
	}
	switch row.ProcessStatus {
	case ProcessSuccess:
		if row.OrderID == nil || row.ErrorCode != nil || !isPaidStatus(payload.TradeStatus) {
			return Value{}, fmt.Errorf("stored successful callback state is invalid")
		}
	case ProcessRejected:
		if row.ErrorCode == nil || *row.ErrorCode > ErrorFactConflict {
			return Value{}, fmt.Errorf("stored rejected callback state is invalid")
		}
	case ProcessIgnored:
		if row.ErrorCode != nil || !isIgnoredStatus(payload.TradeStatus) {
			return Value{}, fmt.Errorf("stored ignored callback state is invalid")
		}
	}
	return Value{ID: row.ID, ConfigID: row.ConfigID, ConfigVersion: row.ConfigVersion, OrderID: row.OrderID, Payload: payload, ProcessStatus: row.ProcessStatus, ErrorCode: row.ErrorCode, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}
