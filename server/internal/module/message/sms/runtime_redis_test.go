package sms

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	projectredis "admin/server/internal/redis"
	"github.com/joho/godotenv"
)

func openSMSRuntimeRedis(t *testing.T) *projectredis.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("Redis integration")
	}
	if err := godotenv.Load("../../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	client, err := projectredis.Open(context.Background(), os.Getenv("REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func isolatedRuntimeCache(t *testing.T, client *projectredis.Client, namespace string) *RuntimeCache {
	t.Helper()
	t.Cleanup(func() { _ = client.ScanDelete(context.Background(), "*"+namespace+"*") })
	return &RuntimeCache{redis: client, namespace: namespace}
}

func runtimeFactsForCacheTest() RuntimeFacts {
	snapshot := validRuntimeSnapshotForTest()
	return factsOf(snapshot)
}

func TestRuntimeCacheTwoInstancesLoadPostgresOnceAndKeepHotReadsInRedis(t *testing.T) {
	first := isolatedRuntimeCache(t, openSMSRuntimeRedis(t), fmt.Sprintf(":test:%d", time.Now().UnixNano()))
	second := isolatedRuntimeCache(t, openSMSRuntimeRedis(t), first.namespace)
	var loads atomic.Int32
	loader := func(ctx context.Context) (RuntimeFacts, error) {
		loads.Add(1)
		timer := time.NewTimer(100 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return RuntimeFacts{}, ctx.Err()
		case <-timer.C:
			return runtimeFactsForCacheTest(), nil
		}
	}
	start := make(chan struct{})
	errorsFound := make(chan error, 64)
	var wait sync.WaitGroup
	stores := []*RuntimeCache{first, second}
	for index := 0; index < 64; index++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			<-start
			facts, err := stores[worker%2].Load(context.Background(), loader)
			if err != nil || !facts.Configured {
				errorsFound <- fmt.Errorf("load=%+v err=%w", facts, err)
			}
		}(index)
	}
	close(start)
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Fatal(err)
	}
	if loads.Load() != 1 {
		t.Fatalf("source loads=%d, want 1", loads.Load())
	}
	for index := 0; index < 100; index++ {
		if _, err := stores[index%2].Load(context.Background(), loader); err != nil {
			t.Fatal(err)
		}
	}
	if loads.Load() != 1 {
		t.Fatalf("hot reads reached source: %d", loads.Load())
	}
}

func TestReadinessCacheTwoInstancesComputeColdValueOnce(t *testing.T) {
	first := isolatedRuntimeCache(t, openSMSRuntimeRedis(t), fmt.Sprintf(":test:%d", time.Now().UnixNano()))
	second := isolatedRuntimeCache(t, openSMSRuntimeRedis(t), first.namespace)
	var computes atomic.Int32
	compute := func(ctx context.Context) (VerifyCodeReadiness, error) {
		computes.Add(1)
		timer := time.NewTimer(100 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return VerifyCodeReadiness{}, ctx.Err()
		case <-timer.C:
			return VerifyCodeReadiness{Ready: true, TTLMinutes: 5}, nil
		}
	}
	start := make(chan struct{})
	errorsFound := make(chan error, 64)
	var wait sync.WaitGroup
	stores := []*RuntimeCache{first, second}
	for index := 0; index < 64; index++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			<-start
			ready, err := stores[worker%2].LoadReadiness(context.Background(), "login", compute)
			if err != nil || !ready.Ready || ready.TTLMinutes != 5 {
				errorsFound <- fmt.Errorf("readiness=%+v err=%w", ready, err)
			}
		}(index)
	}
	close(start)
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Fatal(err)
	}
	if computes.Load() != 1 {
		t.Fatalf("readiness computes=%d, want 1", computes.Load())
	}
}

func TestRuntimeMutationMakesOldSnapshotUnreachableAndRejectsLostLease(t *testing.T) {
	cache := isolatedRuntimeCache(t, openSMSRuntimeRedis(t), fmt.Sprintf(":test:%d", time.Now().UnixNano()))
	var loads atomic.Int32
	loader := func(context.Context) (RuntimeFacts, error) {
		loads.Add(1)
		return runtimeFactsForCacheTest(), nil
	}
	if _, err := cache.Load(context.Background(), loader); err != nil {
		t.Fatal(err)
	}
	if err := cache.Mutate(context.Background(), func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Load(context.Background(), loader); err != nil {
		t.Fatal(err)
	}
	if loads.Load() != 2 {
		t.Fatalf("loads after mutation=%d, want 2", loads.Load())
	}
	if err := cache.Mutate(context.Background(), func(ctx context.Context) error {
		return cache.redis.Delete(ctx, cache.mutationKey())
	}); err == nil {
		t.Fatal("mutation published after losing its tokenized lease")
	}
}

func TestRuntimeCacheFailsClosedOnCorruptSnapshotAndRedisFailure(t *testing.T) {
	cache := isolatedRuntimeCache(t, openSMSRuntimeRedis(t), fmt.Sprintf(":test:%d", time.Now().UnixNano()))
	if _, err := cache.currentGeneration(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := cache.redis.SetString(context.Background(), cache.snapshotKey("1"), `{"schemaVersion":1,"generation":"1","configured":true}`, time.Minute); err != nil {
		t.Fatal(err)
	}
	var loads atomic.Int32
	loader := func(context.Context) (RuntimeFacts, error) {
		loads.Add(1)
		return runtimeFactsForCacheTest(), nil
	}
	if _, err := cache.Load(context.Background(), loader); err == nil {
		t.Fatal("corrupt runtime snapshot was accepted")
	}
	if loads.Load() != 0 {
		t.Fatalf("corrupt snapshot fell back to PostgreSQL %d times", loads.Load())
	}
	if err := cache.redis.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Load(context.Background(), loader); err == nil {
		t.Fatal("closed Redis client was accepted")
	}
	if loads.Load() != 0 {
		t.Fatalf("Redis failure fell back to PostgreSQL %d times", loads.Load())
	}
}
