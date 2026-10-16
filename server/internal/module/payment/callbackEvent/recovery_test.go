package callbackevent

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"admin/server/internal/module/payment/order"
)

func (r *eventRepositoryStub) ListPending(_ context.Context, cutoff, after time.Time, afterID int64, limit int) ([]Model, error) {
	rows := make([]Model, 0)
	for _, row := range r.rows {
		if row.ProcessStatus == ProcessPending && !row.CreatedAt.After(cutoff) && (row.CreatedAt.After(after) || (row.CreatedAt.Equal(after) && row.ID > afterID)) {
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].CreatedAt.Before(rows[j].CreatedAt)
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func recoverFixture(t *testing.T, f eventFixture) error {
	t.Helper()
	r, ok := any(f.service).(interface{ RecoverPending(context.Context) error })
	if !ok {
		t.Fatal("callback service has no durable pending recovery")
	}
	return r.RecoverPending(context.Background())
}

func TestRecoverPendingReplaysPersistedVerifiedEvidenceWithoutProviderRedelivery(t *testing.T) {
	f := newEventFixture(t)
	f.confirmer.err = order.ErrBalanceOverflow
	if _, err := f.service.Receive(context.Background(), 3, 4, f.signed(t, nil)); err == nil {
		t.Fatal("expected recoverable receive failure")
	}
	f.confirmer.err = nil
	now := f.service.now().Add(time.Minute)
	f.service.now = func() time.Time { return now }
	if err := recoverFixture(t, f); err != nil {
		t.Fatal(err)
	}
	if f.repo.rows[1].ProcessStatus != ProcessSuccess || f.confirmer.calls != 2 || f.repo.creates != 1 {
		t.Fatalf("recovery did not finish original evidence: row=%+v calls=%d creates=%d", f.repo.rows[1], f.confirmer.calls, f.repo.creates)
	}
	if err := recoverFixture(t, f); err != nil {
		t.Fatal(err)
	}
	if f.confirmer.calls != 2 {
		t.Fatal("terminal event was credited again")
	}
}

func TestRecoverPendingDoesNotTrustChangedPersistedEvidence(t *testing.T) {
	f := newEventFixture(t)
	f.confirmer.err = errors.New("database unavailable")
	if _, err := f.service.Receive(context.Background(), 3, 4, f.signed(t, nil)); err == nil {
		t.Fatal("expected pending receive")
	}
	row := f.repo.rows[1]
	row.DedupeKey = make([]byte, 32)
	f.repo.rows[1] = row
	f.confirmer.err = nil
	now := f.service.now().Add(time.Minute)
	f.service.now = func() time.Time { return now }
	if err := recoverFixture(t, f); err == nil {
		t.Fatal("corrupt evidence silently recovered")
	}
	if f.confirmer.calls != 1 || f.repo.rows[1].ProcessStatus != ProcessPending {
		t.Fatal("corrupt evidence reached settlement")
	}
}

func TestRecoverPendingVisitsFailedEvidenceOnlyOncePerRun(t *testing.T) {
	f := newEventFixture(t)
	f.confirmer.err = order.ErrBalanceOverflow
	if _, err := f.service.Receive(context.Background(), 3, 4, f.signed(t, nil)); err == nil {
		t.Fatal("expected pending receive")
	}
	now := f.service.now().Add(time.Minute)
	f.service.now = func() time.Time { return now }
	if err := recoverFixture(t, f); err == nil {
		t.Fatal("failed recovery hidden")
	}
	if f.confirmer.calls != 2 || f.repo.rows[1].ProcessStatus != ProcessPending {
		t.Fatalf("failed recovery looped: calls=%d", f.confirmer.calls)
	}
}
