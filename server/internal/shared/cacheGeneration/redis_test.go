package cachegeneration

import (
	"context"
	"errors"
	"testing"
	"time"

	projectredis "admin/server/internal/redis"
)

func TestMutationLeaseTwoStoresSingleWinner(t *testing.T) {
	ctx := context.Background()
	clientA := openGenerationRedis(t)
	clientB := openGenerationRedis(t)
	scope := newGenerationTestScope(t)
	storeA, storeB := NewStore(clientA), NewStore(clientB)
	t.Cleanup(func() { _ = clientA.Delete(context.Background(), StateKey(scope)) })

	if _, err := storeA.Reconcile(ctx, scope, 1); err != nil {
		t.Fatal(err)
	}
	lease, err := storeA.Acquire(ctx, scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storeB.Acquire(ctx, scope, 1); !errors.Is(err, ErrUpdating) {
		t.Fatalf("second acquire error = %v, want ErrUpdating", err)
	}
	if err := lease.Commit(ctx, 2); err != nil {
		t.Fatal(err)
	}
	state, found, err := storeB.Read(ctx, scope)
	if err != nil || !found || state.State != StateReady || state.Generation != 2 {
		t.Fatalf("state=%+v found=%v err=%v", state, found, err)
	}

	next, err := storeB.Acquire(ctx, scope, 2)
	if err != nil {
		t.Fatalf("successor acquire error = %v", err)
	}
	if err := next.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	state, found, err = storeA.Read(ctx, scope)
	if err != nil || !found || state.State != StateReady || state.Generation != 2 {
		t.Fatalf("state after rollback=%+v found=%v err=%v", state, found, err)
	}
}

func TestMutationLeaseRejectsWrongTokenOperations(t *testing.T) {
	ctx := context.Background()
	client := openGenerationRedis(t)
	scope := newGenerationTestScope(t)
	store := NewStore(client)
	t.Cleanup(func() { _ = client.Delete(context.Background(), StateKey(scope)) })

	if _, err := store.Reconcile(ctx, scope, 1); err != nil {
		t.Fatal(err)
	}
	lease, err := store.Acquire(ctx, scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := encodeState(State{
		SchemaVersion: SchemaVersion, State: StateInvalidating, BaseGeneration: 1, MutationToken: "foreign-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SetString(ctx, StateKey(scope), foreign, MutationLeaseTTL); err != nil {
		t.Fatal(err)
	}

	if err := lease.Renew(ctx); !errors.Is(err, ErrMutationTokenMismatch) {
		t.Fatalf("renew with foreign token error = %v", err)
	}
	if err := lease.Commit(ctx, 2); !errors.Is(err, ErrMutationTokenMismatch) {
		t.Fatalf("commit with foreign token error = %v", err)
	}
	if err := lease.Rollback(ctx); !errors.Is(err, ErrMutationTokenMismatch) {
		t.Fatalf("rollback with foreign token error = %v", err)
	}
}

func TestMutationLeaseRejectsStalePublisherAndAuthorityRepairsExpiredState(t *testing.T) {
	ctx := context.Background()
	client := openGenerationRedis(t)
	scope := newGenerationTestScope(t)
	store := NewStore(client)
	t.Cleanup(func() { _ = client.Delete(context.Background(), StateKey(scope)) })

	if _, err := store.Reconcile(ctx, scope, 1); err != nil {
		t.Fatal(err)
	}
	lease, err := store.Acquire(ctx, scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Delete(ctx, StateKey(scope)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Reconcile(ctx, scope, 2); err != nil {
		t.Fatal(err)
	}
	if err := lease.Commit(ctx, 2); !errors.Is(err, ErrMutationTokenMismatch) {
		t.Fatalf("stale mutation lease commit error = %v", err)
	}
	if err := lease.Renew(ctx); !errors.Is(err, ErrMutationTokenMismatch) {
		t.Fatalf("stale mutation lease renew error = %v", err)
	}
	if err := lease.Rollback(ctx); !errors.Is(err, ErrMutationTokenMismatch) {
		t.Fatalf("stale mutation lease rollback error = %v", err)
	}
	state, found, err := store.Read(ctx, scope)
	if err != nil || !found || state.State != StateReady || state.Generation != 2 {
		t.Fatalf("state=%+v found=%v err=%v", state, found, err)
	}
}

func TestReconcileRejectsReadyDowngrade(t *testing.T) {
	ctx := context.Background()
	client := openGenerationRedis(t)
	scope := newGenerationTestScope(t)
	store := NewStore(client)
	t.Cleanup(func() { _ = client.Delete(context.Background(), StateKey(scope)) })

	result, err := store.Reconcile(ctx, scope, 12)
	if err != nil || result != PublishRepaired {
		t.Fatalf("initial reconcile = %v,%v want repaired", result, err)
	}
	result, err = store.Reconcile(ctx, scope, 11)
	if err != nil || result != PublishAlreadyNewer {
		t.Fatalf("downgrade reconcile = %v,%v want already-newer", result, err)
	}
	state, found, err := store.Read(ctx, scope)
	if err != nil || !found || state.State != StateReady || state.Generation != 12 {
		t.Fatalf("state after downgrade=%+v found=%v err=%v", state, found, err)
	}
}

func TestReconcileKeepsActiveMutationUntouched(t *testing.T) {
	ctx := context.Background()
	client := openGenerationRedis(t)
	scope := newGenerationTestScope(t)
	store := NewStore(client)
	t.Cleanup(func() { _ = client.Delete(context.Background(), StateKey(scope)) })

	if _, err := store.Reconcile(ctx, scope, 12); err != nil {
		t.Fatal(err)
	}
	lease, err := store.Acquire(ctx, scope, 12)
	if err != nil {
		t.Fatal(err)
	}
	for _, authoritative := range []int64{11, 12, 13} {
		result, err := store.Reconcile(ctx, scope, authoritative)
		if err != nil || result != PublishSkippedInvalidating {
			t.Fatalf("reconcile(%d) = %v,%v want invalidating", authoritative, result, err)
		}
	}
	state, found, err := store.Read(ctx, scope)
	if err != nil || !found || state.State != StateInvalidating || state.BaseGeneration != 12 || state.MutationToken == "" {
		t.Fatalf("active mutation state=%+v found=%v err=%v", state, found, err)
	}
	if err := lease.Rollback(ctx); err != nil {
		t.Fatalf("lease lost ownership to reconcile: %v", err)
	}
	state, found, err = store.Read(ctx, scope)
	if err != nil || !found || state.State != StateReady || state.Generation != 12 {
		t.Fatalf("state after rollback=%+v found=%v err=%v", state, found, err)
	}
}

func TestReconcileRepairsMissingAndCorruptStates(t *testing.T) {
	ctx := context.Background()
	client := openGenerationRedis(t)
	scope := newGenerationTestScope(t)
	store := NewStore(client)
	t.Cleanup(func() { _ = client.Delete(context.Background(), StateKey(scope)) })

	result, err := store.Reconcile(ctx, scope, 12)
	if err != nil || result != PublishRepaired {
		t.Fatalf("missing state reconcile = %v,%v want repaired", result, err)
	}
	if err := client.SetString(ctx, StateKey(scope), "not-json", 0); err != nil {
		t.Fatal(err)
	}
	if _, found, err := store.Read(ctx, scope); !found || !errors.Is(err, ErrStateCorrupt) {
		t.Fatalf("corrupt read found=%v err=%v want ErrStateCorrupt", found, err)
	}
	result, err = store.Reconcile(ctx, scope, 12)
	if err != nil || result != PublishRepaired {
		t.Fatalf("corrupt state reconcile = %v,%v want repaired", result, err)
	}
	state, found, err := store.Read(ctx, scope)
	if err != nil || !found || state.State != StateReady || state.Generation != 12 {
		t.Fatalf("state after repair=%+v found=%v err=%v", state, found, err)
	}
}

func TestMutationLeaseAcquireRequiresReconcileOnMissingOrCorruptState(t *testing.T) {
	ctx := context.Background()
	client := openGenerationRedis(t)
	scope := newGenerationTestScope(t)
	store := NewStore(client)
	t.Cleanup(func() { _ = client.Delete(context.Background(), StateKey(scope)) })

	if _, err := store.Acquire(ctx, scope, 1); !errors.Is(err, ErrStateMissing) {
		t.Fatalf("acquire on missing state error = %v want ErrStateMissing", err)
	}
	if err := client.SetString(ctx, StateKey(scope), "{", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Acquire(ctx, scope, 1); !errors.Is(err, ErrStateCorrupt) {
		t.Fatalf("acquire on corrupt state error = %v want ErrStateCorrupt", err)
	}
}

func newGenerationTestScope(t *testing.T) Scope {
	t.Helper()
	scope, err := NewScope("system.setting", "test-"+time.Now().UTC().Format("20060102150405.000000000"))
	if err != nil {
		t.Fatal(err)
	}
	return scope
}

func openGenerationRedis(t *testing.T) *projectredis.Client {
	t.Helper()
	settings := loadIntegrationSettings(t)
	client, err := projectredis.Open(context.Background(), settings.RedisURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}
