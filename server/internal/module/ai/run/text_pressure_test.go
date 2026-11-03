package run

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type streamPressureFunc func(context.Context) (int64, error)

func (f streamPressureFunc) StreamBacklog(ctx context.Context) (int64, error) { return f(ctx) }

func TestTextPressureIntervalRefreshesAtMostOncePerSecondAndFailsClosed(t *testing.T) {
	now := time.Now()
	calls := 0
	backlog := int64(0)
	var failure error
	b := newTextDeltaBatch(context.Background(), func(context.Context, string) error { return nil }, streamPressureFunc(func(context.Context) (int64, error) { calls++; return backlog, failure }))
	b.now = func() time.Time { return now }
	if got, err := b.interval(); err != nil || got != 250*time.Millisecond {
		t.Fatal(got, err)
	}
	backlog = 100000
	for i := 0; i < 10; i++ {
		if _, err := b.interval(); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatal("pressure was read per chunk", calls)
	}
	now = now.Add(time.Second)
	if got, err := b.interval(); err != nil || got != time.Second {
		t.Fatal(got, err)
	}
	backlog = 0
	now = now.Add(time.Second)
	if got, err := b.interval(); err != nil || got != 250*time.Millisecond {
		t.Fatal(got, err)
	}
	failure = errors.New("shared pressure unavailable")
	now = now.Add(time.Second)
	if err := b.Append("must not emit"); !errors.Is(err, failure) || b.pending != "" {
		t.Fatal(err, b.pending)
	}
}

func TestTextPressureFailureRetainsPendingAndStopsWrites(t *testing.T) {
	now := time.Now()
	writes := 0
	var failure error
	b := newTextDeltaBatch(context.Background(), func(context.Context, string) error { writes++; return nil }, streamPressureFunc(func(context.Context) (int64, error) { return 0, failure }))
	b.now = func() time.Time { return now }
	if err := b.Append("confirmed"); err != nil {
		t.Fatal(err)
	}
	if err := b.Append("pending"); err != nil {
		t.Fatal(err)
	}
	failure = ErrGateUnavailable
	now = now.Add(time.Second)
	if err := b.Flush(); !errors.Is(err, ErrGateUnavailable) || writes != 1 || b.pending != "pending" {
		t.Fatal(err, writes, b.pending)
	}
}

func TestTextPressureOverloadWaitHonorsCancellationBeforeOneSecond(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	writes := 0
	b := newTextDeltaBatch(ctx, func(context.Context, string) error { writes++; return nil }, streamPressureFunc(func(context.Context) (int64, error) { return 100000, nil }))
	if err := b.Append("first"); err != nil {
		t.Fatal(err)
	}
	if err := b.Append("held"); err != nil {
		t.Fatal(err)
	}
	if err := b.Flush(); !errors.Is(err, context.DeadlineExceeded) || writes != 1 {
		t.Fatal(err, writes)
	}
}

func TestStreamPressureDoesNotQueryPostgresOrFallbackOnRedisFailure(t *testing.T) {
	calls := 0
	g := NewGate(&unavailableGateRedis{}, capacityCounterFunc(func(context.Context, int64, int64) (CapacityCounts, error) { calls++; return CapacityCounts{}, nil }))
	if _, err := g.StreamBacklog(context.Background()); !errors.Is(err, ErrGateUnavailable) || calls != 0 {
		t.Fatal(err, calls)
	}
}

func TestGateRedisOutboxPressureSharedAcrossInstancesAndRecovers(t *testing.T) {
	var backlog atomic.Int64
	var scans atomic.Int32
	backlog.Store(100001)
	a, b, client := gatePair(t, capacityCounterFunc(func(context.Context, int64, int64) (CapacityCounts, error) {
		scans.Add(1)
		return CapacityCounts{Backlog: backlog.Load()}, nil
	}))
	ctx := context.Background()
	policy := testGatePolicy(t)
	if ok, err := a.RefreshCapacity(ctx, policy); err != nil || !ok {
		t.Fatal(ok, err)
	}
	for _, g := range []*Gate{a, b} {
		if err := g.Check(ctx, 1, policy); !errors.Is(err, ErrGateDenied) {
			t.Fatal(err)
		}
		if n, err := g.StreamBacklog(ctx); err != nil || n != 100001 {
			t.Fatal(n, err)
		}
	}
	if scans.Load() != 1 {
		t.Fatal("streams triggered database scans", scans.Load())
	}
	if n, err := client.UniversalClient().Exists(ctx, a.prefix+"user:1").Result(); err != nil || n != 0 {
		t.Fatal("backpressure spent tokens", n, err)
	}
	backlog.Store(0)
	time.Sleep(1100 * time.Millisecond)
	if ok, err := b.RefreshCapacity(ctx, policy); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if err := a.Check(ctx, 1, policy); err != nil {
		t.Fatal(err)
	}
	if n, err := a.StreamBacklog(ctx); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	if err := client.Delete(ctx, a.prefix+"capacity"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.StreamBacklog(ctx); !errors.Is(err, ErrGateUnavailable) {
		t.Fatal(err)
	}
	if scans.Load() != 2 {
		t.Fatal("missing shared pressure scanned database", scans.Load())
	}
}
