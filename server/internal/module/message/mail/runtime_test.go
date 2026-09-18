package mail

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	cachegeneration "admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/yesno"

	"gorm.io/gorm"
)

func TestRuntimeSnapshotCodecPreservesCiphertextAndRejectsMalformedPayloads(t *testing.T) {
	snapshot := validRuntimeSnapshot(3)
	raw, err := encodeRuntimeSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, snapshot.Config.SecretIDCiphertext) || !strings.Contains(raw, snapshot.Config.SecretKeyCiphertext) ||
		strings.Contains(raw, "plain-secret") {
		t.Fatalf("runtime snapshot did not preserve ciphertext boundaries: %s", raw)
	}
	decoded, err := decodeRuntimeSnapshot(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRuntimeSnapshot(decoded, SceneLogin); err != nil {
		t.Fatal(err)
	}
	if decoded.Generation != 3 || decoded.Config.SecretIDCiphertext != snapshot.Config.SecretIDCiphertext ||
		decoded.Config.SecretKeyCiphertext != snapshot.Config.SecretKeyCiphertext {
		t.Fatalf("decoded runtime snapshot = %+v", decoded)
	}

	for name, payload := range map[string]string{
		"unknown field":   strings.Replace(raw, `{`, `{"extra":true,`, 1),
		"duplicate field": strings.Replace(raw, `"generation":3`, `"generation":3,"generation":4`, 1),
		"trailing value":  raw + `{}`,
		"wrong schema":    strings.Replace(raw, `"schemaVersion":1`, `"schemaVersion":2`, 1),
		"zero generation": strings.Replace(raw, `"generation":3`, `"generation":0`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeRuntimeSnapshot(payload); !errors.Is(err, ErrRuntimeSnapshotCorrupt) {
				t.Fatalf("decode error = %v, want ErrRuntimeSnapshotCorrupt", err)
			}
		})
	}
}

func TestRuntimeSnapshotRejectsMissingCredentials(t *testing.T) {
	snapshot := validRuntimeSnapshot(1)
	snapshot.Config.SecretKeyCiphertext = ""
	if err := validateRuntimeSnapshot(snapshot, SceneLogin); err == nil {
		t.Fatal("runtime snapshot accepted missing encrypted credentials")
	}
}

func TestRuntimeMutationNoOpRestoresReadyWithoutAdvancingGeneration(t *testing.T) {
	database, ctx := openMailGenerationDatabase(t)
	client := openMailReadinessRedis(t)
	store := newMailCacheTestStore(t, database, readyMailRepository(0), client)

	if err := store.Mutate(ctx, func(context.Context, int64) (cachegeneration.MutationResult, error) {
		return cachegeneration.MutationResult{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	assertMailGenerationState(t, ctx, database, store, 1)
	if count := mailOutboxCount(t, ctx, database, false); count != 0 {
		t.Fatalf("no-op outbox rows = %d, want 0", count)
	}
}

func TestRuntimeMutationPublishesCommittedGenerationAndMarksOutbox(t *testing.T) {
	database, ctx := openMailGenerationDatabase(t)
	client := openMailReadinessRedis(t)
	store := newMailCacheTestStore(t, database, readyMailRepository(0), client)

	if err := store.Mutate(ctx, func(writeContext context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return advanceMailGeneration(writeContext, database, expected)
	}); err != nil {
		t.Fatal(err)
	}
	assertMailGenerationState(t, ctx, database, store, 2)
	if count := mailOutboxCount(t, ctx, database, false); count != 0 {
		t.Fatalf("pending outbox rows = %d, want 0", count)
	}
	if count := mailOutboxCount(t, ctx, database, true); count != 1 {
		t.Fatalf("published outbox rows = %d, want 1", count)
	}
}

func TestRuntimeMutationKnownRollbackRestoresBaseGeneration(t *testing.T) {
	database, ctx := openMailGenerationDatabase(t)
	client := openMailReadinessRedis(t)
	store := newMailCacheTestStore(t, database, readyMailRepository(0), client)
	forced := errors.New("forced transaction rollback")

	err := store.Mutate(ctx, func(context.Context, int64) (cachegeneration.MutationResult, error) {
		return cachegeneration.MutationResult{}, errors.Join(cachegeneration.ErrMutationRolledBack, forced)
	})
	if !errors.Is(err, forced) || !errors.Is(err, cachegeneration.ErrMutationRolledBack) {
		t.Fatalf("mutation error = %v", err)
	}
	assertMailGenerationState(t, ctx, database, store, 1)
}

func TestRuntimeMutationUncertainCommitPublishesAuthoritativeGeneration(t *testing.T) {
	database, ctx := openMailGenerationDatabase(t)
	client := openMailReadinessRedis(t)
	store := newMailCacheTestStore(t, database, readyMailRepository(0), client)
	forced := errors.New("connection reset after commit")

	err := store.Mutate(ctx, func(writeContext context.Context, expected int64) (cachegeneration.MutationResult, error) {
		if _, err := advanceMailGeneration(writeContext, database, expected); err != nil {
			return cachegeneration.MutationResult{}, err
		}
		return cachegeneration.MutationResult{}, forced
	})
	if err != nil {
		t.Fatalf("uncertain committed mutation returned an error: %v", err)
	}
	assertMailGenerationState(t, ctx, database, store, 2)
	if count := mailOutboxCount(t, ctx, database, false); count != 1 {
		t.Fatalf("pending outbox rows = %d, want 1 for relay recovery", count)
	}
}

func TestRuntimeMutationPublishFailureReturnsBusinessSuccess(t *testing.T) {
	database, ctx := openMailGenerationDatabase(t)
	client := openMailReadinessRedis(t)
	store := newMailCacheTestStore(t, database, readyMailRepository(0), client)

	err := store.Mutate(ctx, func(writeContext context.Context, expected int64) (cachegeneration.MutationResult, error) {
		result, err := advanceMailGeneration(writeContext, database, expected)
		if err != nil {
			return cachegeneration.MutationResult{}, err
		}
		if err := client.Close(); err != nil {
			return cachegeneration.MutationResult{}, err
		}
		return result, nil
	})
	if err != nil {
		t.Fatalf("committed mutation returned Redis publication failure: %v", err)
	}
	generation, err := cachegeneration.NewRepository(database).Current(ctx, mailGenerationScope)
	if err != nil || generation != 2 {
		t.Fatalf("PostgreSQL generation = %d, %v; want 2", generation, err)
	}
	if count := mailOutboxCount(t, ctx, database, false); count != 1 {
		t.Fatalf("pending outbox rows = %d, want 1", count)
	}

	cleanupClient := openMailReadinessRedis(t)
	if err := cleanupClient.DeleteMany(context.Background(), mailCacheTestKeys(t)); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeMutationRetryBudgetIsBounded(t *testing.T) {
	database, ctx := openMailGenerationDatabase(t)
	client := openMailReadinessRedis(t)
	store := newMailCacheTestStore(t, database, readyMailRepository(0), client)
	blockingLease, err := store.states.Acquire(ctx, mailGenerationScope, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = blockingLease.Rollback(context.Background()) })

	now := time.Unix(0, 0)
	waits := 0
	callbackCalls := 0
	store.now = func() time.Time { return now }
	store.wait = func(context.Context, time.Duration) error {
		waits++
		now = now.Add(mailWaitStep)
		return nil
	}
	err = store.Mutate(ctx, func(context.Context, int64) (cachegeneration.MutationResult, error) {
		callbackCalls++
		return cachegeneration.MutationResult{}, nil
	})
	if !errors.Is(err, cachegeneration.ErrUpdating) {
		t.Fatalf("mutation error = %v, want ErrUpdating", err)
	}
	if waits != 25 || callbackCalls != 0 {
		t.Fatalf("waits = %d, callback calls = %d; want 25 and 0", waits, callbackCalls)
	}
}

func validRuntimeSnapshot(generation int64) runtimeSnapshot {
	now := time.Now().UTC()
	return runtimeSnapshot{
		SchemaVersion: mailCacheSchemaVersion,
		Generation:    generation,
		Config: runtimeConfig{
			ID: 1, SecretIDCiphertext: "mail:v1:cipher-id", SecretKeyCiphertext: "mail:v1:cipher-key",
			Region: "ap-guangzhou", FromEmail: "sender@example.com", FromName: "Sender",
			TTLMinutes: 5, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now,
		},
		Templates: []Template{{
			ID: 1, Scene: SceneLogin, Name: "Login", Subject: "Code", Content: "{{code}}",
			TencentTemplateID: intPointer(47941), VariableKeys: []byte(`["code"]`),
			ExampleVariables: []byte(`{"code":"123456"}`), IsEnabled: yesno.Yes,
			CreatedAt: now, UpdatedAt: now,
		}},
		Rules: []RecipientRule{},
	}
}

func advanceMailGeneration(ctx context.Context, database *gorm.DB, expected int64) (cachegeneration.MutationResult, error) {
	result := cachegeneration.MutationResult{}
	err := database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		event, err := cachegeneration.NewRepository(database).AdvanceTx(ctx, tx, mailGenerationScope, expected, time.Now().UTC())
		if err != nil {
			return err
		}
		result = cachegeneration.MutationResult{Changed: true, Generation: event.Generation, OutboxID: event.ID}
		return nil
	})
	return result, err
}

func assertMailGenerationState(t *testing.T, ctx context.Context, database *gorm.DB, store *runtimeStore, want int64) {
	t.Helper()
	generation, err := cachegeneration.NewRepository(database).Current(ctx, mailGenerationScope)
	if err != nil || generation != want {
		t.Fatalf("PostgreSQL generation = %d, %v; want %d", generation, err, want)
	}
	state, found, err := store.states.Read(ctx, mailGenerationScope)
	if err != nil || !found || state.State != cachegeneration.StateReady || state.Generation != want {
		t.Fatalf("Redis state = %+v, found=%v, err=%v; want ready %d", state, found, err, want)
	}
}

func mailOutboxCount(t *testing.T, ctx context.Context, database *gorm.DB, published bool) int64 {
	t.Helper()
	condition := "published_at IS NULL"
	if published {
		condition = "published_at IS NOT NULL"
	}
	var count int64
	if err := database.WithContext(ctx).Raw(`SELECT count(*) FROM system_config_cache_outbox WHERE namespace = ? AND scope_key = ? AND `+condition,
		mailGenerationScope.Namespace, mailGenerationScope.ScopeKey).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func intPointer(value int) *int { return &value }

func TestRuntimeStoreValidateDependenciesRejectsIncompleteStartupWiring(t *testing.T) {
	if err := newRuntimeStore(nil, nil).ValidateDependencies(); err == nil {
		t.Fatal("ValidateDependencies accepted missing Mail runtime dependencies")
	}
}
