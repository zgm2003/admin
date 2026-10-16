package order

import (
	"context"
	"time"

	"admin/server/internal/module/payment"
)

type Status int16

const (
	StatusPending Status = 1
	StatusPaid    Status = 2
	StatusClosed  Status = 3
)

type ProofSource int16

const (
	ProofSourceCallback ProofSource = 1
	ProofSourceQuery    ProofSource = 2
)

func (source ProofSource) Valid() bool {
	return source == ProofSourceCallback || source == ProofSourceQuery
}

func canTransition(from, to Status) bool {
	switch from {
	case StatusPending:
		return to == StatusPaid || to == StatusClosed
	case StatusClosed:
		return to == StatusPaid
	default:
		return false
	}
}

type ConfirmedPayment struct {
	ConfigID      int64
	ConfigVersion int64
	Environment   payment.Environment
	OrderNo       string
	TradeNo       string
	AppID         string
	SellerID      *string
	ProofSource   ProofSource
	AmountCents   int64
	Status        string
	PaidAt        *time.Time
}

type SettlementResult struct {
	OrderID       int64
	TransactionID int64
	BalanceUnits  int64
	Replayed      bool
}

type repository interface {
	Confirm(context.Context, ConfirmedPayment, *int64, time.Time) (SettlementResult, error)
}

var successStatuses = map[string]struct{}{"TRADE_SUCCESS": {}, "TRADE_FINISHED": {}}
