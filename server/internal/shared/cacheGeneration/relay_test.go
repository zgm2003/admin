package cachegeneration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	projectredis "admin/server/internal/redis"
)

type relayRescheduleRecord struct {
	id          int64
	token       string
	safeError   string
	availableAt time.Time
	now         time.Time
}

type fakeRelayRepository struct {
	mutex        sync.Mutex
	current      int64
	currentErr   error
	claimErr     error
	publishErr   error
	events       []Event
	claimed      bool
	publishedIDs []int64
	reschedules  []relayRescheduleRecord
}

func (f *fakeRelayRepository) Current(context.Context, Scope) (int64, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	if f.currentErr != nil {
		return 0, f.currentErr
	}
	return f.current, nil
}

func (f *fakeRelayRepository) ClaimPending(_ context.Context, limit int, token string, _ time.Time, _ time.Duration) ([]Event, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	if f.claimed {
		return nil, nil
	}
	f.claimed = true
	count := min(limit, len(f.events))
	claimed := make([]Event, 0, count)
	for _, event := range f.events[:count] {
		claimed = append(claimed, Event{
			ID: event.ID, Scope: event.Scope, Generation: event.Generation, Attempts: event.Attempts + 1, LockToken: token,
		})
	}
	return claimed, nil
}

func (f *fakeRelayRepository) MarkPublished(_ context.Context, id int64, _ string, _ time.Time) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	if f.publishErr != nil {
		return f.publishErr
	}
	f.publishedIDs = append(f.publishedIDs, id)
	return nil
}

func (f *fakeRelayRepository) Reschedule(_ context.Context, id int64, token, safeError string, availableAt, now time.Time) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.reschedules = append(f.reschedules, relayRescheduleRecord{
		id: id, token: token, safeError: safeError, availableAt: availableAt, now: now,
	})
	return nil
}

func (f *fakeRelayRepository) snapshot() ([]int64, []relayRescheduleRecord) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return append([]int64{}, f.publishedIDs...), append([]relayRescheduleRecord{}, f.reschedules...)
}

type fakeRelayStore struct {
	mutex  sync.Mutex
	calls  []int64
	result PublishResult
	err    error
}

func (f *fakeRelayStore) Reconcile(_ context.Context, _ Scope, authoritative int64) (PublishResult, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.calls = append(f.calls, authoritative)
	if f.err != nil {
		return 0, f.err
	}
	return f.result, nil
}

func (f *fakeRelayStore) snapshot() []int64 {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return append([]int64{}, f.calls...)
}

func TestRelayRunOncePublishesLatestAuthoritativeGeneration(t *testing.T) {
	scope := mustGenerationScope(t)
	repository := &fakeRelayRepository{current: 7, events: []Event{{ID: 3, Scope: scope, Generation: 5}}}
	store := &fakeRelayStore{result: PublishPublished}
	relay := NewRelay(repository, store, nil)

	published, err := relay.RunOnce(context.Background())
	if err != nil || published != 1 {
		t.Fatalf("RunOnce = %d,%v want 1,nil", published, err)
	}
	if calls := store.snapshot(); len(calls) != 1 || calls[0] != 7 {
		t.Fatalf("reconcile calls = %v want [7]", calls)
	}
	publishedIDs, reschedules := repository.snapshot()
	if len(publishedIDs) != 1 || publishedIDs[0] != 3 || len(reschedules) != 0 {
		t.Fatalf("published=%v reschedules=%v", publishedIDs, reschedules)
	}
}

func TestRelayRunOnceMarksAlreadyNewerEventsPublished(t *testing.T) {
	scope := mustGenerationScope(t)
	repository := &fakeRelayRepository{current: 7, events: []Event{{ID: 4, Scope: scope, Generation: 6}}}
	store := &fakeRelayStore{result: PublishAlreadyNewer}
	relay := NewRelay(repository, store, nil)

	published, err := relay.RunOnce(context.Background())
	if err != nil || published != 1 {
		t.Fatalf("RunOnce = %d,%v want 1,nil", published, err)
	}
	publishedIDs, _ := repository.snapshot()
	if len(publishedIDs) != 1 || publishedIDs[0] != 4 {
		t.Fatalf("published = %v want [4]", publishedIDs)
	}
}

func TestRelayRunOnceReschedulesOnDependencyFailureWithSanitizedError(t *testing.T) {
	scope := mustGenerationScope(t)
	repository := &fakeRelayRepository{current: 7, events: []Event{{ID: 5, Scope: scope, Generation: 4, Attempts: 2}}}
	store := &fakeRelayStore{err: fmt.Errorf("dial redis://cache-user:cache-secret@127.0.0.1:6379: token=abc password=hunter2")}
	relay := NewRelay(repository, store, nil)

	published, err := relay.RunOnce(context.Background())
	if err != nil || published != 0 {
		t.Fatalf("RunOnce = %d,%v want 0,nil", published, err)
	}
	_, reschedules := repository.snapshot()
	if len(reschedules) != 1 {
		t.Fatalf("reschedules = %v want one record", reschedules)
	}
	record := reschedules[0]
	if record.id != 5 {
		t.Fatalf("rescheduled id = %d want 5", record.id)
	}
	if !strings.Contains(record.safeError, "dependency-unavailable") {
		t.Fatalf("safe error = %q want error class", record.safeError)
	}
	for _, forbidden := range []string{"redis://", "cache-secret", "token=abc", "password=hunter2", "127.0.0.1"} {
		if strings.Contains(record.safeError, forbidden) {
			t.Fatalf("safe error %q leaks %q", record.safeError, forbidden)
		}
	}
	if len([]rune(record.safeError)) > 512 {
		t.Fatalf("safe error length = %d want <=512", len([]rune(record.safeError)))
	}
	if !record.availableAt.After(record.now) {
		t.Fatalf("availableAt %v must be after now %v", record.availableAt, record.now)
	}
}

func TestRelayTwoInstancesDoNotPublishTheSameEvent(t *testing.T) {
	scope := mustGenerationScope(t)
	repository := &fakeRelayRepository{current: 7, events: []Event{{ID: 9, Scope: scope, Generation: 3}}}
	store := &fakeRelayStore{result: PublishPublished}

	first, err := NewRelay(repository, store, nil).RunOnce(context.Background())
	if err != nil || first != 1 {
		t.Fatalf("first relay = %d,%v want 1,nil", first, err)
	}
	second, err := NewRelay(repository, store, nil).RunOnce(context.Background())
	if err != nil || second != 0 {
		t.Fatalf("second relay = %d,%v want 0,nil", second, err)
	}
	publishedIDs, _ := repository.snapshot()
	if len(publishedIDs) != 1 || publishedIDs[0] != 9 {
		t.Fatalf("published = %v want exactly [9]", publishedIDs)
	}
}

func TestRelayRunOnceReturnsClaimFailure(t *testing.T) {
	repository := &fakeRelayRepository{claimErr: errors.New("claim unavailable")}
	relay := NewRelay(repository, &fakeRelayStore{}, nil)

	if _, err := relay.RunOnce(context.Background()); err == nil {
		t.Fatal("RunOnce accepted an unavailable outbox")
	}
}

func TestRelayRunReturnsOnContextCancel(t *testing.T) {
	repository := &fakeRelayRepository{current: 1}
	relay := NewRelay(repository, &fakeRelayStore{result: PublishPublished}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- relay.Run(ctx) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned %v want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}

func TestRelayRunRejectsMissingDependencies(t *testing.T) {
	relay := NewRelay(nil, nil, nil)
	if err := relay.Run(context.Background()); err == nil {
		t.Fatal("Run accepted missing dependencies")
	}
	if _, err := relay.RunOnce(context.Background()); err == nil {
		t.Fatal("RunOnce accepted missing dependencies")
	}
}

func TestRelayRunOnceRecoversExpiredClaimWithoutDowngradingState(t *testing.T) {
	db, ctx := openGenerationDatabase(t)
	repository := NewRepository(db)
	client := openGenerationRedis(t)
	store := NewStore(client)
	scope := mustGenerationScopeWithSuffix(t, "relay-recovery", client)
	t.Cleanup(func() {
		_ = client.Delete(context.Background(), StateKey(scope))
	})

	if err := db.WithContext(ctx).Exec(
		`INSERT INTO system_config_cache_generation (namespace, scope_key, generation, created_at, updated_at)
		 VALUES (?, ?, 2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		scope.Namespace, scope.ScopeKey).Error; err != nil {
		t.Fatal(err)
	}
	var outboxID int64
	if err := db.WithContext(ctx).Raw(
		`INSERT INTO system_config_cache_outbox (namespace, scope_key, generation, attempts, available_at, created_at, updated_at)
		 VALUES (?, ?, 2, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) RETURNING id`,
		scope.Namespace, scope.ScopeKey).Scan(&outboxID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.Reconcile(ctx, scope, 3); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	claimed, err := repository.ClaimPending(ctx, 10, "crashed-worker", now, 100*time.Millisecond)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim = %d,%v want 1", len(claimed), err)
	}

	relay := NewRelay(repository, store, nil)
	relay.now = func() time.Time { return now.Add(2 * time.Second) }
	published, err := relay.RunOnce(ctx)
	if err != nil || published != 1 {
		t.Fatalf("relay published = %d,%v want 1", published, err)
	}

	var row struct {
		Attempts    int
		PublishedAt *time.Time
		LastError   string
	}
	if err := db.WithContext(ctx).Raw(
		`SELECT attempts, published_at, last_error FROM system_config_cache_outbox WHERE id = ?`, outboxID).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.PublishedAt == nil || row.Attempts != 2 {
		t.Fatalf("outbox row = %+v want published after two attempts", row)
	}
	state, found, err := store.Read(ctx, scope)
	if err != nil || !found || state.State != StateReady {
		t.Fatalf("state = %+v found=%v err=%v", state, found, err)
	}
	if state.Generation != 3 {
		t.Fatalf("relay downgraded ready generation to %d", state.Generation)
	}
}

func TestRelayTwoInstancesPublishSingleEventOnce(t *testing.T) {
	db, ctx := openGenerationDatabase(t)
	repository := NewRepository(db)
	client := openGenerationRedis(t)
	store := NewStore(client)
	scope := mustGenerationScopeWithSuffix(t, "relay-skip-locked", client)
	t.Cleanup(func() {
		_ = client.Delete(context.Background(), StateKey(scope))
	})

	if err := db.WithContext(ctx).Exec(
		`INSERT INTO system_config_cache_generation (namespace, scope_key, generation, created_at, updated_at)
		 VALUES (?, ?, 2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		scope.Namespace, scope.ScopeKey).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(
		`INSERT INTO system_config_cache_outbox (namespace, scope_key, generation, attempts, available_at, created_at, updated_at)
		 VALUES (?, ?, 2, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		scope.Namespace, scope.ScopeKey).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.Reconcile(ctx, scope, 1); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	var mutex sync.Mutex
	completed := 0
	var waitGroup sync.WaitGroup
	for index := 0; index < 2; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			relay := NewRelay(repository, store, nil)
			relay.now = func() time.Time { return now }
			published, err := relay.RunOnce(ctx)
			if err != nil {
				t.Errorf("relay RunOnce: %v", err)
				return
			}
			mutex.Lock()
			completed += published
			mutex.Unlock()
		}()
	}
	waitGroup.Wait()

	if completed != 1 {
		t.Fatalf("published events = %d want 1", completed)
	}
	var row struct {
		Attempts    int
		PublishedAt *time.Time
	}
	if err := db.WithContext(ctx).Raw(
		`SELECT attempts, published_at FROM system_config_cache_outbox WHERE namespace = ? AND scope_key = ?`,
		scope.Namespace, scope.ScopeKey).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.PublishedAt == nil || row.Attempts != 1 {
		t.Fatalf("outbox row = %+v want single claim and publish", row)
	}
	state, found, err := store.Read(ctx, scope)
	if err != nil || !found || state.State != StateReady || state.Generation != 2 {
		t.Fatalf("state = %+v found=%v err=%v", state, found, err)
	}
}

func mustGenerationScopeWithSuffix(t *testing.T, prefix string, client *projectredis.Client) Scope {
	t.Helper()
	scope, err := NewScope("system.setting", prefix+"-"+time.Now().UTC().Format("20060102150405.000000000"))
	if err != nil {
		t.Fatal(err)
	}
	if client != nil {
		if err := client.Delete(context.Background(), StateKey(scope)); err != nil {
			t.Fatal(err)
		}
	}
	return scope
}

func mustGenerationScope(t *testing.T) Scope {
	t.Helper()
	scope, err := NewScope("system.setting", "global")
	if err != nil {
		t.Fatal(err)
	}
	return scope
}
