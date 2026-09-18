package ratelimitpolicy

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	projectconfig "admin/server/internal/config"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/apperror"
	cachegeneration "admin/server/internal/shared/cacheGeneration"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

type countingPolicyRepository struct {
	calls atomic.Int32
	delay time.Duration
	hook  func(context.Context)
	once  sync.Once
}

func (r *countingPolicyRepository) List(ctx context.Context, platformID int64) (Catalog, error) {
	r.calls.Add(1)
	if r.delay > 0 {
		timer := time.NewTimer(r.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return Catalog{}, ctx.Err()
		case <-timer.C:
		}
	}
	if r.hook != nil {
		r.once.Do(func() { r.hook(ctx) })
	}
	return Catalog{PlatformID: platformID, Policies: fixedPoliciesWithTimestamp(time.Now().UTC())}, nil
}

func TestRateLimitPolicyStoreRejectsMissingDependencies(t *testing.T) {
	store := NewRateLimitPolicyStore(nil, nil)
	if _, err := store.Load(context.Background(), 1); err == nil {
		t.Fatal("Load accepted missing cache generation dependencies")
	}
}

func TestRateLimitPolicyCatalogIsFixed(t *testing.T) {
	got := FixedRateLimitPolicies()
	if len(got) != 2 {
		t.Fatalf("policy count = %d, want 2", len(got))
	}
	if got[0].Key != "business_email_minute" || got[0].Limit != 1 || got[0].WindowSeconds != 60 {
		t.Fatalf("business email policy = %+v", got[0])
	}
	if got[1].Key != "business_email_10m" || got[1].Limit != 5 || got[1].WindowSeconds != 600 {
		t.Fatalf("email policy = %+v", got[1])
	}
}

func TestRateLimitPolicyInputRejectsUnknownKeyAndOutOfRangeValues(t *testing.T) {
	for _, input := range []RateLimitPolicyInput{
		{Key: "custom", Limit: 1, WindowSeconds: 60},
		{Key: "business_email_minute", Limit: 0, WindowSeconds: 60},
		{Key: "business_email_minute", Limit: 100001, WindowSeconds: 60},
		{Key: "business_email_minute", Limit: 1, WindowSeconds: 0},
		{Key: "business_email_minute", Limit: 1, WindowSeconds: 86401},
	} {
		if err := ValidateRateLimitPolicyInput(input); err == nil {
			t.Fatalf("accepted %+v", input)
		}
	}
}

func TestRateLimitSnapshotRoundTripUsesGenerationAndRejectsOldVersionFields(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	catalog := RateLimitCatalog{PlatformID: 1, Policies: fixedPoliciesWithTimestamp(now)}
	snapshot, err := snapshotFromCatalog(4, catalog)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := encodeRateLimitSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeRateLimitSnapshot(raw, 4, 1)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Generation != 4 || decoded.PlatformID != 1 || len(decoded.Policies) != 2 {
		t.Fatalf("decoded snapshot = %+v", decoded)
	}
	roundTrip := catalogFromSnapshot(decoded)
	if len(roundTrip.Policies) != 2 {
		t.Fatalf("round trip catalog = %+v", roundTrip)
	}
	for _, policy := range roundTrip.Policies {
		spec, ok := fixedRateLimitSpecByKey(policy.Key)
		if !ok || policy.Mode != spec.Mode || policy.Dimension != spec.Dimension {
			t.Fatalf("round trip policy %q lost static metadata: %+v", policy.Key, policy)
		}
	}

	for name, payload := range map[string]string{
		"empty":            `{}`,
		"old version":      strings.Replace(raw, `"generation":4`, `"generation":4,"version":4`, 1),
		"old state":        strings.Replace(raw, `{`, `{"state":"ready",`, 1),
		"mutation token":   strings.Replace(raw, `{`, `{"mutationToken":"token",`, 1),
		"unknown field":    strings.Replace(raw, `{`, `{"extra":true,`, 1),
		"duplicate field":  strings.Replace(raw, `"generation":4`, `"generation":4,"generation":5`, 1),
		"wrong generation": strings.Replace(raw, `"generation":4`, `"generation":5`, 1),
		"wrong platform":   strings.Replace(raw, `"platformId":1`, `"platformId":2`, 1),
		"trailing value":   raw + `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeRateLimitSnapshot(payload, 4, 1); err == nil {
				t.Fatalf("accepted malformed snapshot: %s", payload)
			}
		})
	}
}

func TestRateLimitStoreTwoInstancesUseOneColdFillAndHotReadsAvoidPostgres(t *testing.T) {
	database, ctx := openRepositoryDatabase(t)
	scope := newRateLimitTestScope(t, database, ctx)
	firstClient := openRateLimitRedis(t)
	secondClient := openRateLimitRedis(t)
	repository := &countingPolicyRepository{}
	first := newRateLimitTestStore(t, repository, database, firstClient, scope)
	second := newRateLimitStore(repository, secondClient)
	second.SetGenerations(cachegeneration.NewRepository(database), cachegeneration.NewStore(secondClient))
	second.scope = scope

	leaderEntered := make(chan struct{})
	contenderWaited := make(chan struct{})
	releaseLeader := make(chan struct{})
	var entered sync.Once
	var waited sync.Once
	repository.delay = 0
	repository.hook = func(context.Context) {
		entered.Do(func() { close(leaderEntered) })
		<-releaseLeader
	}
	for _, store := range []*rateLimitPolicyStore{first, second} {
		store.wait = func(ctx context.Context, duration time.Duration) error {
			waited.Do(func() { close(contenderWaited) })
			return waitRateLimitRetry(ctx, duration)
		}
	}

	errorsFound := make(chan error, 2)
	var group sync.WaitGroup
	for _, store := range []*rateLimitPolicyStore{first, second} {
		group.Add(1)
		go func(store *rateLimitPolicyStore) {
			defer group.Done()
			catalog, err := store.Load(context.Background(), 1)
			if err != nil || len(catalog.Policies) != 2 {
				errorsFound <- fmt.Errorf("catalog=%+v err=%w", catalog, err)
			}
		}(store)
	}
	select {
	case <-leaderEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("no cold fill leader reached PostgreSQL")
	}
	select {
	case <-contenderWaited:
	case <-time.After(2 * time.Second):
		t.Fatal("second instance did not contend for the fill lease")
	}
	close(releaseLeader)
	group.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Fatal(err)
	}
	if repository.calls.Load() != 1 {
		t.Fatalf("PostgreSQL loads=%d, want 1", repository.calls.Load())
	}
	for index := 0; index < 20; index++ {
		stores := []*rateLimitPolicyStore{first, second}
		if _, err := stores[index%len(stores)].Load(ctx, 1); err != nil {
			t.Fatal(err)
		}
	}
	if repository.calls.Load() != 1 {
		t.Fatalf("hot reads reached PostgreSQL: %d", repository.calls.Load())
	}
}

func TestRateLimitStoreRebuildsCorruptSnapshot(t *testing.T) {
	database, ctx := openRepositoryDatabase(t)
	scope := newRateLimitTestScope(t, database, ctx)
	client := openRateLimitRedis(t)
	repository := &countingPolicyRepository{}
	store := newRateLimitTestStore(t, repository, database, client, scope)
	key, _ := cachegeneration.SnapshotKey(scope, 1, "ratelimit:1")
	if err := client.SetString(ctx, key, `{"schemaVersion":1,"generation":1,"platformId":1,"policies":{},"version":1}`, time.Minute); err != nil {
		t.Fatal(err)
	}
	catalog, err := store.Load(ctx, 1)
	if err != nil || len(catalog.Policies) != 2 {
		t.Fatalf("rebuilt catalog=%+v err=%v", catalog, err)
	}
	if repository.calls.Load() != 1 {
		t.Fatalf("PostgreSQL loads=%d, want 1", repository.calls.Load())
	}
	raw, found, err := client.GetString(ctx, key)
	if err != nil || !found || strings.Contains(raw, `"version"`) {
		t.Fatalf("rebuilt snapshot found=%v err=%v raw=%s", found, err, raw)
	}
}

func TestRateLimitStoreRedisFailureFailsClosedWithoutPostgres(t *testing.T) {
	database, ctx := openRepositoryDatabase(t)
	scope := newRateLimitTestScope(t, database, ctx)
	client := openRateLimitRedis(t)
	repository := &countingPolicyRepository{}
	store := newRateLimitStore(repository, client)
	store.SetGenerations(cachegeneration.NewRepository(database), cachegeneration.NewStore(client))
	store.scope = scope
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(ctx, 1); err == nil {
		t.Fatal("closed Redis was accepted")
	}
	if repository.calls.Load() != 0 {
		t.Fatalf("Redis failure fell back to PostgreSQL: %d", repository.calls.Load())
	}
}

func TestRateLimitStoreDropsOldFillWhenGenerationAdvances(t *testing.T) {
	database, ctx := openRepositoryDatabase(t)
	scope := newRateLimitTestScope(t, database, ctx)
	client := openRateLimitRedis(t)
	repository := &countingPolicyRepository{}
	store := newRateLimitTestStore(t, repository, database, client, scope)
	repository.hook = func(hookContext context.Context) {
		result, err := advanceRateLimitGeneration(hookContext, database, scope, 1)
		if err != nil {
			t.Errorf("advance generation: %v", err)
			return
		}
		if _, err := store.states.Reconcile(hookContext, scope, result.Generation); err != nil {
			t.Errorf("publish generation: %v", err)
		}
	}
	catalog, err := store.Load(ctx, 1)
	if err != nil || len(catalog.Policies) != 2 {
		t.Fatalf("catalog after generation advance=%+v err=%v", catalog, err)
	}
	oldKey, _ := cachegeneration.SnapshotKey(scope, 1, "ratelimit:1")
	if _, found, err := client.GetString(ctx, oldKey); err != nil || found {
		t.Fatalf("old snapshot found=%v err=%v", found, err)
	}
	newKey, _ := cachegeneration.SnapshotKey(scope, 2, "ratelimit:1")
	if _, found, err := client.GetString(ctx, newKey); err != nil || !found {
		t.Fatalf("new snapshot found=%v err=%v", found, err)
	}
}

func TestRateLimitStoreInvalidatingReadBudgetIsBounded(t *testing.T) {
	database, ctx := openRepositoryDatabase(t)
	scope := newRateLimitTestScope(t, database, ctx)
	client := openRateLimitRedis(t)
	repository := &countingPolicyRepository{}
	store := newRateLimitTestStore(t, repository, database, client, scope)
	lease, err := store.states.Acquire(ctx, scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lease.Rollback(context.Background()) })

	now := time.Unix(0, 0)
	waits := 0
	store.now = func() time.Time { return now }
	store.wait = func(context.Context, time.Duration) error {
		waits++
		now = now.Add(rateLimitPolicyWaitStep)
		return nil
	}
	_, err = store.Load(ctx, 1)
	var applicationError *apperror.Error
	if !errors.As(err, &applicationError) || applicationError.Code != apperror.CodeDependencyUnavailable ||
		applicationError.Cause == nil || !strings.Contains(applicationError.Cause.Error(), "budget exhausted") {
		t.Fatalf("Load error=%v, want bounded invalidating failure", err)
	}
	if waits != 25 || repository.calls.Load() != 0 {
		t.Fatalf("waits=%d PostgreSQL loads=%d, want 25 and 0", waits, repository.calls.Load())
	}
}

func openRateLimitRedis(t *testing.T) *projectredis.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("Redis integration")
	}
	_ = godotenv.Load("../../../../../.env")
	settings, err := projectconfig.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	client, err := projectredis.Open(context.Background(), settings.RedisURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func newRateLimitTestScope(t *testing.T, database *gorm.DB, ctx context.Context) cachegeneration.Scope {
	t.Helper()
	scope := cachegeneration.Scope{Namespace: "message.mail", ScopeKey: fmt.Sprintf("test-%d", time.Now().UnixNano())}
	now := time.Now().UTC()
	if err := database.WithContext(ctx).Exec(`
INSERT INTO system_config_cache_generation(namespace,scope_key,generation,created_at,updated_at)
VALUES (?,?,1,?,?)`, scope.Namespace, scope.ScopeKey, now, now).Error; err != nil {
		t.Fatal(err)
	}
	return scope
}

func newRateLimitTestStore(t *testing.T, repository policyLoader, database *gorm.DB, client *projectredis.Client, scope cachegeneration.Scope) *rateLimitPolicyStore {
	t.Helper()
	keys := rateLimitCacheTestKeys(t, scope)
	if err := client.DeleteMany(context.Background(), keys); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), keys) })
	store := newRateLimitStore(repository, client)
	store.SetGenerations(cachegeneration.NewRepository(database), cachegeneration.NewStore(client))
	store.scope = scope
	if _, err := store.states.Reconcile(context.Background(), scope, 1); err != nil {
		t.Fatal(err)
	}
	return store
}

func rateLimitCacheTestKeys(t *testing.T, scope cachegeneration.Scope) []string {
	t.Helper()
	stateKey := cachegeneration.StateKey(scope)
	keys := []string{stateKey}
	for generation := int64(1); generation <= 4; generation++ {
		for platformID := int64(1); platformID <= 2; platformID++ {
			variant := fmt.Sprintf("ratelimit:%d", platformID)
			snapshot, err := cachegeneration.SnapshotKey(scope, generation, variant)
			if err != nil {
				t.Fatal(err)
			}
			fill, err := cachegeneration.FillKey(scope, generation, variant)
			if err != nil {
				t.Fatal(err)
			}
			keys = append(keys, snapshot)
			keys = append(keys, rateLimitFillKeys(stateKey, fill)...)
		}
	}
	keys = append(keys, rateLimitFillKeys(stateKey, "repair:ratelimit:1")...)
	return keys
}

func rateLimitFillKeys(scope, target string) []string {
	scopeHash := sha256.Sum256([]byte(scope))
	targetHash := sha256.Sum256([]byte(target))
	prefix := fmt.Sprintf("cachefill:{%x}:", scopeHash[:8])
	return []string{fmt.Sprintf("%skey:%x", prefix, targetHash), prefix + "slots", prefix + "starts"}
}

func advanceRateLimitGeneration(ctx context.Context, database *gorm.DB, scope cachegeneration.Scope, expected int64) (cachegeneration.MutationResult, error) {
	result := cachegeneration.MutationResult{}
	err := database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		event, err := cachegeneration.NewRepository(database).AdvanceTx(ctx, tx, scope, expected, time.Now().UTC())
		if err != nil {
			return err
		}
		result = cachegeneration.MutationResult{Changed: true, Generation: event.Generation, OutboxID: event.ID}
		return nil
	})
	return result, err
}

func fixedPoliciesWithTimestamp(at time.Time) []RateLimitPolicy {
	policies := FixedRateLimitPolicies()
	for index := range policies {
		policies[index].PlatformID = 1
		policies[index].CreatedAt = at
		policies[index].UpdatedAt = at
	}
	return policies
}

func TestRateLimitStoreValidateDependenciesRejectsIncompleteStartupWiring(t *testing.T) {
	if err := NewStore(nil, nil).ValidateDependencies(); err == nil {
		t.Fatal("ValidateDependencies accepted missing Mail rate-limit dependencies")
	}
}
