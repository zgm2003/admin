package sms

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	goredis "github.com/redis/go-redis/v9"

	"admin/server/internal/config"
)

func openLimiterRedis(t *testing.T) (goredis.UniversalClient, context.Context) {
	t.Helper()
	if err := godotenv.Load(filepath.Join("..", "..", "..", "..", ".env")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	client := goredis.NewClient(&goredis.Options{Addr: addressOf(t, settings.RedisURL)})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping redis: %v", err)
	}
	return client, ctx
}

func addressOf(t *testing.T, url string) string {
	t.Helper()
	trimmed := strings.TrimPrefix(strings.TrimPrefix(url, "redis://"), "rediss://")
	trimmed = strings.SplitN(trimmed, "/", 2)[0]
	if trimmed == "" {
		t.Fatal("redis address is empty")
	}
	return trimmed
}

func uniqueKey(prefix string) string {
	return prefix + ":" + time.Now().Format("150405.000000000")
}

func TestReserveAllowsWhenBothWindowsHaveRoom(t *testing.T) {
	client, ctx := openLimiterRedis(t)
	limiter := NewRedisLimiter(client)
	keyMinute, keyTen := uniqueKey("rate:sms:send:v1:test:minute"), uniqueKey("rate:sms:send:v1:test:ten")
	t.Cleanup(func() { _ = client.Del(context.Background(), keyMinute, keyTen).Err() })

	result, err := limiter.Reserve(ctx,
		LimitRequest{Key: keyMinute, Limit: 2, Window: time.Minute},
		LimitRequest{Key: keyTen, Limit: 5, Window: 10 * time.Minute},
	)
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if !result.Allowed || result.RetryAfterSeconds != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestReserveRejectsWhenTheShortWindowIsExhausted(t *testing.T) {
	client, ctx := openLimiterRedis(t)
	limiter := NewRedisLimiter(client)
	keyMinute, keyTen := uniqueKey("rate:sms:send:v1:test:minute"), uniqueKey("rate:sms:send:v1:test:ten")
	t.Cleanup(func() { _ = client.Del(context.Background(), keyMinute, keyTen).Err() })

	requests := []LimitRequest{
		{Key: keyMinute, Limit: 1, Window: time.Minute},
		{Key: keyTen, Limit: 5, Window: 10 * time.Minute},
	}
	if result, err := limiter.Reserve(ctx, requests...); err != nil || !result.Allowed {
		t.Fatalf("first reserve = %+v, %v", result, err)
	}
	denied, err := limiter.Reserve(ctx, requests...)
	if err != nil {
		t.Fatalf("second reserve error = %v", err)
	}
	if denied.Allowed || denied.RetryAfterSeconds <= 0 || denied.RetryAfterSeconds > 60 {
		t.Fatalf("denied = %+v", denied)
	}
}

func TestReserveRejectsWhenTheLongWindowIsExhausted(t *testing.T) {
	client, ctx := openLimiterRedis(t)
	limiter := NewRedisLimiter(client)
	keyMinute, keyTen := uniqueKey("rate:sms:send:v1:test:minute"), uniqueKey("rate:sms:send:v1:test:ten")
	t.Cleanup(func() { _ = client.Del(context.Background(), keyMinute, keyTen).Err() })

	requests := []LimitRequest{
		{Key: keyMinute, Limit: 100, Window: time.Minute},
		{Key: keyTen, Limit: 2, Window: 10 * time.Minute},
	}
	for attempt := 0; attempt < 2; attempt++ {
		if result, err := limiter.Reserve(ctx, requests...); err != nil || !result.Allowed {
			t.Fatalf("reserve %d = %+v, %v", attempt, result, err)
		}
	}
	denied, err := limiter.Reserve(ctx, requests...)
	if err != nil {
		t.Fatalf("third reserve error = %v", err)
	}
	if denied.Allowed || denied.RetryAfterSeconds <= 0 {
		t.Fatalf("denied = %+v", denied)
	}
}

func TestReserveSharesOneWindowPerKeyAndIsolatesDifferentKeys(t *testing.T) {
	client, ctx := openLimiterRedis(t)
	limiter := NewRedisLimiter(client)
	shared, other := uniqueKey("rate:sms:send:v1:test:shared"), uniqueKey("rate:sms:send:v1:test:other")
	t.Cleanup(func() { _ = client.Del(context.Background(), shared, other).Err() })

	// The key carries platform + phone hmac + policy key only, so every scene of
	// the same phone shares this window.
	if result, err := limiter.Reserve(ctx, LimitRequest{Key: shared, Limit: 1, Window: time.Minute}); err != nil || !result.Allowed {
		t.Fatalf("first shared reserve = %+v, %v", result, err)
	}
	if result, err := limiter.Reserve(ctx, LimitRequest{Key: shared, Limit: 1, Window: time.Minute}); err != nil || result.Allowed {
		t.Fatalf("second shared reserve = %+v, %v", result, err)
	}
	if result, err := limiter.Reserve(ctx, LimitRequest{Key: other, Limit: 1, Window: time.Minute}); err != nil || !result.Allowed {
		t.Fatalf("isolated reserve = %+v, %v", result, err)
	}
}

func TestReserveRejectsCorruptState(t *testing.T) {
	client, ctx := openLimiterRedis(t)
	limiter := NewRedisLimiter(client)
	key := uniqueKey("rate:sms:send:v1:test:corrupt")
	t.Cleanup(func() { _ = client.Del(context.Background(), key).Err() })
	if err := client.Set(ctx, key, "not-a-number", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}

	if _, err := limiter.Reserve(ctx, LimitRequest{Key: key, Limit: 1, Window: time.Minute}); err == nil ||
		!strings.Contains(strings.ToLower(err.Error()), "corrupt") {
		t.Fatalf("corrupt state error = %v", err)
	}
}

func TestReserveFailsClosedWhenRedisIsUnavailable(t *testing.T) {
	client := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1", DialTimeout: time.Second, MaxRetries: 0})
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if _, err := NewRedisLimiter(client).Reserve(ctx, LimitRequest{Key: "rate:sms:send:v1:test:down", Limit: 1, Window: time.Minute}); err == nil {
		t.Fatal("Reserve() accepted an unavailable Redis")
	}
}

func TestReserveRejectsInvalidRequests(t *testing.T) {
	client, ctx := openLimiterRedis(t)
	limiter := NewRedisLimiter(client)
	key := uniqueKey("rate:sms:send:v1:test:invalid")

	for _, requests := range [][]LimitRequest{
		{},
		{{Key: "", Limit: 1, Window: time.Minute}},
		{{Key: key, Limit: 0, Window: time.Minute}},
		{{Key: key, Limit: 1, Window: 0}},
		{{Key: key, Limit: 1, Window: 48 * time.Hour}},
		{{Key: key, Limit: 1, Window: time.Minute}, {Key: key, Limit: 1, Window: 10 * time.Minute}},
	} {
		if _, err := limiter.Reserve(ctx, requests...); err == nil {
			t.Fatalf("Reserve(%+v) accepted an invalid request", requests)
		}
	}
}
