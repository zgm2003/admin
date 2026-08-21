package auth

import (
	"context"
	"os"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/module/authstate"
	projectredis "admin/server/internal/redis"
	"github.com/joho/godotenv"
)

func TestSessionKey(t *testing.T) {
	if got := SessionKey("admin", 11); got != "auth:session:admin:11" {
		t.Fatalf("SessionKey(admin, 11) = %q", got)
	}
}

func openAuthRedis(t *testing.T) *projectredis.Client {
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

func TestSessionCachePublishesOnlyAgainstMatchingReadyStates(t *testing.T) {
	client := openAuthRedis(t)
	states := authstate.NewStore(client)
	cache := NewSessionCache(client)
	ctx := context.Background()
	keys := []string{
		authstate.UserStateKey(73001), authstate.SessionsStateKey("admin", 73001), SessionKey("admin", 73002),
	}
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), keys) })
	_ = client.DeleteMany(ctx, keys)
	userFact := authstate.UserFact{UserID: 73001, Generation: "user-generation", IsEnabled: true}
	sessionsFact := authstate.SessionsFact{Platform: "admin", UserID: 73001, Generation: "sessions-generation"}
	_, _, _ = states.InstallUserReadyIfMissing(ctx, userFact)
	_, _, _ = states.InstallSessionsReadyIfMissing(ctx, sessionsFact)

	snapshot := SessionSnapshot{
		SchemaVersion: 1, UserID: 73001, SessionID: 73002, Platform: "admin", SessionVersion: 3,
		PolicyVersion: 4, UserGeneration: userFact.Generation, SessionsGeneration: sessionsFact.Generation,
		DeviceID: "550e8400-e29b-41d4-a716-446655440000", ClientIP: "127.0.0.1",
		RefreshExpiresAt: time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond), Revoked: false,
	}
	published, err := cache.PublishIfCurrent(ctx, snapshot, time.Minute)
	if err != nil || !published {
		t.Fatalf("PublishIfCurrent() = %v,%v", published, err)
	}
	read, found, err := cache.Read(ctx, "admin", 73002)
	if err != nil || !found || read != snapshot {
		t.Fatalf("Read() = %+v,%v,%v", read, found, err)
	}

	if err := client.Delete(ctx, authstate.SessionsStateKey("admin", 73001)); err != nil {
		t.Fatal(err)
	}
	_, _, _ = states.InstallSessionsReadyIfMissing(ctx, authstate.SessionsFact{Platform: "admin", UserID: 73001, Generation: "new-generation"})
	published, err = cache.PublishIfCurrent(ctx, snapshot, time.Minute)
	if err != nil || published {
		t.Fatalf("stale PublishIfCurrent() = %v,%v", published, err)
	}
}

func TestSessionCacheRejectsCorruptAndMismatchedSnapshots(t *testing.T) {
	client := openAuthRedis(t)
	cache := NewSessionCache(client)
	ctx := context.Background()
	key := SessionKey("admin", 73003)
	t.Cleanup(func() { _ = client.Delete(context.Background(), key) })
	payloads := []string{
		`{"schemaVersion":1,"userId":1,"sessionId":73003,"platform":"admin","sessionVersion":1,"policyVersion":1,"userGeneration":"u","sessionsGeneration":"s","deviceId":"550e8400-e29b-41d4-a716-446655440000","clientIp":"127.0.0.1","refreshExpiresAt":"2026-08-20T10:00:00Z","revoked":false,"unknown":true}`,
		`{"schemaVersion":1,"userId":1,"sessionId":999,"platform":"admin","sessionVersion":1,"policyVersion":1,"userGeneration":"u","sessionsGeneration":"s","deviceId":"550e8400-e29b-41d4-a716-446655440000","clientIp":"127.0.0.1","refreshExpiresAt":"2026-08-20T10:00:00Z","revoked":false}`,
	}
	for _, payload := range payloads {
		if err := client.SetString(ctx, key, payload, time.Minute); err != nil {
			t.Fatal(err)
		}
		if _, found, err := cache.Read(ctx, "admin", 73003); err == nil || !found {
			t.Fatalf("invalid snapshot accepted: %s", payload)
		}
	}
}
