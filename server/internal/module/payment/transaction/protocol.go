package transaction

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
)

type SourceType int16

const (
	SourcePayment    SourceType = 1
	SourceRedeemCode SourceType = 2
	SourceAI         SourceType = 3
)

func (v SourceType) Valid() bool { return v == SourcePayment || v == SourceRedeemCode || v == SourceAI }

type Direction int16

const (
	DirectionCredit Direction = 1
	DirectionDebit  Direction = 2
)

func (v Direction) Valid() bool { return v == DirectionCredit || v == DirectionDebit }

// Validate checks the same source, direction and signed arithmetic invariants as
// the append-only PostgreSQL ledger, before writes and when reading old facts.
func (row Model) Validate() error {
	if row.UserID < 1 || row.PlatformID < 1 || row.AmountUnits < 1 || !row.Direction.Valid() {
		return fmt.Errorf("payment transaction input invalid")
	}
	sources := 0
	for _, id := range []*int64{row.OrderID, row.RedeemCodeID, row.AIUsageChargeID} {
		if id != nil {
			if *id < 1 {
				return fmt.Errorf("payment transaction source invalid")
			}
			sources++
		}
	}
	if sources != 1 || (row.AIUsageChargeID != nil) != (row.Direction == DirectionDebit) {
		return fmt.Errorf("payment transaction source direction invalid")
	}
	var after int64
	if row.Direction == DirectionCredit {
		if row.BalanceBeforeUnits > math.MaxInt64-row.AmountUnits {
			return fmt.Errorf("payment transaction balance overflows")
		}
		after = row.BalanceBeforeUnits + row.AmountUnits
	} else {
		if row.BalanceBeforeUnits < math.MinInt64+row.AmountUnits {
			return fmt.Errorf("payment transaction balance overflows")
		}
		after = row.BalanceBeforeUnits - row.AmountUnits
	}
	if after != row.BalanceAfterUnits {
		return fmt.Errorf("payment transaction balance mismatch")
	}
	return nil
}

type repository interface {
	Find(context.Context, int64) (Model, error)
	Count(context.Context, ListQuery) (int64, error)
	List(context.Context, ListQuery) ([]Model, error)
}

type Service struct{ repository repository }

func NewService(repository repository) *Service { return &Service{repository: repository} }

func NewTransactionNo() (string, error) {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate transaction number: %w", err)
	}
	return "PT" + hex.EncodeToString(bytes), nil
}
