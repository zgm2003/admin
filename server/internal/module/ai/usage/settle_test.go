package usage

import (
	chargeItem "admin/server/internal/module/ai/usage/chargeItem"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/ai/usage/charge"
	ledger "admin/server/internal/module/payment/transaction"
)

type settlementFake struct {
	row     charge.Model
	ledger  *ledger.Model
	debits  int
	settled bool
}

func (f *settlementFake) WithCharge(ctx context.Context, id int64, fn func(SettlementTx) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return fn(f)
}
func (f *settlementFake) FindFacts(context.Context) (SettlementFacts, error) {
	digest := f.row.SnapshotDigest
	if digest == "" {
		digest = "digest"
	}
	f.row.SnapshotDigest = digest
	if f.row.PriceVersionID == 0 {
		f.row.PriceVersionID = 8
	}
	if f.row.MultiplierPPM == 0 {
		f.row.MultiplierPPM = 1000000
	}
	facts := SettlementFacts{RunID: f.row.RunID, UserID: f.row.UserID, PlatformID: f.row.PlatformID, PriceVersionID: f.row.PriceVersionID, MultiplierPPM: f.row.MultiplierPPM, SnapshotDigest: digest, Terminal: true, Rates: []FrozenRate{{ID: 21, Operation: "text.generate", UsageKey: "output", Selector: json.RawMessage(`{}`), Unit: "token", UnitQuantity: "1", BillingUnitPriceUnits: 2}}}
	if f.row.ActualUnits != nil && *f.row.ActualUnits > 0 {
		facts.ProviderRequestIDs = []int64{11}
		facts.UsageDigests = map[int64]string{11: strings.Repeat("a", 64)}
		facts.RequiredItemKeys = []string{"11:output"}
		facts.Quantities = map[string]string{"11:output": fmt.Sprint(*f.row.ActualUnits / 2)}
		facts.ExpectedRates = map[string]int64{"11:output": 21}
	}
	return facts, nil
}
func (f *settlementFake) FindItems(context.Context) ([]chargeItem.Model, error) {
	if f.row.ActualUnits == nil || *f.row.ActualUnits == 0 {
		return nil, nil
	}
	return []chargeItem.Model{{ChargeID: f.row.ID, ProviderRequestID: 11, PriceRateID: 21, ItemKey: "11:output", Operation: "text.generate", UsageKey: "output", SelectorSnapshot: json.RawMessage(`{}`), Quantity: fmt.Sprint(*f.row.ActualUnits / 2), UnitQuantity: "1", Unit: "token", BillingUnitPriceUnits: 2, MultiplierPPM: 1000000, AmountUnits: *f.row.ActualUnits, RawUsageDigest: strings.Repeat("a", 64)}}, nil
}
func (f *settlementFake) Charge() charge.Model {
	if f.row.SnapshotDigest == "" {
		f.row.SnapshotDigest = "digest"
	}
	if f.row.PriceVersionID == 0 {
		f.row.PriceVersionID = 8
	}
	if f.row.MultiplierPPM == 0 {
		f.row.MultiplierPPM = 1000000
	}
	return f.row
}
func (f *settlementFake) FindLedger(context.Context) (ledger.Model, bool, error) {
	if f.ledger == nil {
		return ledger.Model{}, false, nil
	}
	return *f.ledger, true, nil
}
func (f *settlementFake) Debit(_ context.Context, amount int64, _ time.Time) (int64, int64, error) {
	f.debits++
	return 3, 3 - amount, nil
}
func (f *settlementFake) AppendLedger(_ context.Context, row ledger.Model) error {
	f.ledger = &row
	return nil
}
func (f *settlementFake) MarkSettled(_ context.Context, _ time.Time) error {
	f.row.Status = 2
	f.settled = true
	return nil
}

func TestSettleDebitsOnceAllowsNegativeAndValidatesReplay(t *testing.T) {
	amount := int64(10)
	store := &settlementFake{row: charge.Model{ID: 1, RunID: 7, UserID: 3, PlatformID: 4, Status: 1, Currency: "CNY", ActualUnits: &amount, UsageComplete: 1}}
	s := NewService(store)
	if err := s.Settle(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if store.ledger == nil || store.ledger.Direction != ledger.DirectionDebit || store.ledger.BalanceBeforeUnits != 3 || store.ledger.BalanceAfterUnits != -7 || store.ledger.AmountUnits != 10 || !store.settled {
		t.Fatalf("wrong charge settlement: %+v", store)
	}
	if err := s.Settle(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if store.debits != 1 {
		t.Fatal("replayed charge debited twice")
	}
	store.ledger.AmountUnits = 11
	if err := s.Settle(context.Background(), 1); !errors.Is(err, ErrLedgerConflict) {
		t.Fatalf("mismatched immutable ledger accepted: %v", err)
	}
}

func TestSettleUnknownIsNotZeroAndExplicitZeroHasNoLedger(t *testing.T) {
	store := &settlementFake{row: charge.Model{ID: 1, RunID: 7, UserID: 3, PlatformID: 4, Status: 4, Currency: "CNY"}}
	s := NewService(store)
	if err := s.Settle(context.Background(), 1); !errors.Is(err, ErrUsageUnknown) {
		t.Fatalf("unknown charge settled: %v", err)
	}
	if store.debits != 0 || store.settled || store.ledger != nil {
		t.Fatal("unknown charge changed financial facts")
	}
	zero := int64(0)
	store.row.Status = 1
	store.row.ActualUnits = &zero
	store.row.UsageComplete = 1
	if err := s.Settle(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if !store.settled || store.debits != 0 || store.ledger != nil {
		t.Fatal("known zero wrote a nonzero ledger")
	}
}
