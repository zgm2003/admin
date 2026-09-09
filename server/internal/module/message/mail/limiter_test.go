package mail

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMailReservationReturnsSharedNextAllowance(t *testing.T) {
	client := openMailReadinessRedis(t).UniversalClient()
	ctx := context.Background()
	prefix := fmt.Sprintf("test:mail:next:%d:", time.Now().UnixNano())
	requests := []LimitRequest{{Key: prefix + "short", Limit: 2, Window: time.Minute}, {Key: prefix + "long", Limit: 2, Window: 10 * time.Minute}}
	t.Cleanup(func() { _ = client.Del(ctx, "rate:"+requests[0].Key, "rate:"+requests[1].Key).Err() })
	limiter := NewRedisLimiter(client)
	first, err := limiter.Reserve(ctx, requests...)
	if err != nil || !first.Allowed || first.RetryAfterSeconds != 0 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := limiter.Reserve(ctx, requests...)
	if err != nil || !second.Allowed || second.RetryAfterSeconds < 299 || second.RetryAfterSeconds > 300 {
		t.Fatalf("long window must govern next resend: %+v err=%v", second, err)
	}
	third, err := limiter.Reserve(ctx, requests...)
	if err != nil || third.Allowed || third.RetryAfterSeconds < 299 {
		t.Fatalf("third=%+v err=%v", third, err)
	}
}

func TestMailLimiterTwoInstancesShareBothWindows(t *testing.T) {
	first := openMailReadinessRedis(t).UniversalClient()
	second := openMailReadinessRedis(t).UniversalClient()
	ctx := context.Background()
	requests := businessLimitRequests(defaultPolicyCatalog(), time.Now().UnixNano(), "quota@example.com")
	var keys []string
	for _, request := range requests {
		keys = append(keys, "rate:"+request.Key)
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
