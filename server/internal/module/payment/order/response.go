package order

import (
	"context"
	"fmt"
	"time"

	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/money"
)

type Actions struct {
	Pay   bool `json:"pay"`
	Sync  bool `json:"sync"`
	Close bool `json:"close"`
}

type SafeValue struct {
	ID          int64      `json:"id"`
	OrderNo     string     `json:"orderNo"`
	PackageName string     `json:"packageName"`
	Amount      string     `json:"amount"`
	Status      Status     `json:"status"`
	StatusLabel string     `json:"statusLabel"`
	ExpiresAt   time.Time  `json:"expiresAt"`
	PaidAt      *time.Time `json:"paidAt"`
	CreditedAt  *time.Time `json:"creditedAt"`
	ClosedAt    *time.Time `json:"closedAt"`
	Actions     Actions    `json:"actions"`
}

func safeValue(ctx context.Context, row Model, now time.Time, purchaseAvailable bool) (SafeValue, error) {
	if row.ID < 1 || row.OrderNo == "" || row.UserID < 1 || row.PlatformID < 1 || row.PackageName == "" || row.AmountCents < 1 || row.Status < StatusPending || row.Status > StatusClosed || row.ExpiresAt.IsZero() {
		return SafeValue{}, fmt.Errorf("stored payment order is invalid")
	}
	amount, err := money.FormatRMBCents(row.AmountCents)
	if err != nil {
		return SafeValue{}, err
	}
	actions := Actions{}
	if purchaseAvailable && row.Status == StatusPending && row.ExpiresAt.After(now) {
		actions.Pay = true
	}
	statusLabel := "待支付"
	if i18n.LocaleFromContext(ctx) == i18n.EnUS {
		statusLabel = "Pending"
	}
	switch row.Status {
	case StatusPaid:
		statusLabel = "已支付并入账"
		if i18n.LocaleFromContext(ctx) == i18n.EnUS {
			statusLabel = "Paid and credited"
		}
	case StatusClosed:
		statusLabel = "已关闭"
		if i18n.LocaleFromContext(ctx) == i18n.EnUS {
			statusLabel = "Closed"
		}
	}
	packageName := row.PackageName
	if row.PackageID == nil {
		packageName = "自定义充值"
		if i18n.LocaleFromContext(ctx) == i18n.EnUS {
			packageName = "Custom recharge"
		}
	}
	return SafeValue{ID: row.ID, OrderNo: row.OrderNo, PackageName: packageName, Amount: amount, Status: row.Status, StatusLabel: statusLabel, ExpiresAt: row.ExpiresAt, PaidAt: row.PaidAt, CreditedAt: row.CreditedAt, ClosedAt: row.ClosedAt, Actions: actions}, nil
}
