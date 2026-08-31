package permission

import (
	"context"
	"os"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/module/permission/state"
	projectredis "admin/server/internal/redis"
	"github.com/joho/godotenv"
)

func TestSnapshotKey(t *testing.T) {
	if got := SnapshotKey("admin", 4, 7, 9); got != "authz:permission:admin:4:7:9" {
		t.Fatalf("SnapshotKey() = %q", got)
	}
}

func TestSnapshotSchemaVersionIsCurrent(t *testing.T) {
	if accessSnapshotSchemaVersion != 4 {
		t.Fatalf("accessSnapshotSchemaVersion = %d, want 4", accessSnapshotSchemaVersion)
	}
}

func TestSnapshotCachePublishesOnlyForMatchingReadyVersion(t *testing.T) {
	client := openAccessRedis(t)
	states := permissionstate.NewStore(client)
	cache := NewSnapshotCache(client)
	ctx := context.Background()
	cleanupAccessKeys(t, client, 94001, "admin", 4, 3)
	_, _, _ = states.InstallReadyIfMissing(ctx, permissionstate.Version{UserID: 94001, Version: 3})
	snapshot, err := buildSnapshot(baseSource(3))
	if err != nil {
		t.Fatal(err)
	}
	cached := cachedSnapshot(94001, 1, "admin", 4, snapshot)
	published, err := cache.PublishIfCurrent(ctx, cached, time.Minute)
	if err != nil || !published {
		t.Fatalf("PublishIfCurrent() = %v,%v", published, err)
	}
	read, found, err := cache.Read(ctx, 1, "admin", 4, 94001, 3)
	if err != nil || !found || read.PlatformID != 1 || read.Version != 3 {
		t.Fatalf("Read() = %+v,%v,%v", read, found, err)
	}

	lease, err := permissionstate.NewInvalidator(states).Acquire(ctx, []permissionstate.Version{{UserID: 94001, Version: 3}})
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Commit(ctx, map[int64]int64{94001: 4}); err != nil {
		t.Fatal(err)
	}
	published, err = cache.PublishIfCurrent(ctx, cached, time.Minute)
	if err != nil || published {
		t.Fatalf("stale PublishIfCurrent() = %v,%v", published, err)
	}
}

func TestSnapshotCacheRejectsUnknownFieldsAndMismatchedIdentity(t *testing.T) {
	client := openAccessRedis(t)
	cache := NewSnapshotCache(client)
	ctx := context.Background()
	key := SnapshotKey("admin", 4, 94002, 3)
	t.Cleanup(func() { _ = client.Delete(context.Background(), key) })
	for _, payload := range []string{
		`{"schemaVersion":4,"userId":94002,"platformId":1,"platform":"admin","policyVersion":4,"version":3,"roleCodes":[],"menuTree":[],"permissionCodes":[],"unknown":true}`,
		`{"schemaVersion":4,"userId":999,"platformId":1,"platform":"admin","policyVersion":4,"version":3,"roleCodes":[],"menuTree":[],"permissionCodes":[]}`,
		`{"schemaVersion":4,"userId":94002,"platformId":2,"platform":"admin","policyVersion":4,"version":3,"roleCodes":[],"menuTree":[],"permissionCodes":[]}`,
		`{"schemaVersion":3,"userId":94002,"platformId":1,"platform":"admin","policyVersion":4,"version":3,"roleCodes":[],"menuTree":[],"permissionCodes":[]}`,
		`{"schemaVersion":4,"userId":94002,"platformId":1,"platform":"admin","policyVersion":4,"version":3,"roleCodes":[],"menuTree":[{"code":"account:user:list","menuType":"page","path":"/account/users","componentPath":"account/users","i18nKey":"navigation.accountUsers","icon":null,"isHidden":0,"children":[],"unexpected":true}],"permissionCodes":[]}`,
	} {
		if err := client.SetString(ctx, key, payload, time.Minute); err != nil {
			t.Fatal(err)
		}
		if _, found, err := cache.Read(ctx, 1, "admin", 4, 94002, 3); err == nil || !found {
			t.Fatalf("invalid cached snapshot accepted: %s", payload)
		}
	}
}

func openAccessRedis(t *testing.T) *projectredis.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("Redis integration test")
	}
	if err := godotenv.Load("../../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
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

func cleanupAccessKeys(t *testing.T, client *projectredis.Client, userID int64, platform string, policyVersion, version int64) {
	t.Helper()
	keys := []string{permissionstate.StateKey(userID), SnapshotKey(platform, policyVersion, userID, version)}
	if err := client.DeleteMany(context.Background(), keys); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), keys) })
}
