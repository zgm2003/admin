package rateLimitPolicy

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"admin/server/internal/config"
	projectredis "admin/server/internal/redis"
	cachegeneration "admin/server/internal/shared/cacheGeneration"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func openStoreRedis(t *testing.T) (*projectredis.Client, context.Context) {
	t.Helper()
	if err := godotenv.Load(filepath.Join("..", "..", "..", "..", "..", ".env")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	client, err := projectredis.Open(ctx, settings.RedisURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client, ctx
}

func testCatalog(platformID int64, limit int) Catalog {
	now := time.Now().UTC()
	catalog := Catalog{PlatformID: platformID, PlatformCode: "admin", PlatformName: "Admin"}
	for _, fixed := range FixedPolicies() {
		catalog.Policies = append(catalog.Policies, Model{
			PlatformID: platformID, Key: fixed.Key, Mode: fixed.Mode, Dimension: fixed.Dimension,
			Limit: limit, WindowSeconds: fixed.WindowSeconds, CreatedAt: now, UpdatedAt: now,
		})
	}
	return catalog
}

func TestSnapshotCodecUsesGenerationAndRejectsRevisionAndOldState(t *testing.T) {
	snapshot, err := snapshotFromCatalog(4, testCatalog(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := encodeSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeSnapshot(raw, 4, 1)
	if err != nil || decoded.Generation != 4 || decoded.PlatformID != 1 || len(decoded.Policies) != 2 {
		t.Fatalf("decoded snapshot=%+v err=%v", decoded, err)
	}
	for name, payload := range map[string]string{
		"revision":         strings.Replace(raw, `"limit":2`, `"limit":2,"revision":1`, 1),
		"state":            strings.Replace(raw, `{`, `{"state":"ready",`, 1),
		"mutation token":   strings.Replace(raw, `{`, `{"mutationToken":"token",`, 1),
		"unknown field":    strings.Replace(raw, `{`, `{"extra":true,`, 1),
		"duplicate field":  strings.Replace(raw, `"generation":4`, `"generation":4,"generation":5`, 1),
		"wrong generation": strings.Replace(raw, `"generation":4`, `"generation":5`, 1),
		"wrong platform":   strings.Replace(raw, `"platformId":1`, `"platformId":2`, 1),
		"trailing value":   raw + `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeSnapshot(payload, 4, 1); err == nil {
				t.Fatalf("accepted malformed snapshot: %s", payload)
			}
		})
	}
}

func TestStoreTwoInstancesUseOneColdFillAndHotReadsAvoidPostgres(t *testing.T) {
	database, ctx := openPolicySchema(t)
	scope := newSMSRateLimitTestScope(t, database, ctx)
	firstClient, _ := openStoreRedis(t)
	secondClient, _ := openStoreRedis(t)
	first := newSMSRateLimitTestStore(t, database, firstClient, scope)
	second := NewStore(secondClient)
	second.SetGenerations(cachegeneration.NewRepository(database), cachegeneration.NewStore(secondClient))
	second.scope = scope

	var loads atomic.Int32
	leaderEntered := make(chan struct{})
	contenderWaited := make(chan struct{})
	releaseLeader := make(chan struct{})
	var entered sync.Once
	var waited sync.Once
	loader := func(ctx context.Context) (Catalog, error) {
		loads.Add(1)
		entered.Do(func() { close(leaderEntered) })
		select {
		case <-ctx.Done():
			return Catalog{}, ctx.Err()
		case <-releaseLeader:
			return testCatalog(1, 2), nil
		}
	}
	for _, store := range []*RedisStore{first, second} {
		store.wait = func(ctx context.Context, duration time.Duration) error {
			waited.Do(func() { close(contenderWaited) })
			return waitForSMSRateLimit(ctx, duration)
		}
	}
	errorsFound := make(chan error, 2)
	var group sync.WaitGroup
	for _, store := range []*RedisStore{first, second} {
		group.Add(1)
		go func(store *RedisStore) {
			defer group.Done()
			catalog, err := store.Load(context.Background(), 1, loader)
			if err != nil || len(catalog.Policies) != 2 {
				errorsFound <- fmt.Errorf("catalog=%+v err=%w", catalog, err)
			}
		}(store)
	}
	select {
	case <-leaderEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("no cold fill leader entered the loader")
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
	if loads.Load() != 1 {
		t.Fatalf("loader calls=%d, want 1", loads.Load())
	}
	for index := 0; index < 20; index++ {
		stores := []*RedisStore{first, second}
		if _, err := stores[index%len(stores)].Load(ctx, 1, loader); err != nil {
			t.Fatal(err)
		}
	}
	if loads.Load() != 1 {
		t.Fatalf("hot reads reached PostgreSQL: %d", loads.Load())
	}
}

func TestStoreRebuildsCorruptSnapshot(t *testing.T) {
	database, ctx := openPolicySchema(t)
	scope := newSMSRateLimitTestScope(t, database, ctx)
	client, _ := openStoreRedis(t)
	store := newSMSRateLimitTestStore(t, database, client, scope)
	key, _ := cachegeneration.SnapshotKey(scope, 1, "ratelimit:1")
	if err := client.SetString(ctx, key, `{"schemaVersion":1,"generation":1,"platformId":1,"policies":{},"revision":1}`, time.Minute); err != nil {
		t.Fatal(err)
	}
	var loads atomic.Int32
	catalog, err := store.Load(ctx, 1, func(context.Context) (Catalog, error) {
		loads.Add(1)
		return testCatalog(1, 2), nil
	})
	if err != nil || len(catalog.Policies) != 2 {
		t.Fatalf("catalog=%+v err=%v", catalog, err)
	}
	if loads.Load() != 1 {
		t.Fatalf("loader calls=%d, want 1", loads.Load())
	}
	raw, found, err := client.GetString(ctx, key)
	if err != nil || !found || strings.Contains(raw, "revision") {
		t.Fatalf("rebuilt snapshot found=%v err=%v raw=%s", found, err, raw)
	}
}

func TestStoreRedisFailureFailsClosedWithoutPostgres(t *testing.T) {
	database, ctx := openPolicySchema(t)
	scope := newSMSRateLimitTestScope(t, database, ctx)
	client, _ := openStoreRedis(t)
	store := NewStore(client)
	store.SetGenerations(cachegeneration.NewRepository(database), cachegeneration.NewStore(client))
	store.scope = scope
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	var loads atomic.Int32
	if _, err := store.Load(ctx, 1, func(context.Context) (Catalog, error) {
		loads.Add(1)
		return testCatalog(1, 2), nil
	}); err == nil {
		t.Fatal("closed Redis was accepted")
	}
	if loads.Load() != 0 {
		t.Fatalf("Redis failure reached PostgreSQL: %d", loads.Load())
	}
}

func TestStoreDropsOldFillWhenGenerationAdvances(t *testing.T) {
	database, ctx := openPolicySchema(t)
	scope := newSMSRateLimitTestScope(t, database, ctx)
	client, _ := openStoreRedis(t)
	store := newSMSRateLimitTestStore(t, database, client, scope)
	var loads atomic.Int32
	catalog, err := store.Load(ctx, 1, func(loadContext context.Context) (Catalog, error) {
		if loads.Add(1) == 1 {
			result, advanceErr := advanceSMSRateLimitGeneration(loadContext, database, scope, 1)
			if advanceErr != nil {
				return Catalog{}, advanceErr
			}
			if _, reconcileErr := store.states.Reconcile(loadContext, scope, result.Generation); reconcileErr != nil {
				return Catalog{}, reconcileErr
			}
		}
		return testCatalog(1, 2), nil
	})
	if err != nil || len(catalog.Policies) != 2 {
		t.Fatalf("catalog=%+v err=%v", catalog, err)
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

func TestStoreInvalidatingReadBudgetIsBounded(t *testing.T) {
	database, ctx := openPolicySchema(t)
	scope := newSMSRateLimitTestScope(t, database, ctx)
	client, _ := openStoreRedis(t)
	store := newSMSRateLimitTestStore(t, database, client, scope)
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
		now = now.Add(smsRateLimitWaitStep)
		return nil
	}
	var loads atomic.Int32
	_, err = store.Load(ctx, 1, func(context.Context) (Catalog, error) {
		loads.Add(1)
		return testCatalog(1, 2), nil
	})
	if err == nil || !strings.Contains(err.Error(), "budget exhausted") {
		t.Fatalf("Load error=%v, want bounded failure", err)
	}
	if waits != 25 || loads.Load() != 0 {
		t.Fatalf("waits=%d loads=%d, want 25 and 0", waits, loads.Load())
	}
}

func TestStoreRejectsMissingGenerationDependencies(t *testing.T) {
	if _, err := NewStore(nil).Load(context.Background(), 1, func(context.Context) (Catalog, error) {
		return Catalog{}, nil
	}); err == nil {
		t.Fatal("store accepted missing dependencies")
	}
}

func newSMSRateLimitTestScope(t *testing.T, database *gorm.DB, ctx context.Context) cachegeneration.Scope {
	t.Helper()
	scope := cachegeneration.Scope{Namespace: "message.sms", ScopeKey: fmt.Sprintf("test-%d", time.Now().UnixNano())}
	now := time.Now().UTC()
	if err := database.WithContext(ctx).Exec(`
INSERT INTO system_config_cache_generation(namespace,scope_key,generation,created_at,updated_at)
VALUES (?,?,1,?,?)`, scope.Namespace, scope.ScopeKey, now, now).Error; err != nil {
		t.Fatal(err)
	}
	return scope
}

func newSMSRateLimitTestStore(t *testing.T, database *gorm.DB, client *projectredis.Client, scope cachegeneration.Scope) *RedisStore {
	t.Helper()
	keys := smsRateLimitCacheKeys(t, scope)
	if err := client.DeleteMany(context.Background(), keys); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), keys) })
	store := NewStore(client)
	store.SetGenerations(cachegeneration.NewRepository(database), cachegeneration.NewStore(client))
	store.scope = scope
	if _, err := store.states.Reconcile(context.Background(), scope, 1); err != nil {
		t.Fatal(err)
	}
	return store
}

func smsRateLimitCacheKeys(t *testing.T, scope cachegeneration.Scope) []string {
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
			keys = append(keys, smsRateLimitFillKeys(stateKey, fill)...)
		}
	}
	keys = append(keys, smsRateLimitFillKeys(stateKey, "repair:ratelimit:1")...)
	return keys
}

func smsRateLimitFillKeys(scope, target string) []string {
	scopeHash := sha256.Sum256([]byte(scope))
	targetHash := sha256.Sum256([]byte(target))
	prefix := fmt.Sprintf("cachefill:{%x}:", scopeHash[:8])
	return []string{fmt.Sprintf("%skey:%x", prefix, targetHash), prefix + "slots", prefix + "starts"}
}

func advanceSMSRateLimitGeneration(ctx context.Context, database *gorm.DB, scope cachegeneration.Scope, expected int64) (cachegeneration.MutationResult, error) {
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

func TestRateLimitStoreValidateDependenciesRejectsIncompleteStartupWiring(t *testing.T) {
	if err := NewStore(nil).ValidateDependencies(); err == nil {
		t.Fatal("ValidateDependencies accepted missing SMS rate-limit dependencies")
	}
}
