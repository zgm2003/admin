package usage

import (
	"context"
	"errors"
	"strconv"

	ledger "admin/server/internal/module/payment/transaction"
)

type SettlementPreview struct {
	ChargeID     string `json:"chargeId"`
	RunID        string `json:"runId"`
	Status       int16  `json:"status"`
	Currency     string `json:"currency"`
	ActualUnits  int64  `json:"actualUnits"`
	LedgerExists bool   `json:"ledgerExists"`
}

// Preview uses the settlement transaction's short row locks to inspect one
// consistent set of facts. It performs no writes, including failure recording.
func (s *Service) Preview(ctx context.Context, chargeID int64) (SettlementPreview, error) {
	var out SettlementPreview
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if s == nil || s.repository == nil || chargeID < 1 {
		return out, errors.New("AI settlement is not configured or charge is invalid")
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
		facts, err := tx.FindFacts(ctx)
		if err != nil {
			return err
		}
		items, err := tx.FindItems(ctx)
		if err != nil {
			return err
		}
		if err = ValidateSettlementFacts(row, items, facts); err != nil {
			return err
		}
		entry, found, err := tx.FindLedger(ctx)
		if err != nil {
			return err
		}
		actual := *row.ActualUnits
		if found && (actual == 0 || entry.AIUsageChargeID == nil || *entry.AIUsageChargeID != row.ID || entry.UserID != row.UserID || entry.PlatformID != row.PlatformID || entry.AmountUnits != actual || entry.Direction != ledger.DirectionDebit || entry.Validate() != nil) {
			return ErrLedgerConflict
		}
		if row.Status == 2 && ((actual > 0) != found) {
			return ErrLedgerConflict
		}
		out = SettlementPreview{ChargeID: strconv.FormatInt(row.ID, 10), RunID: strconv.FormatInt(row.RunID, 10), Status: row.Status, Currency: row.Currency, ActualUnits: actual, LedgerExists: found}
		return nil
	})
	if err != nil {
		return SettlementPreview{}, err
	}
	return out, nil
}
