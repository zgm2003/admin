package permission

import (
	permissionstate "admin/server/internal/module/permission/state"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type concurrentSource struct{ calls atomic.Int32 }

func (s *concurrentSource) FindMenuVersion(context.Context, int64) (int64, error) { return 1, nil }
func (s *concurrentSource) FindSourceWithVersion(ctx context.Context, _, _ int64) (Source, error) {
	s.calls.Add(1)
	timer := time.NewTimer(50 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return Source{}, ctx.Err()
	case <-timer.C:
		return baseSource(3), nil
	}
}

func TestTwoInstancesCollapseConcurrentAccessMiss(t *testing.T) {
	a, b := openAccessRedis(t), openAccessRedis(t)
	id := time.Now().UnixNano() / 1000
	cleanupAccessKeys(t, a, id, "admin", 4, 3)
	source := &concurrentSource{}
	services := []*Service{newAccessTestService(a, source), newAccessTestService(b, source)}
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			v, err := services[i%2].Current(context.Background(), accessIdentity(id))
			if err != nil || v.Version != 3 {
				t.Errorf("snapshot=%+v err=%v", v, err)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	if source.calls.Load() != 1 {
		t.Fatalf("cold source calls=%d", source.calls.Load())
	}
}

func TestMenuRevisionInvalidatesWarmCacheWithoutUserVersionChange(t *testing.T) {
	r := openAccessRedis(t)
	id := time.Now().UnixNano() / 1000
	ctx := context.Background()
	cleanupAccessKeys(t, r, id, "admin", 4, 3)
	source := &countingSourceStore{sources: []Source{baseSource(3), baseSource(3)}}
	service := newAccessTestService(r, source)
	if _, err := service.Current(ctx, accessIdentity(id)); err != nil {
		t.Fatal(err)
	}
	if err := r.SetString(ctx, permissionstate.MenuStateKey(1), `{"schemaVersion":2,"state":"invalidating","baseVersion":1,"mutationToken":"test"}`, time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Current(ctx, accessIdentity(id)); err == nil {
		t.Fatal("served stale local cache during catalog mutation")
	}
	if source.calls != 1 {
		t.Fatal("invalidating catalog queried source")
	}
	if err := r.SetString(ctx, permissionstate.MenuStateKey(1), `{"schemaVersion":2,"state":"ready","version":2}`, 0); err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.Current(ctx, accessIdentity(id))
	if err != nil || source.calls != 2 || snapshot.Version != 3 {
		t.Fatalf("new catalog source=%d version=%d err=%v", source.calls, snapshot.Version, err)
	}
	key := SnapshotKey("admin", 4, id, 3, 2)
	t.Cleanup(func() { _ = r.Delete(ctx, key) })
	if raw, found, err := r.GetString(ctx, key); err != nil || !found || raw == "" {
		t.Fatal(fmt.Sprintf("new catalog snapshot missing: %v", err))
	}
}
