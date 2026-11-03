package transaction

import (
	"context"
	"fmt"
	"time"

	"admin/server/internal/shared/money"
)

type PersonalValue struct {
	ID            int64      `json:"id"`
	TransactionNo string     `json:"transactionNo"`
	SourceType    SourceType `json:"sourceType"`
	Direction     Direction  `json:"direction"`
	Amount        string     `json:"amount"`
	BalanceBefore string     `json:"balanceBefore"`
	BalanceAfter  string     `json:"balanceAfter"`
	CreatedAt     time.Time  `json:"createdAt"`
}

type Value struct {
	PersonalValue
	UserID          int64  `json:"userId"`
	Username        string `json:"username"`
	PlatformID      int64  `json:"platformId"`
	PlatformCode    string `json:"platformCode"`
	OrderID         *int64 `json:"orderId"`
	RedeemCodeID    *int64 `json:"redeemCodeId,omitempty"`
	AIUsageChargeID *int64 `json:"aiUsageChargeId,omitempty"`
}

func value(_ context.Context, row Model) (Value, error) {
	if row.ID < 1 {
		return Value{}, fmt.Errorf("stored payment transaction is invalid")
	}
	if err := row.Validate(); err != nil {
		return Value{}, err
	}
	source := SourcePayment
	if row.AIUsageChargeID != nil {
		source = SourceAI
	} else if row.RedeemCodeID != nil {
		source = SourceRedeemCode
	}
	amount, err := money.FormatRMBUnits(row.AmountUnits)
	if err != nil {
		return Value{}, err
	}
	before, err := money.FormatRMBUnits(row.BalanceBeforeUnits)
	if err != nil {
		return Value{}, err
	}
	after, err := money.FormatRMBUnits(row.BalanceAfterUnits)
	if err != nil {
		return Value{}, err
	}
	return Value{PersonalValue: PersonalValue{ID: row.ID, TransactionNo: row.TransactionNo, SourceType: source, Direction: row.Direction, Amount: amount, BalanceBefore: before, BalanceAfter: after, CreatedAt: row.CreatedAt}, UserID: row.UserID, Username: row.Username, PlatformID: row.PlatformID, PlatformCode: row.PlatformCode, OrderID: row.OrderID, RedeemCodeID: row.RedeemCodeID, AIUsageChargeID: row.AIUsageChargeID}, nil
}

type Page struct {
	List     []Value `json:"list"`
	Total    int64   `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"pageSize"`
}

func page(ctx context.Context, rows []Model, total int64, query ListQuery) (Page, error) {
	values := make([]Value, 0, len(rows))
	for _, row := range rows {
		projected, err := value(ctx, row)
		if err != nil {
			return Page{}, err
		}
		values = append(values, projected)
	}
	return Page{List: values, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}
