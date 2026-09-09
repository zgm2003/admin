package mail

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	mailtemplate "admin/server/internal/module/message/mail/template"
	"admin/server/internal/shared/yesno"
)

type runtimeContextKey struct{}

func TestRuntimeRebuildContextKeepsValuesAndDetachesCancellation(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.WithValue(context.Background(), runtimeContextKey{}, "request-value"))
	shared, cancelShared := newRuntimeRebuildContext(parent)
	defer cancelShared()
	cancelParent()
	if err := shared.Err(); err != nil {
		t.Fatalf("shared rebuild inherited caller cancellation: %v", err)
	}
	if got := shared.Value(runtimeContextKey{}); got != "request-value" {
		t.Fatalf("shared rebuild request value=%v", got)
	}
	deadline, ok := shared.Deadline()
	if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > mailRuntimeLoadLockTTL*time.Duration(mailRuntimeRebuildAttempts) {
		t.Fatalf("shared rebuild deadline=%v ok=%v", deadline, ok)
	}
}

func TestRuntimeMutationBlocksReadsAndUsesTokenizedPublication(t *testing.T) {
	client := openMailReadinessRedis(t)
	ctx := context.Background()
	store := newRuntimeStore(nil, client)
	keys := runtimeTestKeys(store)
	if err := client.DeleteMany(ctx, keys); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), keys) })
	if err := client.SetString(ctx, mailRuntimeGenerationKey, "7", 0); err != nil {
		t.Fatal(err)
	}
	for _, fixed := range mailtemplate.FixedCatalog() {
		if err := client.SetString(ctx, store.key(fixed.Scene), `{"generation":7}`, time.Minute); err != nil {
			t.Fatal(err)
		}
	}

	mutation, err := store.beginMutation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.readGeneration(ctx); err == nil {
		t.Fatal("runtime generation remained readable during mutation")
	}
	if _, err := store.beginMutation(ctx); err == nil {
		t.Fatal("concurrent runtime mutation acquired the active lease")
	}
	for _, fixed := range mailtemplate.FixedCatalog() {
		if _, found, readErr := client.GetString(ctx, store.key(fixed.Scene)); readErr != nil || found {
			t.Fatalf("old runtime snapshot %q found=%v err=%v", fixed.Scene, found, readErr)
		}
	}
	if err := store.finishMutation(ctx, runtimeMutation{token: "stale-token"}); err == nil {
		t.Fatal("stale runtime mutation token published")
	}
	if err := store.finishMutation(ctx, mutation); err != nil {
		t.Fatal(err)
	}
	generation, err := store.readGeneration(ctx)
	if err != nil || generation != 8 {
		t.Fatalf("runtime generation=%d err=%v", generation, err)
	}
}

func TestRuntimeMutationPublicationOutlivesCancellationAndWriteFailure(t *testing.T) {
	client := openMailReadinessRedis(t)
	store := newRuntimeStore(nil, client)
	keys := runtimeTestKeys(store)
	if err := client.DeleteMany(context.Background(), keys); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), keys) })

	ctx, cancel := context.WithCancel(context.Background())
	if err := store.Mutate(ctx, func(context.Context) error {
		cancel()
		return nil
	}); err != nil {
		t.Fatalf("canceled caller prevented runtime publication: %v", err)
	}
	if _, found, err := client.GetString(context.Background(), mailRuntimeMutationKey); err != nil || found {
		t.Fatalf("runtime mutation remained after publication found=%v err=%v", found, err)
	}

	forced := errors.New("forced database failure")
	if err := store.Mutate(context.Background(), func(context.Context) error { return forced }); !errors.Is(err, forced) {
		t.Fatalf("runtime mutation write error=%v", err)
	}
	if _, found, err := client.GetString(context.Background(), mailRuntimeMutationKey); err != nil || found {
		t.Fatalf("runtime mutation remained after rollback found=%v err=%v", found, err)
	}
}

func runtimeTestKeys(store *runtimeStore) []string {
	keys := []string{mailRuntimeGenerationKey, mailRuntimeMutationKey, mailRuntimeLoadLockKey}
	for _, fixed := range mailtemplate.FixedCatalog() {
		keys = append(keys, store.key(fixed.Scene))
	}
	return keys
}

func TestRuntimeSnapshotCarriesEncryptedMailCredentials(t *testing.T) {
	now := time.Now().UTC()
	snapshot := runtimeSnapshot{
		Generation: 3,
		Config: runtimeConfig{
			ID:                  1,
			SecretIDCiphertext:  "mail:v1:secret-id",
			SecretKeyCiphertext: "mail:v1:secret-key",
			Region:              "ap-guangzhou",
			FromEmail:           "sender@example.com",
			FromName:            "Sender",
			TTLMinutes:          5,
			IsEnabled:           yesno.Yes,
			CreatedAt:           now,
			UpdatedAt:           now,
		},
		Templates: []Template{{ID: 1, Scene: SceneLogin, TencentTemplateID: 47941, IsEnabled: yesno.Yes}},
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), snapshot.Config.SecretIDCiphertext) || !strings.Contains(string(raw), snapshot.Config.SecretKeyCiphertext) {
		t.Fatalf("runtime snapshot omitted encrypted credentials: %s", raw)
	}
	var decoded runtimeSnapshot
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := validateRuntimeSnapshot(decoded, SceneLogin); err != nil {
		t.Fatal(err)
	}
	if decoded.Config.SecretIDCiphertext != snapshot.Config.SecretIDCiphertext || decoded.Config.SecretKeyCiphertext != snapshot.Config.SecretKeyCiphertext {
		t.Fatalf("credentials did not round-trip: %+v", decoded.Config)
	}
}

func TestRuntimeSnapshotRejectsMissingCredentials(t *testing.T) {
	snapshot := runtimeSnapshot{
		Generation: 1,
		Config:     runtimeConfig{ID: 1, Region: "ap-guangzhou", FromEmail: "sender@example.com", FromName: "Sender", TTLMinutes: 5, IsEnabled: yesno.Yes, UpdatedAt: time.Now().UTC()},
		Templates:  []Template{{ID: 1, Scene: SceneLogin, TencentTemplateID: 47941, IsEnabled: yesno.Yes}},
	}
	if err := validateRuntimeSnapshot(snapshot, SceneLogin); err == nil {
		t.Fatal("runtime snapshot accepted missing encrypted credentials")
	}
}
