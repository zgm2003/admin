package realtime

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"admin/server/internal/config"
	projectredis "admin/server/internal/redis"
	"github.com/joho/godotenv"
)

func TestTicketIssueUsesHashTTLAndConsumesOnce(t *testing.T) {
	client, _ := openRealtimeTicketRedis(t)
	store := NewTicketStore(client)
	now := time.Now().UTC()
	store.now = func() time.Time { return now }
	subject := TicketSubject{SchemaVersion: 1, PlatformID: 1, PlatformCode: "admin", UserID: 10, SessionID: 20, SessionVersion: 3, AccessExpiresAt: now.Add(12 * time.Second), IssuedAt: now}
	raw, err := store.Issue(context.Background(), subject)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 43 || strings.Contains(raw, "=") {
		t.Fatalf("ticket=%q", raw)
	}
	second, err := store.Issue(context.Background(), subject)
	if err != nil || second == raw {
		t.Fatalf("second=%q err=%v", second, err)
	}
	digest := sha256.Sum256([]byte(raw))
	key := "realtime:ticket:v1:" + hex.EncodeToString(digest[:])
	if _, found, err := client.GetString(context.Background(), key); err != nil || !found {
		t.Fatalf("hashed key found=%v err=%v", found, err)
	}
	if _, found, err := client.GetString(context.Background(), "realtime:ticket:v1:"+raw); err != nil || found {
		t.Fatalf("raw key found=%v err=%v", found, err)
	}
	ttl, found, err := client.TTL(context.Background(), key)
	if err != nil || !found || ttl <= 0 || ttl > 12*time.Second {
		t.Fatalf("ttl=%v found=%v err=%v", ttl, found, err)
	}
	got, err := store.Consume(context.Background(), raw)
	if err != nil || got != subject {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if _, err := store.Consume(context.Background(), raw); !errors.Is(err, ErrTicketUnavailable) {
		t.Fatalf("second consume=%v", err)
	}
}

func TestTicketRejectsInvalidExpiredAndCorruptValues(t *testing.T) {
	client, _ := openRealtimeTicketRedis(t)
	store := NewTicketStore(client)
	now := time.Now().UTC()
	store.now = func() time.Time { return now }
	base := TicketSubject{SchemaVersion: 1, PlatformID: 1, PlatformCode: "admin", UserID: 10, SessionID: 20, SessionVersion: 3, AccessExpiresAt: now.Add(time.Minute), IssuedAt: now}
	expired := base
	expired.AccessExpiresAt = now
	if _, err := store.Issue(context.Background(), expired); err == nil {
		t.Fatal("expired subject accepted")
	}
	for _, raw := range []string{"", "bad", "../../../secret", strings.Repeat("a", 44)} {
		if _, err := store.Consume(context.Background(), raw); err == nil || (raw != "" && strings.Contains(err.Error(), raw)) {
			t.Fatalf("raw=%q err=%v", raw, err)
		}
	}

	raw, err := store.Issue(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(raw))
	key := "realtime:ticket:v1:" + hex.EncodeToString(digest[:])
	payload, _ := json.Marshal(base)
	corrupt := strings.TrimSuffix(string(payload), "}") + `,"unknown":true}`
	if err := client.SetString(context.Background(), key, corrupt, time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Consume(context.Background(), raw); err == nil {
		t.Fatal("unknown field accepted")
	}

	closed, _ := openRealtimeTicketRedis(t)
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	failureStore := NewTicketStore(closed)
	validRaw := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	if _, err := failureStore.Consume(context.Background(), validRaw); err == nil || strings.Contains(err.Error(), validRaw) {
		t.Fatalf("Redis failure err=%v", err)
	}
}

func openRealtimeTicketRedis(t *testing.T) (*projectredis.Client, string) {
	t.Helper()
	raw := realtimeTicketRedisURL(t)
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/6"
	redisURL := parsed.String()
	client, err := projectredis.Open(context.Background(), redisURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client, redisURL
}

func realtimeTicketRedisURL(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", ".."))
	if err := godotenv.Load(filepath.Join(repoRoot, "server", ".env")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	return settings.RedisURL
}
