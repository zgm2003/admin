package order

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"admin/server/internal/module/payment"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	redis "github.com/redis/go-redis/v9"
)

func channelRedis(t *testing.T) (*redis.Client, *redis.Client, context.Context, string) {
	t.Helper()
	if testing.Short() {
		t.Skip("Redis integration")
	}
	if err := godotenv.Load("../../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	opts, err := redis.ParseURL(os.Getenv("REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	a, b := redis.NewClient(opts), redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	app := "test-payment-" + uuid.NewString()
	t.Cleanup(func() { cancel(); _ = a.Close(); _ = b.Close() })
	return a, b, ctx, app
}

func TestChannelLimiterSharesEightInflightSlotsAcrossInstances(t *testing.T) {
	a, b, ctx, app := channelRedis(t)
	limiters := []*RedisChannelLimiter{NewRedisChannelLimiter(a), NewRedisChannelLimiter(b)}
	start := make(chan struct{})
	results := make(chan func(context.Context) error, 64)
	var wg sync.WaitGroup
	for i := range 64 {
		wg.Go(func() {
			<-start
			release, err := limiters[i%2].Reserve(ctx, payment.EnvironmentSandbox, app)
			if err == nil {
				results <- release
			} else if !errors.Is(err, ErrChannelBudget) {
				t.Error(err)
			}
		})
	}
	close(start)
	wg.Wait()
	close(results)
	var held []func(context.Context) error
	for release := range results {
		held = append(held, release)
	}
	if len(held) != 8 {
		t.Fatalf("inflight winners=%d, want 8", len(held))
	}
	// Environment is part of the coordinate, while credential/config versions
	// are deliberately not part of it.
	prod, err := limiters[0].Reserve(ctx, payment.EnvironmentProduction, app)
	if err != nil {
		t.Fatal(err)
	}
	if err := prod(ctx); err != nil {
		t.Fatal(err)
	}
	for _, release := range held {
		if err := release(ctx); err != nil {
			t.Fatal(err)
		}
	}
	again, err := limiters[1].Reserve(ctx, payment.EnvironmentSandbox, app)
	if err != nil {
		t.Fatal(err)
	}
	if err := again(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestChannelLimiterDoesNotReleaseRateBudgetWithInflightSlot(t *testing.T) {
	a, _, ctx, app := channelRedis(t)
	l := NewRedisChannelLimiter(a)
	// Seed genuine recent admissions to isolate the rate gate from scheduler
	// timing. The next reservation must not pass merely because no I/O is live.
	slots, starts := channelBudgetKeys(payment.EnvironmentSandbox, app)
	now, err := a.Time(ctx).Result()
	if err != nil {
		t.Fatal(err)
	}
	members := make([]redis.Z, 40)
	for i := range members {
		members[i] = redis.Z{Score: float64(now.UnixMilli()), Member: fmt.Sprintf("start-%d", i)}
	}
	if err := a.ZAdd(ctx, starts, members...).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = a.Del(cleanup, slots, starts).Err()
	})
	if _, err := l.Reserve(ctx, payment.EnvironmentSandbox, app); !errors.Is(err, ErrChannelBudget) {
		t.Fatalf("41st start accepted: %v", err)
	}
	if err := a.ZAdd(ctx, starts, redis.Z{Score: float64(now.Add(-time.Second).UnixMilli()), Member: "start-0"}).Err(); err != nil {
		t.Fatal(err)
	}
	release, err := l.Reserve(ctx, payment.EnvironmentSandbox, app)
	if err != nil {
		t.Fatal(err)
	}
	if err := release(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Reserve(ctx, payment.EnvironmentSandbox, app); !errors.Is(err, ErrChannelBudget) {
		t.Fatalf("release returned consumed start token: %v", err)
	}
}

func TestChannelLimiterOldTokenCannotReleaseSuccessors(t *testing.T) {
	a, b, ctx, app := channelRedis(t)
	l := NewRedisChannelLimiter(a)
	old, err := l.Reserve(ctx, payment.EnvironmentSandbox, app)
	if err != nil {
		t.Fatal(err)
	}
	slots, _ := channelBudgetKeys(payment.EnvironmentSandbox, app)
	// Simulate process death and lease expiry, without sleeping six seconds.
	if err := a.Del(ctx, slots).Err(); err != nil {
		t.Fatal(err)
	}
	var successors []func(context.Context) error
	for range 8 {
		release, err := NewRedisChannelLimiter(b).Reserve(ctx, payment.EnvironmentSandbox, app)
		if err != nil {
			t.Fatal(err)
		}
		successors = append(successors, release)
	}
	if err := old(ctx); !errors.Is(err, ErrChannelLeaseLost) {
		t.Fatalf("old owner release=%v", err)
	}
	if _, err := l.Reserve(ctx, payment.EnvironmentSandbox, app); !errors.Is(err, ErrChannelBudget) {
		t.Fatalf("old token removed a successor: %v", err)
	}
	for _, release := range successors {
		if err := release(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

func TestChannelLimiterFailsClosedWithoutRedisOrIdentity(t *testing.T) {
	ctx := context.Background()
	if _, err := NewRedisChannelLimiter(nil).Reserve(ctx, payment.EnvironmentSandbox, "app"); err == nil {
		t.Fatal("missing Redis accepted")
	}
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	_ = client.Close()
	l := NewRedisChannelLimiter(client)
	for _, input := range []struct {
		env payment.Environment
		app string
	}{{0, "app"}, {payment.EnvironmentSandbox, ""}, {payment.EnvironmentSandbox, "app"}} {
		if _, err := l.Reserve(ctx, input.env, input.app); err == nil {
			t.Fatal("invalid or unavailable budget accepted")
		}
	}
}
