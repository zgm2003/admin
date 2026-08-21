package accessstate

import (
	"context"
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

	if err := client.SetString(ctx, key, `{"schemaVersion":1,"state":"ready","version":3,"unknown":true}`, 0); err != nil {
		t.Fatal(err)
	}
	if _, found, err := store.Read(ctx, 91001); err == nil || !found {
		t.Fatal("unknown access state field was accepted")
	}
}

func openAccessStateRedis(t *testing.T) *projectredis.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("Redis integration test")
	}
	if err := godotenv.Load("../../../.env"); err != nil && !os.IsNotExist(err) {
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
