package authstate

import (
	"context"
	"os"
	"testing"

	"admin/server/internal/config"
	projectredis "admin/server/internal/redis"
	"github.com/joho/godotenv"
)

func TestStoreInstallsAndReadsClosedReadyStates(t *testing.T) {
	client := openAuthenticationStateRedis(t)
	store := NewStore(client)
	ctx := context.Background()
	userKey := UserStateKey(71001)
	sessionsKey := SessionsStateKey("admin", 71001)
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), []string{userKey, sessionsKey}) })
	_ = client.DeleteMany(ctx, []string{userKey, sessionsKey})

	userFact := UserFact{UserID: 71001, Generation: "user-generation", IsEnabled: true}
	state, installed, err := store.InstallUserReadyIfMissing(ctx, userFact)
	if err != nil || !installed || state.State != StateReady || state.UserID != userFact.UserID || state.Generation != userFact.Generation {
		t.Fatalf("InstallUserReadyIfMissing() = %+v,%v,%v", state, installed, err)
	}
	actual, installed, err := store.InstallUserReadyIfMissing(ctx, UserFact{UserID: 71001, Generation: "other", Deleted: true})
	if err != nil || installed || actual.Generation != userFact.Generation || actual.Deleted {
		t.Fatalf("second install = %+v,%v,%v", actual, installed, err)
	}

	sessionsFact := SessionsFact{Platform: "admin", UserID: 71001, Generation: "sessions-generation"}
	sessions, installed, err := store.InstallSessionsReadyIfMissing(ctx, sessionsFact)
	if err != nil || !installed || sessions.State != StateReady || sessions.Platform != "admin" {
		t.Fatalf("InstallSessionsReadyIfMissing() = %+v,%v,%v", sessions, installed, err)
	}
	read, found, err := store.ReadSessions(ctx, "admin", 71001)
	if err != nil || !found || read.Generation != sessionsFact.Generation {
		t.Fatalf("ReadSessions() = %+v,%v,%v", read, found, err)
	}
}

func TestStoreRejectsUnknownAndMismatchedStateFields(t *testing.T) {
	client := openAuthenticationStateRedis(t)
	store := NewStore(client)
	ctx := context.Background()
	key := UserStateKey(71002)
	t.Cleanup(func() { _ = client.Delete(context.Background(), key) })

	invalidPayloads := []string{
		`{"schemaVersion":1,"state":"ready","userId":71002,"generation":"g","isEnabled":true,"deleted":false,"mutationToken":null,"unknown":true}`,
		`{"schemaVersion":1,"state":"ready","userId":999,"generation":"g","isEnabled":true,"deleted":false,"mutationToken":null}`,
		`{"schemaVersion":1,"state":"ready","userId":71002,"generation":"","isEnabled":true,"deleted":false,"mutationToken":null}`,
	}
	for _, payload := range invalidPayloads {
		if err := client.SetString(ctx, key, payload, 0); err != nil {
			t.Fatal(err)
		}
		if _, found, err := store.ReadUser(ctx, 71002); err == nil || !found {
			t.Fatalf("invalid payload accepted: %s", payload)
		}
	}
}

func openAuthenticationStateRedis(t *testing.T) *projectredis.Client {
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
