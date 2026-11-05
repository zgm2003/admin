package run

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	projectredis "admin/server/internal/redis"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

type capacityCounterFunc func(context.Context, int64, int64) (CapacityCounts, error)

func (f capacityCounterFunc) CountCapacity(ctx context.Context, q, a int64) (CapacityCounts, error) {
	return f(ctx, q, a)
}

type unavailableGateRedis struct{ calls int }

func (r *unavailableGateRedis) EvalString(context.Context, string, []string, ...any) (string, error) {
	r.calls++
	return "", errors.New("Redis unavailable")
}

func testGateLimits() GateLimits {
	return GateLimits{
		GlobalRatePerSecond:         200,
		GlobalRateBurst:             400,
		QueuedBackpressureThreshold: 1000,
		ActiveBackpressureThreshold: 2000,
	}
}

func testGatePolicy(t *testing.T) Policy {
	t.Helper()
	p, err := NewPolicyReader(validPolicySettings()).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGateDoesNotQueryPostgresWhenRedisIsUnavailableOrContextCanceled(t *testing.T) {
	r := &unavailableGateRedis{}
	calls := 0
	g := NewGate(r, capacityCounterFunc(func(context.Context, int64, int64) (CapacityCounts, error) { calls++; return CapacityCounts{}, nil }), testGateLimits())
	if err := g.Check(context.Background(), 1, testGatePolicy(t)); !errors.Is(err, ErrGateUnavailable) {
		t.Fatalf("err=%v", err)
	}
	if calls != 0 {
		t.Fatalf("PostgreSQL fallback calls=%d", calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := g.Check(ctx, 1, testGatePolicy(t)); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if r.calls != 1 || calls != 0 {
		t.Fatalf("canceled side effects redis=%d pg=%d", r.calls, calls)
	}
}

func openGateRedis(t *testing.T) *projectredis.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("Redis integration")
	}
	_, file, _, _ := runtime.Caller(0)
	if err := godotenv.Load(filepath.Join(filepath.Dir(file), "../../../../.env")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client, err := projectredis.Open(ctx, os.Getenv("REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func gatePair(t *testing.T, counter CapacityCounter) (*Gate, *Gate, *projectredis.Client) {
	t.Helper()
	first, second := openGateRedis(t), openGateRedis(t)
	prefix := "test:ai:run:gate:{" + uuid.NewString() + "}:"
	a, b := NewGate(first, counter, testGateLimits()), NewGate(second, counter, testGateLimits())
	a.prefix, b.prefix = prefix, prefix
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = first.ScanDelete(ctx, prefix+"*")
	})
	return a, b, first
}

func TestGateRedisTwoInstancesShareUserAndGlobalBuckets(t *testing.T) {
	var calls atomic.Int32
	counter := capacityCounterFunc(func(ctx context.Context, q, a int64) (CapacityCounts, error) {
		calls.Add(1)
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > time.Second {
			t.Error("capacity scan is unbounded")
		}
		return CapacityCounts{}, nil
	})
	a, b, client := gatePair(t, counter)
	ctx := context.Background()
	policy := testGatePolicy(t)
	if refreshed, err := a.RefreshCapacity(ctx, policy); err != nil || !refreshed {
		t.Fatalf("refresh=%v err=%v", refreshed, err)
	}
	for i := 0; i < 5; i++ {
		g := a
		if i%2 == 1 {
			g = b
		}
		if err := g.Check(ctx, 1, policy); err != nil {
			t.Fatalf("burst %d err=%v", i, err)
		}
	}
	err := b.Check(ctx, 1, policy)
	var denied *GateError
	if !errors.Is(err, ErrGateDenied) || !errors.As(err, &denied) || denied.RetryAfter <= 0 {
		t.Fatalf("limit error=%v", err)
	}
	if err := b.Check(ctx, 2, policy); err != nil {
		t.Fatalf("other user err=%v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("request path counted PostgreSQL %d times", calls.Load())
	}
	if ttl, err := client.UniversalClient().PTTL(ctx, a.prefix+"capacity").Result(); err != nil || ttl <= 0 || ttl > 2*time.Second {
		t.Fatalf("capacity TTL=%s err=%v", ttl, err)
	}
}

func TestGateRedisGlobalDenialDoesNotSpendUserToken(t *testing.T) {
	a, b, client := gatePair(t, capacityCounterFunc(func(context.Context, int64, int64) (CapacityCounts, error) { return CapacityCounts{}, nil }))
	ctx := context.Background()
	a.limits.GlobalRatePerSecond, a.limits.GlobalRateBurst = 1, 1
	b.limits.GlobalRatePerSecond, b.limits.GlobalRateBurst = 1, 1
	policy := testGatePolicy(t)
	if _, err := a.RefreshCapacity(ctx, policy); err != nil {
		t.Fatal(err)
	}
	if err := a.Check(ctx, 1, policy); err != nil {
		t.Fatal(err)
	}
	if err := b.Check(ctx, 2, policy); !errors.Is(err, ErrGateDenied) {
		t.Fatalf("global limit=%v", err)
	}
	if count, err := client.UniversalClient().Exists(ctx, a.prefix+"user:2").Result(); err != nil || count != 0 {
		t.Fatalf("denied user budget was spent: keys=%d err=%v", count, err)
	}
}

func TestGateRedisCapacityLeaderLimitsColdScansAndRejectsBadObservations(t *testing.T) {
	var calls atomic.Int32
	counter := capacityCounterFunc(func(context.Context, int64, int64) (CapacityCounts, error) {
		calls.Add(1)
		return CapacityCounts{}, nil
	})
	a, b, client := gatePair(t, counter)
	policy := testGatePolicy(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	start := make(chan struct{})
	// Each client already owns its Ping connection. Opening 32 new pooled
	// sockets is a separate load scenario; this test isolates ownership.
	errorsFound := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			g := a
			if i%2 == 1 {
				g = b
			}
			_, err := g.RefreshCapacity(ctx, policy)
			if err != nil {
				errorsFound <- err
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Fatalf("refresh err=%v timeout=%v canceled=%v scans=%d", err, errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled), calls.Load())
	}
	if calls.Load() != 1 {
		t.Fatalf("cold scans=%d", calls.Load())
	}
	if err := client.SetString(ctx, a.prefix+"capacity", "malformed", time.Second); err != nil {
		t.Fatal(err)
	}
	if err := b.Check(ctx, 1, policy); !errors.Is(err, ErrGateUnavailable) {
		t.Fatalf("corrupt observation=%v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("corrupt observation triggered PG=%d", calls.Load())
	}
	if err := client.Delete(ctx, a.prefix+"capacity"); err != nil {
		t.Fatal(err)
	}
	if err := b.Check(ctx, 1, policy); !errors.Is(err, ErrGateUnavailable) {
		t.Fatalf("missing observation=%v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("missing follower bypassed leader: PG=%d", calls.Load())
	}
}

func TestGateRedisCapacityBackpressureAndPolicyIncreaseFailClosed(t *testing.T) {
	a, b, _ := gatePair(t, capacityCounterFunc(func(context.Context, int64, int64) (CapacityCounts, error) {
		return CapacityCounts{Queued: 1000, Active: 1}, nil
	}))
	ctx := context.Background()
	policy := testGatePolicy(t)
	if _, err := a.RefreshCapacity(ctx, policy); err != nil {
		t.Fatal(err)
	}
	if err := b.Check(ctx, 1, policy); !errors.Is(err, ErrGateDenied) {
		t.Fatalf("queued threshold=%v", err)
	}
	a.limits.QueuedBackpressureThreshold = 2000
	b.limits.QueuedBackpressureThreshold = 2000
	if err := b.Check(ctx, 1, policy); !errors.Is(err, ErrGateUnavailable) {
		t.Fatalf("policy increased past observed bound=%v", err)
	}
}

func TestGateRedisRejectsMalformedBucketWithoutConsumption(t *testing.T) {
	a, _, client := gatePair(t, capacityCounterFunc(func(context.Context, int64, int64) (CapacityCounts, error) { return CapacityCounts{}, nil }))
	ctx := context.Background()
	policy := testGatePolicy(t)
	if _, err := a.RefreshCapacity(ctx, policy); err != nil {
		t.Fatal(err)
	}
	clock, err := client.UniversalClient().Time(ctx).Result()
	if err != nil {
		t.Fatal(err)
	}
	for _, tokens := range []string{"nan", "inf", "-1", "bad"} {
		if err := client.UniversalClient().HSet(ctx, a.prefix+"user:1", "tokens", tokens, "at", clock.UnixMicro()).Err(); err != nil {
			t.Fatal(err)
		}
		if err := a.Check(ctx, 1, policy); !errors.Is(err, ErrGateUnavailable) {
			t.Fatalf("tokens=%q admission err=%v", tokens, err)
		}
	}
}
