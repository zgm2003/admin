package transaction

import (
	"context"
	"fmt"
	"math"
	"time"

	"admin/server/internal/shared/money"
)

type PersonalValue struct {
	ID            int64      `json:"id"`
	TransactionNo string     `json:"transactionNo"`
	SourceType    SourceType `json:"sourceType"`
	Amount        string     `json:"amount"`
	BalanceBefore string     `json:"balanceBefore"`
	BalanceAfter  string     `json:"balanceAfter"`
	CreatedAt     time.Time  `json:"createdAt"`

	RunID        *int64 `json:"runId,omitempty"`
	AgentName    string `json:"agentName,omitempty"`
	ModelName    string `json:"modelName,omitempty"`
	InputTokens  int64  `json:"inputTokens,omitempty"`
	OutputTokens int64  `json:"outputTokens,omitempty"`
}

type Value struct {
	PersonalValue
	UserID       int64  `json:"userId"`
	Username     string `json:"username"`
	PlatformID   int64  `json:"platformId"`
	PlatformCode string `json:"platformCode"`
	OrderID      *int64 `json:"orderId"`
	RedeemCodeID *int64 `json:"redeemCodeId,omitempty"`
}

func value(_ context.Context, row Model) (Value, error) {
	if row.ID < 1 || row.UserID < 1 || row.PlatformID < 1 || row.AmountUnits == 0 {
		return Value{}, fmt.Errorf("stored payment transaction is invalid")
	}
	source := row.SourceType
	if !source.Valid() {
		switch {
		case row.OrderID != nil && row.RedeemCodeID == nil:
			source = SourcePayment
		case row.OrderID == nil && row.RedeemCodeID != nil:
			source = SourceRedeemCode
		default:
			return Value{}, fmt.Errorf("stored payment transaction source invalid")
		}
	}
	var beforeUnits int64
	switch source {
	case SourceAIUsage:
		if row.AmountUnits > 0 || row.AIUsageChargeID == nil || row.RunID == nil || row.BalanceBeforeUnits == nil {
			return Value{}, fmt.Errorf("stored AI usage transaction is invalid")
		}
		if row.AmountUnits == math.MinInt64 || *row.BalanceBeforeUnits < math.MinInt64-row.AmountUnits {
			return Value{}, fmt.Errorf("stored AI usage transaction balance overflows")
		}
		beforeUnits = *row.BalanceBeforeUnits
		if beforeUnits+row.AmountUnits != row.BalanceAfterUnits {
			return Value{}, fmt.Errorf("stored AI usage transaction balance mismatch")
		}
	default:
		if row.AmountUnits < 1 || row.AIUsageChargeID != nil {
			return Value{}, fmt.Errorf("stored payment transaction is invalid")
		}
		if (source == SourcePayment && (row.OrderID == nil || row.RedeemCodeID != nil)) ||
			(source == SourceRedeemCode && (row.OrderID != nil || row.RedeemCodeID == nil)) {
			return Value{}, fmt.Errorf("stored payment transaction source mismatch")
		}
		if row.BalanceAfterUnits < math.MinInt64+row.AmountUnits {
			return Value{}, fmt.Errorf("stored payment transaction balance before overflows")
		}
		beforeUnits = row.BalanceAfterUnits - row.AmountUnits
	}
	amount, err := money.FormatRMBUnits(row.AmountUnits)
	if err != nil {
		return Value{}, err
	}
	before, err := money.FormatRMBUnits(beforeUnits)
	if err != nil {
		return Value{}, err
	}
	after, err := money.FormatRMBUnits(row.BalanceAfterUnits)
	if err != nil {
		return Value{}, err
	}
	personal := PersonalValue{
		ID: row.ID, TransactionNo: row.TransactionNo, SourceType: source,
		Amount: amount, BalanceBefore: before, BalanceAfter: after, CreatedAt: row.CreatedAt,
	}
	if source == SourceAIUsage {
		personal.RunID = row.RunID
		personal.AgentName = row.AgentName
		personal.ModelName = row.ModelName
		personal.InputTokens = row.InputTokens
		personal.OutputTokens = row.OutputTokens
	}
	return Value{PersonalValue: personal, UserID: row.UserID, Username: row.Username, PlatformID: row.PlatformID, PlatformCode: row.PlatformCode, OrderID: row.OrderID, RedeemCodeID: row.RedeemCodeID}, nil
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
