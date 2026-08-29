package authstate

import (
	"context"
	"errors"
	"testing"
)

func TestMutationLeaseCommitsAndRollsBackExactFacts(t *testing.T) {
	client := openAuthenticationStateRedis(t)
	store := NewStore(client)
	invalidator := NewInvalidator(store)
	ctx := context.Background()
	keys := []string{UserStateKey(72001), SessionsStateKey("admin", 72001)}
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), keys) })
	_ = client.DeleteMany(ctx, keys)

	prior := MutationFacts{
		Users:    []UserFact{{UserID: 72001, Generation: "user-old", IsEnabled: true}},
		Sessions: []SessionsFact{{Platform: "admin", UserID: 72001, Generation: "sessions-old"}},
	}
	if _, _, err := store.InstallUserReadyIfMissing(ctx, prior.Users[0]); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.InstallSessionsReadyIfMissing(ctx, prior.Sessions[0]); err != nil {
		t.Fatal(err)
	}
	lease, err := invalidator.Acquire(ctx, prior)
	if err != nil {
		t.Fatal(err)
	}
	if state, _, err := store.ReadUser(ctx, 72001); err != nil || state.State != StateInvalidating {
		t.Fatalf("invalidating user = %+v,%v", state, err)
	}
	next := MutationFacts{
		Users:    []UserFact{{UserID: 72001, Generation: "user-new", IsEnabled: false}},
		Sessions: []SessionsFact{{Platform: "admin", UserID: 72001, Generation: "sessions-new"}},
	}
	if err := lease.Commit(ctx, next); err != nil {
		t.Fatal(err)
	}
	if state, _, err := store.ReadUser(ctx, 72001); err != nil || state.State != StateReady || state.Generation != "user-new" || state.IsEnabled {
		t.Fatalf("committed user = %+v,%v", state, err)
	}

	lease, err = invalidator.Acquire(ctx, next)
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if state, _, err := store.ReadSessions(ctx, "admin", 72001); err != nil || state.State != StateReady || state.Generation != "sessions-new" {
		t.Fatalf("rolled back sessions = %+v,%v", state, err)
	}
}

func TestMutationAcquireRestoresEarlierKeysWhenCandidateChanged(t *testing.T) {
	client := openAuthenticationStateRedis(t)
	store := NewStore(client)
	invalidator := NewInvalidator(store)
	ctx := context.Background()
	keys := []string{UserStateKey(72002), SessionsStateKey("admin", 72002)}
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), keys) })
	_ = client.DeleteMany(ctx, keys)

	userFact := UserFact{UserID: 72002, Generation: "user-current", IsEnabled: true}
	sessionsFact := SessionsFact{Platform: "admin", UserID: 72002, Generation: "sessions-current"}
	_, _, _ = store.InstallUserReadyIfMissing(ctx, userFact)
	_, _, _ = store.InstallSessionsReadyIfMissing(ctx, sessionsFact)

	_, err := invalidator.Acquire(ctx, MutationFacts{
		Users:    []UserFact{userFact},
		Sessions: []SessionsFact{{Platform: "admin", UserID: 72002, Generation: "sessions-stale"}},
	})
	if !errors.Is(err, ErrGenerationChanged) {
		t.Fatalf("Acquire() error = %v", err)
	}
	state, found, readErr := store.ReadUser(ctx, 72002)
	if readErr != nil || !found || state.State != StateReady || state.Generation != userFact.Generation {
		t.Fatalf("restored user = %+v,%v,%v", state, found, readErr)
	}
}
