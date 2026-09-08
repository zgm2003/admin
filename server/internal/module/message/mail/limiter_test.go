package mail

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-redis/redis_rate/v10"
)

func TestMailLimiterTwoInstancesShareBothWindows(t *testing.T) {
	first := openMailReadinessRedis(t).UniversalClient()
	second := openMailReadinessRedis(t).UniversalClient()
	ctx := context.Background()
	requests := businessLimitRequests(defaultPolicyCatalog(), time.Now().UnixNano(), SceneLogin, "quota@example.com", "")
	var keys []string
	for _, request := range requests {
		for _, key := range append([]string{request.Key}, request.LegacyKeys...) {
			keys = append(keys, "rate:"+key)
		}
	}
	t.Cleanup(func() { _ = first.Del(ctx, keys...).Err() })
	var winners atomic.Int32
	var workers sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 32; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			<-start
			limiter := NewRedisLimiter(first)
			if i%2 == 1 {
				limiter = NewRedisLimiter(second)
			}
			allowed, err := limiter.Allow(ctx, requests...)
			if err != nil {
				t.Error(err)
			} else if allowed {
				winners.Add(1)
			}
		}(i)
	}
	close(start)
	workers.Wait()
	if winners.Load() != 1 {
		t.Fatalf("winners=%d", winners.Load())
	}
}

func TestMailLimiterMigratesLegacySceneDebtWithoutRefill(t *testing.T) {
	client := openMailReadinessRedis(t).UniversalClient()
	ctx := context.Background()
	requests := businessLimitRequests(defaultPolicyCatalog(), time.Now().UnixNano(), SceneForget, "legacy@example.com", "")
	var keys []string
	for _, request := range requests {
		for _, key := range append([]string{request.Key}, request.LegacyKeys...) {
			keys = append(keys, "rate:"+key)
		}
	}
	t.Cleanup(func() { _ = client.Del(ctx, keys...).Err() })
	legacy := redis_rate.NewLimiter(client)
	for _, request := range requests {
		for _, key := range request.LegacyKeys[:2] {
			if _, err := legacy.Allow(ctx, key, redis_rate.Limit{Rate: request.Limit, Burst: request.Limit, Period: request.Window}); err != nil {
				t.Fatal(err)
			}
		}
	}
	limiter := NewRedisLimiter(client)
	for i := 0; i < 2; i++ {
		if allowed, err := limiter.Allow(ctx, requests...); err != nil || allowed {
			t.Fatalf("legacy debt bypass: allowed=%v err=%v", allowed, err)
		}
	}
	for _, request := range requests {
		if ttl, err := client.PTTL(ctx, "rate:"+request.Key).Result(); err != nil || ttl <= request.Window/time.Duration(request.Limit) {
			t.Fatalf("merged debt TTL=%s err=%v", ttl, err)
		}
		for _, key := range request.LegacyKeys {
			if n, err := client.Exists(ctx, "rate:"+key).Result(); err != nil || n != 0 {
				t.Fatalf("legacy key remains: %v", err)
			}
		}
	}
}

func TestMailLimiterRejectionAndCorruptionDoNotDebitOtherWindow(t *testing.T) {
	client := openMailReadinessRedis(t).UniversalClient()
	ctx := context.Background()
	prefix := fmt.Sprintf("test:mail:atomic:%d:", time.Now().UnixNano())
	first := LimitRequest{Key: prefix + "minute", Limit: 10, Window: time.Minute}
	second := LimitRequest{Key: prefix + "ten", Limit: 1, Window: 10 * time.Minute}
	t.Cleanup(func() { _ = client.Del(ctx, "rate:"+first.Key, "rate:"+second.Key).Err() })
	limiter := NewRedisLimiter(client)
	if allowed, err := limiter.Allow(ctx, second); err != nil || !allowed {
		t.Fatal(err)
	}
	if allowed, err := limiter.Allow(ctx, first, second); err != nil || allowed {
		t.Fatalf("allowed=%v err=%v", allowed, err)
	}
	if n, _ := client.Exists(ctx, "rate:"+first.Key).Result(); n != 0 {
		t.Fatal("rejected send debited first window")
	}
	for _, value := range []string{"corrupt", "nan", "inf", "1e300"} {
		if err := client.Set(ctx, "rate:"+second.Key, value, time.Minute).Err(); err != nil {
			t.Fatal(err)
		}
		if _, err := limiter.Allow(ctx, first, second); err == nil {
			t.Fatalf("corrupt state %q accepted", value)
		}
		if n, err := client.Exists(ctx, "rate:"+first.Key).Result(); err != nil || n != 0 {
			t.Fatalf("partial write before corruption failure: n=%d err=%v", n, err)
		}
	}
}
