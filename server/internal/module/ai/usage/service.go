package usage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"admin/server/internal/module/ai/usage/charge"
	chargeItem "admin/server/internal/module/ai/usage/chargeItem"
	ledger "admin/server/internal/module/payment/transaction"
)

var ErrUsageUnknown = errors.New("AI usage is not complete")
var ErrLedgerConflict = errors.New("AI settlement conflicts with the immutable ledger")

// SettlementTx exposes only the accounting operations needed by settlement.
// Its implementation owns PostgreSQL; services do not receive GORM handles.
type SettlementTx interface {
	FindFacts(context.Context) (SettlementFacts, error)
	FindItems(context.Context) ([]chargeItem.Model, error)
	Charge() charge.Model
	FindLedger(context.Context) (ledger.Model, bool, error)
	Debit(context.Context, int64, time.Time) (int64, int64, error)
	AppendLedger(context.Context, ledger.Model) error
	MarkSettled(context.Context, time.Time) error
}
type settlementRepository interface {
	WithCharge(context.Context, int64, func(SettlementTx) error) error
}
type settlementFailureRecorder interface {
	MarkFailure(context.Context, int64, error, time.Time) error
}
type Service struct {
	repository settlementRepository
	now        func() time.Time
}

func NewService(repository settlementRepository) *Service {
	return &Service{repository: repository, now: time.Now}
}

func (s *Service) Settle(ctx context.Context, chargeID int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.repository == nil || chargeID < 1 {
		return errors.New("AI settlement is not configured or charge is invalid")
	}
	err := s.repository.WithCharge(ctx, chargeID, func(tx SettlementTx) error {
		row := tx.Charge()
		if row.ID != chargeID || row.UserID < 1 || row.PlatformID < 1 || row.RunID < 1 || row.Currency != "CNY" {
			return ErrLedgerConflict
		}
		if row.UsageComplete != 1 || row.ActualUnits == nil || *row.ActualUnits < 0 || row.Status == 0 || row.Status == 4 {
			return ErrUsageUnknown
		}
		if row.Status != 1 && row.Status != 2 && row.Status != 3 {
			return ErrLedgerConflict
		}
		facts, e := tx.FindFacts(ctx)
		if e != nil {
			return e
		}
		items, e := tx.FindItems(ctx)
		if e != nil {
			return e
		}
		if e = ValidateSettlementFacts(row, items, facts); e != nil {
			return e
		}
		actual := *row.ActualUnits
		existing, found, err := tx.FindLedger(ctx)
		if err != nil {
			return err
		}
		if found {
			if actual == 0 || existing.AIUsageChargeID == nil || *existing.AIUsageChargeID != row.ID || existing.UserID != row.UserID || existing.PlatformID != row.PlatformID || existing.AmountUnits != actual || existing.Direction != ledger.DirectionDebit || existing.Validate() != nil {
				return ErrLedgerConflict
			}
		}
		if row.Status == 2 {
			if (actual > 0) != found {
				return ErrLedgerConflict
			}
			return nil
		}
		now := s.now().UTC()
		if !found && actual > 0 {
			before, after, err := tx.Debit(ctx, actual, now)
			if err != nil {
				return err
			}
			id := row.ID
			entry := ledger.Model{TransactionNo: fmt.Sprintf("AI%020d", row.ID), UserID: row.UserID, PlatformID: row.PlatformID, Direction: ledger.DirectionDebit, AmountUnits: actual, BalanceBeforeUnits: before, BalanceAfterUnits: after, AIUsageChargeID: &id, CreatedAt: now, UpdatedAt: now}
			if err := entry.Validate(); err != nil {
				return err
			}
			if err := tx.AppendLedger(ctx, entry); err != nil {
				return err
			}
		}
		return tx.MarkSettled(ctx, now)
	})
	if err != nil {
		if recorder, ok := s.repository.(settlementFailureRecorder); ok {
			recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			recordErr := recorder.MarkFailure(recordCtx, chargeID, err, s.now().UTC())
			cancel()
			return errors.Join(err, recordErr)
		}
	}
	return err
}
