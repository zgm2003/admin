package accessstate

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestMutationLeaseAcquiresCommitsAndRollsBackSortedUsers(t *testing.T) {
	client := openAccessStateRedis(t)
	store := NewStore(client)
	invalidator := NewInvalidator(store)
	ctx := context.Background()
	keys := []string{StateKey(92007), StateKey(92009)}
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), keys) })
	_ = client.DeleteMany(ctx, keys)

	lease, err := invalidator.Acquire(ctx, []Version{{UserID: 92009, Version: 4}, {UserID: 92007, Version: 3}, {UserID: 92009, Version: 4}})
	if err != nil {
		t.Fatal(err)
	}
	if got := lease.UserIDs(); !reflect.DeepEqual(got, []int64{92007, 92009}) {
		t.Fatalf("lease user IDs = %v", got)
	}
	for _, userID := range lease.UserIDs() {
		state, found, readErr := store.Read(ctx, userID)
		if readErr != nil || !found || state.State != StateInvalidating {
			t.Fatalf("invalidating state %d = %+v,%v,%v", userID, state, found, readErr)
		}
	}
	if err := lease.Commit(ctx, map[int64]int64{92007: 4, 92009: 5}); err != nil {
		t.Fatal(err)
	}
	for userID, version := range map[int64]int64{92007: 4, 92009: 5} {
		state, _, readErr := store.Read(ctx, userID)
		if readErr != nil || state.State != StateReady || state.Version != version {
			t.Fatalf("committed state %d = %+v,%v", userID, state, readErr)
		}
	}

	lease, err = invalidator.Acquire(ctx, []Version{{UserID: 92007, Version: 4}, {UserID: 92009, Version: 5}})
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if state, _, err := store.Read(ctx, 92007); err != nil || state.State != StateReady || state.Version != 4 {
		t.Fatalf("rolled back state = %+v,%v", state, err)
	}
}

func TestAcquireRestoresEarlierUsersWhenLaterVersionChanged(t *testing.T) {
	client := openAccessStateRedis(t)
	store := NewStore(client)
	invalidator := NewInvalidator(store)
	ctx := context.Background()
	keys := []string{StateKey(92101), StateKey(92102)}
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), keys) })
	_ = client.DeleteMany(ctx, keys)
	_, _, _ = store.InstallReadyIfMissing(ctx, Version{UserID: 92101, Version: 1})
	_, _, _ = store.InstallReadyIfMissing(ctx, Version{UserID: 92102, Version: 4})

	_, err := invalidator.Acquire(ctx, []Version{{UserID: 92101, Version: 1}, {UserID: 92102, Version: 3}})
	if !errors.Is(err, ErrVersionChanged) {
		t.Fatalf("Acquire() error = %v", err)
	}
	state, found, readErr := store.Read(ctx, 92101)
	if readErr != nil || !found || state.State != StateReady || state.Version != 1 {
		t.Fatalf("restored first user = %+v,%v,%v", state, found, readErr)
	}
}

func TestMutationTokenMismatchNeverOverwritesAnotherWriter(t *testing.T) {
	client := openAccessStateRedis(t)
	store := NewStore(client)
	ctx := context.Background()
	key := StateKey(92201)
	t.Cleanup(func() { _ = client.Delete(context.Background(), key) })
	_ = client.Delete(ctx, key)
	lease, err := NewInvalidator(store).Acquire(ctx, []Version{{UserID: 92201, Version: 1}})
	if err != nil {
		t.Fatal(err)
	}
	other := `{"schemaVersion":2,"state":"invalidating","version":0,"mutationToken":"other-writer","baseVersion":1}`
	if err := client.SetString(ctx, key, other, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := lease.Commit(ctx, map[int64]int64{92201: 2}); !errors.Is(err, ErrMutationTokenMismatch) {
		t.Fatalf("Commit() error = %v", err)
	}
	raw, found, err := client.GetString(ctx, key)
	if err != nil || !found || raw != other {
		t.Fatalf("other writer state = %q,%v,%v", raw, found, err)
	}
}

func TestRenewalFailureCancelsTransactionContext(t *testing.T) {
	client := openAccessStateRedis(t)
	store := NewStore(client)
	invalidator := NewInvalidator(store)
	invalidator.leaseTTL = time.Second
	invalidator.renewEvery = 10 * time.Millisecond
	ctx := context.Background()
	key := StateKey(92301)
	t.Cleanup(func() { _ = client.Delete(context.Background(), key) })
	_ = client.Delete(ctx, key)
	lease, err := invalidator.Acquire(ctx, []Version{{UserID: 92301, Version: 1}})
	if err != nil {
		t.Fatal(err)
	}
	transactionCtx, stop := lease.StartRenewal(ctx)
	defer stop()
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-transactionCtx.Done():
		if context.Cause(transactionCtx) == nil {
			t.Fatal("renewal cancellation has no cause")
		}
	case <-time.After(time.Second):
		t.Fatal("renewal failure did not cancel transaction context")
	}
}
