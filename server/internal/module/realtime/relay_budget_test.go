package realtime

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

// Both relay clients compete for the same last publication slot. This verifies
// the Redis-side bound and actual Pub/Sub, not a process-local limiter. It is
// a contract test, not a throughput or recovery benchmark.
func TestRelayRedisTwoInstancesShareSlidingPublicationBudget(t *testing.T) {
	first, second := openSubscriberRedis(t), openSubscriberRedis(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	key := "test:realtime:relay:budget:" + uuid.NewString()
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Second)
		defer cleanupCancel()
		_ = first.Delete(cleanupCtx, key)
	})
	clock, err := first.UniversalClient().Time(ctx).Result()
	if err != nil {
		t.Fatal(err)
	}
	members := make([]goredis.Z, 15999)
	for i := range members {
		members[i] = goredis.Z{Score: float64(clock.UnixMicro()), Member: strconv.Itoa(i)}
	}
	if err := first.UniversalClient().ZAdd(ctx, key, members...).Err(); err != nil {
		t.Fatal(err)
	}
	if err := first.UniversalClient().PExpire(ctx, key, 2*time.Second).Err(); err != nil {
		t.Fatal(err)
	}
	channel := "test:realtime:relay:channel:" + uuid.NewString()
	pubsub := first.UniversalClient().Subscribe(ctx, channel)
	defer pubsub.Close()
	if _, err := pubsub.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	firstRelay, secondRelay := NewRelay(nil, first, nil), NewRelay(nil, second, nil)
	firstRelay.budgetKey, secondRelay.budgetKey = key, key
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, relay := range []*Relay{firstRelay, secondRelay} {
		wg.Add(1)
		go func(r *Relay) {
			defer wg.Done()
			<-start
			attemptCtx, attemptCancel := context.WithTimeout(ctx, 150*time.Millisecond)
			defer attemptCancel()
			results <- r.publishBudgeted(attemptCtx, channel, []byte("payload"))
		}(relay)
	}
	close(start)
	wg.Wait()
	close(results)
	successes, timeouts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, context.DeadlineExceeded) {
			timeouts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || timeouts != 1 {
		t.Fatalf("successes=%d timeouts=%d", successes, timeouts)
	}
	message, err := pubsub.ReceiveMessage(ctx)
	if err != nil || message.Payload != "payload" {
		t.Fatalf("message=%v err=%v", message, err)
	}
	if size, err := first.UniversalClient().ZCard(ctx, key).Result(); err != nil || size != 16000 {
		t.Fatalf("window size=%d err=%v", size, err)
	}
	if ttl, err := first.UniversalClient().PTTL(ctx, key).Result(); err != nil || ttl <= 0 || ttl > 2*time.Second {
		t.Fatalf("ttl=%s err=%v", ttl, err)
	}
}
