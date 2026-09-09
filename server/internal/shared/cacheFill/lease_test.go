package cachefill

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/joho/godotenv"
	redis "github.com/redis/go-redis/v9"
)

func TestTwoInstancesBoundColdAdmissionAndProtectSuccessor(t *testing.T) {
	if testing.Short() {
		t.Skip("Redis integration")
	}
	if err := godotenv.Load("../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	opts, err := redis.ParseURL(os.Getenv("REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	a, b := redis.NewClient(opts), redis.NewClient(opts)
	defer a.Close()
	defer b.Close()
	ctx := context.Background()
	scope := fmt.Sprintf("test-fill-%d", time.Now().UnixNano())
	var winners atomic.Int32
	var wg sync.WaitGroup
	var owner *Lease
	var mu sync.Mutex
	start := make(chan struct{})
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			c := a
			if i%2 == 1 {
				c = b
			}
			l, e := Try(ctx, c, scope, "same")
			if e != nil {
				t.Error(e)
			}
			if l != nil {
				winners.Add(1)
				mu.Lock()
				owner = l
				mu.Unlock()
			}
		}(i)
	}
	close(start)
	wg.Wait()
	if winners.Load() != 1 {
		t.Fatalf("owners=%d", winners.Load())
	}
	// Simulate expiry without waiting six seconds, then let another instance win.
	if err := a.Del(ctx, owner.key).Err(); err != nil {
		t.Fatal(err)
	}
	next, err := Try(ctx, b, scope, "same")
	if err != nil || next == nil {
		t.Fatalf("successor=%v err=%v", next, err)
	}
	if err := owner.Release(ctx); err != nil {
		t.Fatal(err)
	}
	if v, err := a.Get(ctx, next.key).Result(); err != nil || v != next.token {
		t.Fatal("old owner removed successor")
	}
	if err := next.Release(ctx); err != nil {
		t.Fatal(err)
	}
	var held []*Lease
	for i := 0; i < 40; i++ {
		l, err := Try(ctx, a, scope, fmt.Sprint(i))
		if err != nil {
			t.Fatal(err)
		}
		if l != nil {
			held = append(held, l)
		}
	}
	if len(held) != 32 {
		t.Fatalf("global concurrent budget=%d", len(held))
	}
	for _, l := range held {
		if err := l.Release(ctx); err != nil {
			t.Fatal(err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := Try(canceled, a, scope, "cancel"); err == nil {
		t.Fatal("canceled admission accepted")
	}
}
