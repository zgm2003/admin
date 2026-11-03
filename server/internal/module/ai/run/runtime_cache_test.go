package run

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	cache "admin/server/internal/shared/configCache"
	"github.com/google/uuid"
)

type runtimeSourceFixture struct {
	mu     sync.Mutex
	calls  int
	prompt string
	next   *time.Time
}

func (f *runtimeSourceFixture) LoadRuntime(ctx context.Context, id int64, at time.Time) (RuntimeSnapshot, *time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	s := fixtureSnapshot()
	s.AgentID = id
	s.AcceptedAt = at
	s.Prompt = f.prompt
	return s, f.next, nil
}

type runtimeGenerationsFixture struct {
	mu         sync.Mutex
	generation int64
	reads      int
}

func (f *runtimeGenerationsFixture) Current(context.Context, cache.Scope) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	return f.generation, nil
}
func (f *runtimeGenerationsFixture) MarkPublishedIfUnclaimed(context.Context, int64, time.Time) (bool, error) {
	return true, nil
}
func TestRuntimeRedisSnapshotTwoReadersHotPathAndGenerationMutation(t *testing.T) {
	a, b := openGateRedis(t), openGateRedis(t)
	scope := cache.Scope{Namespace: "test.ai.runtime", ScopeKey: uuid.NewString()}
	ctx := context.Background()
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = a.ScanDelete(cleanup, "config-cache:*:"+scope.Namespace+":"+scope.ScopeKey+"*")
	})
	source := &runtimeSourceFixture{prompt: "old"}
	gen := &runtimeGenerationsFixture{generation: 1}
	first := NewRuntime(source, a, gen, cache.NewStore(a))
	second := NewRuntime(source, b, gen, cache.NewStore(b))
	first.scope = scope
	second.scope = scope
	old, err := first.Get(ctx, 1, time.Now())
	if err != nil || old.Prompt != "old" {
		t.Fatalf("first=%+v err=%v", old, err)
	}
	if _, err = second.Get(ctx, 1, time.Now()); err != nil {
		t.Fatal(err)
	}
	source.mu.Lock()
	calls := source.calls
	source.mu.Unlock()
	if calls != 1 {
		t.Fatalf("hot path source calls=%d", calls)
	}
	err = first.Mutate(ctx, func(ctx context.Context, base int64) (cache.MutationResult, error) {
		source.mu.Lock()
		source.prompt = "new"
		source.mu.Unlock()
		gen.mu.Lock()
		gen.generation = base + 1
		gen.mu.Unlock()
		return cache.MutationResult{Changed: true, Generation: base + 1, OutboxID: 2}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	current, err := second.Get(ctx, 1, time.Now())
	if err != nil || current.Prompt != "new" || old.Prompt != "old" {
		t.Fatalf("new=%+v old=%+v err=%v", current, old, err)
	}
	if err := a.SetString(ctx, cache.StateKey(scope), "broken", time.Minute); err != nil {
		t.Fatal(err)
	}
	source.mu.Lock()
	before := source.calls
	source.mu.Unlock()
	if _, err := second.Get(ctx, 1, time.Now()); err == nil {
		t.Fatal("corrupt state returned old snapshot")
	}
	source.mu.Lock()
	after := source.calls
	source.mu.Unlock()
	if before != after {
		t.Fatalf("corrupt state fell through to DB: %d -> %d", before, after)
	}
}
func TestRuntimeDependenciesFailClosed(t *testing.T) {
	if _, err := NewRuntime(nil, nil, nil, nil).Get(context.Background(), 1, time.Now()); err == nil {
		t.Fatal("missing dependencies accepted")
	}
	if err := NewRuntime(nil, nil, nil, nil).Mutate(context.Background(), nil); err == nil {
		t.Fatal("missing mutation dependencies accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewRuntime(nil, nil, nil, nil).Get(ctx, 1, time.Now()); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled=%v", err)
	}
}
