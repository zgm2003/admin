package rateLimitPolicy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"admin/server/internal/config"
	projectredis "admin/server/internal/redis"
	"github.com/joho/godotenv"
)

func openStoreRedis(t *testing.T) (*RedisStore, *projectredis.Client, context.Context) {
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
	return NewStore(client), client, ctx
}

func testCatalog(platformID int64, limit int) Catalog {
	catalog := Catalog{PlatformID: platformID, PlatformCode: "admin", PlatformName: "Admin"}
	for _, fixed := range FixedPolicies() {
		catalog.Policies = append(catalog.Policies, Model{
			PlatformID: platformID, Key: fixed.Key, Mode: fixed.Mode, Dimension: fixed.Dimension,
			Limit: limit, WindowSeconds: fixed.WindowSeconds, Revision: 1,
		})
	}
	return catalog
}

func TestStoreLoadsOnceAndInvalidatesAfterAMutation(t *testing.T) {
	store, _, ctx := openStoreRedis(t)
	platformID := time.Now().UnixNano()
	t.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = store.Mutate(cleanupContext, platformID, func(context.Context) error { return nil })
	})

	loads := 0
	load := func(context.Context) (Catalog, error) {
		loads++
		return testCatalog(platformID, 2), nil
	}

	first, err := store.Load(ctx, platformID, load)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if first.PlatformID != platformID || len(first.Policies) != 2 {
		t.Fatalf("catalog = %+v", first)
	}
	if _, err := store.Load(ctx, platformID, load); err != nil {
		t.Fatalf("cached Load() error = %v", err)
	}
	if loads != 1 {
		t.Fatalf("loader calls = %d, want 1", loads)
	}

	if err := store.Mutate(ctx, platformID, func(context.Context) error { return nil }); err != nil {
		t.Fatalf("Mutate() error = %v", err)
	}
	if _, err := store.Load(ctx, platformID, load); err != nil {
		t.Fatalf("Load() after mutation error = %v", err)
	}
	if loads != 2 {
		t.Fatalf("loader calls after mutation = %d, want 2", loads)
	}
}

func TestStoreKeepsTheSnapshotWhenTheChangeFails(t *testing.T) {
	store, _, ctx := openStoreRedis(t)
	platformID := time.Now().UnixNano() + 1
	t.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = store.Mutate(cleanupContext, platformID, func(context.Context) error { return nil })
	})

	loads := 0
	load := func(context.Context) (Catalog, error) {
		loads++
		return testCatalog(platformID, 2), nil
	}
	if _, err := store.Load(ctx, platformID, load); err != nil {
		t.Fatal(err)
	}

	failure := context.Canceled
	if err := store.Mutate(ctx, platformID, func(context.Context) error { return failure }); err == nil {
		t.Fatal("Mutate() accepted a failing change")
	}
	if _, err := store.Load(ctx, platformID, load); err != nil {
		t.Fatal(err)
	}
	if loads != 1 {
		t.Fatalf("failed mutation invalidated the snapshot: loader calls = %d", loads)
	}
}

func TestStoreRejectsCorruptSnapshots(t *testing.T) {
	store, client, ctx := openStoreRedis(t)
	platformID := time.Now().UnixNano() + 2
	key := snapshotKey(platformID)
	if err := client.SetString(ctx, key, `{"schemaVersion":1,"platformId":1,"unexpected":true}`, time.Minute); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = store.Mutate(cleanupContext, platformID, func(context.Context) error { return nil })
	})

	if _, err := store.Load(ctx, platformID, func(context.Context) (Catalog, error) {
		return testCatalog(platformID, 2), nil
	}); err == nil || !strings.Contains(err.Error(), "snapshot") {
		t.Fatalf("corrupt snapshot error = %v", err)
	}
}

func TestStoreStaleOwnerCannotDeleteASuccessor(t *testing.T) {
	_, client, ctx := openStoreRedis(t)
	platformID := time.Now().UnixNano() + 3
	leaseKey := mutationKey(platformID)

	if err := client.SetString(ctx, leaseKey, "successor-token", 30*time.Second); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = client.EvalString(cleanupContext, releaseMutationScript, []string{leaseKey}, "successor-token")
	})

	result, err := client.EvalString(ctx, publishMutationScript, []string{leaseKey, snapshotKey(platformID)}, "stale-owner-token")
	if err != nil {
		t.Fatal(err)
	}
	if result != "stale" {
		t.Fatalf("stale publish result = %q", result)
	}
	if _, found, err := client.GetString(ctx, leaseKey); err != nil || !found {
		t.Fatalf("stale owner deleted the successor lease: found=%v err=%v", found, err)
	}
}
