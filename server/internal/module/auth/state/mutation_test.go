package authstate

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type receivedInvalidation struct {
	SchemaVersion int    `json:"schemaVersion"`
	TargetType    string `json:"targetType"`
	PlatformCode  string `json:"platformCode,omitempty"`
	UserID        int64  `json:"userId,omitempty"`
}

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

func TestAcquirePublishesRealtimeInvalidationInSameRedisCommand(t *testing.T) {
	client := openAuthenticationStateRedis(t)
	store := NewStore(client)
	invalidator := NewInvalidator(store)
	ctx := context.Background()
	keys := []string{UserStateKey(72003), SessionsStateKey("admin", 72003)}
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), keys) })
	_ = client.DeleteMany(ctx, keys)

	pubsub := client.UniversalClient().Subscribe(ctx, realtimeInvalidationChannel)
	t.Cleanup(func() { _ = pubsub.Close() })
	if _, err := pubsub.Receive(ctx); err != nil {
		t.Fatal(err)
	}

	userFact := UserFact{UserID: 72003, Generation: "user-current", IsEnabled: true}
	sessionsFact := SessionsFact{Platform: "admin", UserID: 72003, Generation: "sessions-current"}
	if _, _, err := store.InstallUserReadyIfMissing(ctx, userFact); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.InstallSessionsReadyIfMissing(ctx, sessionsFact); err != nil {
		t.Fatal(err)
	}

	lease, err := invalidator.Acquire(ctx, MutationFacts{Users: []UserFact{userFact}})
	if err != nil {
		t.Fatal(err)
	}
	if got := receiveRealtimeInvalidation(t, pubsub.Channel()); got.SchemaVersion != 1 || got.TargetType != "user" || got.UserID != userFact.UserID || got.PlatformCode != "" {
		t.Fatalf("user invalidation = %+v", got)
	}
	if state, found, err := store.ReadUser(ctx, userFact.UserID); err != nil || !found || state.State != StateInvalidating {
		t.Fatalf("user state after message = %+v,%v,%v", state, found, err)
	}
	if err := lease.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	lease, err = invalidator.Acquire(ctx, MutationFacts{Sessions: []SessionsFact{sessionsFact}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lease.Rollback(context.Background()) })
	if got := receiveRealtimeInvalidation(t, pubsub.Channel()); got.SchemaVersion != 1 || got.TargetType != "platformUser" || got.PlatformCode != sessionsFact.Platform || got.UserID != sessionsFact.UserID {
		t.Fatalf("sessions invalidation = %+v", got)
	}
	if state, found, err := store.ReadSessions(ctx, sessionsFact.Platform, sessionsFact.UserID); err != nil || !found || state.State != StateInvalidating {
		t.Fatalf("sessions state after message = %+v,%v,%v", state, found, err)
	}
}

func receiveRealtimeInvalidation(t *testing.T, messages <-chan *goredis.Message) receivedInvalidation {
	t.Helper()
	select {
	case message := <-messages:
		var payload receivedInvalidation
		if err := json.Unmarshal([]byte(message.Payload), &payload); err != nil {
			t.Fatal(err)
		}
		return payload
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for realtime invalidation")
		return receivedInvalidation{}
	}
}
