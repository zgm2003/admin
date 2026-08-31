package accessstate

import (
	"context"
	"errors"
	"os"
	"testing"

	"admin/server/internal/config"
	projectredis "admin/server/internal/redis"
	"github.com/joho/godotenv"
)

func TestStoreInstallsAndStrictlyReadsReadyState(t *testing.T) {
	client := openAccessStateRedis(t)
	store := NewStore(client)
	ctx := context.Background()
	key := StateKey(91001)
	t.Cleanup(func() { _ = client.Delete(context.Background(), key) })
	_ = client.Delete(ctx, key)

	state, installed, err := store.InstallReadyIfMissing(ctx, Version{UserID: 91001, Version: 3})
	if err != nil || !installed || state.State != StateReady || state.Version != 3 {
		t.Fatalf("InstallReadyIfMissing() = %+v,%v,%v", state, installed, err)
	}
	actual, installed, err := store.InstallReadyIfMissing(ctx, Version{UserID: 91001, Version: 4})
	if err != nil || installed || actual.Version != 3 {
		t.Fatalf("second install = %+v,%v,%v", actual, installed, err)
	}

	if err := client.SetString(ctx, key, `{"schemaVersion":2,"state":"ready","version":3,"unknown":true}`, 0); err != nil {
		t.Fatal(err)
	}
	if _, found, err := store.Read(ctx, 91001); err == nil || !found {
		t.Fatal("unknown access state field was accepted")
	}
}

func TestStoreRebuildReadyStateReplacesStaleVersion(t *testing.T) {
	client := openAccessStateRedis(t)
	store := NewStore(client)
	ctx := context.Background()
	key := StateKey(91002)
	t.Cleanup(func() { _ = client.Delete(context.Background(), key) })
	_ = client.Delete(ctx, key)
	if err := client.SetString(ctx, key, `{"schemaVersion":2,"state":"ready","version":3}`, 0); err != nil {
		t.Fatal(err)
	}
	if err := store.RebuildReadyState(ctx, []Version{{UserID: 91002, Version: 6}}); err != nil {
		t.Fatalf("RebuildReadyState() error = %v", err)
	}
	state, found, err := store.Read(ctx, 91002)
	if err != nil || !found || state.Version != 6 || state.State != StateReady {
		t.Fatalf("rebuilt state = %+v found=%v err=%v", state, found, err)
	}
}

func TestStoreRebuildReadyStateRejectsActiveMutation(t *testing.T) {
	client := openAccessStateRedis(t)
	store := NewStore(client)
	ctx := context.Background()
	key := StateKey(91003)
	t.Cleanup(func() { _ = client.Delete(context.Background(), key) })
	_ = client.Delete(ctx, key)
	if err := client.SetString(ctx, key, `{"schemaVersion":2,"state":"invalidating","version":0,"mutationToken":"token","baseVersion":3}`, 0); err != nil {
		t.Fatal(err)
	}
	if err := store.RebuildReadyState(ctx, []Version{{UserID: 91003, Version: 6}}); !errors.Is(err, ErrUpdating) {
		t.Fatalf("RebuildReadyState() error = %v, want ErrUpdating", err)
	}
}

func openAccessStateRedis(t *testing.T) *projectredis.Client {
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
