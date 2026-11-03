package usage

import (
	"admin/server/internal/module/ai/usage/charge"
	"context"
	"errors"
	"testing"
	"time"
)

type previewStore struct {
	*settlementFake
	failures     int
	invalidFacts bool
}

func (p *previewStore) MarkFailure(context.Context, int64, error, time.Time) error {
	p.failures++
	return nil
}
func (p *previewStore) WithCharge(ctx context.Context, id int64, fn func(SettlementTx) error) error {
	return fn(p)
}
func (p *previewStore) FindFacts(ctx context.Context) (SettlementFacts, error) {
	facts, err := p.settlementFake.FindFacts(ctx)
	if p.invalidFacts {
		facts.RunID++
	}
	return facts, err
}
func TestPreviewValidatesWithoutWritesOrFailureRecording(t *testing.T) {
	amount := int64(10)
	p := &previewStore{settlementFake: &settlementFake{row: charge.Model{ID: 1, RunID: 7, UserID: 3, PlatformID: 4, Status: 3, Currency: "CNY", ActualUnits: &amount, UsageComplete: 1}}}
	s := NewService(p)
	out, err := s.Preview(context.Background(), 1)
	if err != nil || out.ActualUnits != 10 || out.Status != 3 || out.ChargeID != "1" || out.LedgerExists {
		t.Fatalf("%+v %v", out, err)
	}
	p.invalidFacts = true
	if _, err = s.Preview(context.Background(), 1); !errors.Is(err, ErrFrozenSnapshotMismatch) {
		t.Fatal(err)
	}
	p.invalidFacts = false
	p.row.Status = 4
	if _, err = s.Preview(context.Background(), 1); !errors.Is(err, ErrUsageUnknown) {
		t.Fatal(err)
	}
	if p.debits != 0 || p.settled || p.ledger != nil || p.failures != 0 {
		t.Fatal("preview mutated accounting")
	}
}
func TestPreviewValidatesSettledLedgerAndCancellation(t *testing.T) {
	amount := int64(10)
	p := &previewStore{settlementFake: &settlementFake{row: charge.Model{ID: 1, RunID: 7, UserID: 3, PlatformID: 4, Status: 2, Currency: "CNY", ActualUnits: &amount, UsageComplete: 1}}}
	if _, err := NewService(p).Preview(context.Background(), 1); !errors.Is(err, ErrLedgerConflict) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewService(p).Preview(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
