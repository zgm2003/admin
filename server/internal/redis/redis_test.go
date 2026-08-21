package redis_test

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"admin/server/internal/config"
	projectredis "admin/server/internal/redis"
	"github.com/joho/godotenv"
)

func TestOpenHonorsCanceledContext(t *testing.T) {
	context, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := projectredis.Open(context, "redis://127.0.0.1:6379/0")
	if err == nil {
		t.Fatal("expected canceled Redis connection to fail")
	}
	if !strings.Contains(err.Error(), "ping Redis") {
		t.Fatalf("error = %q", err)
	}
}

func TestStringOperationsDistinguishMissingKeys(t *testing.T) {
	client := openIntegrationClient(t)
	ctx := context.Background()
	key := "test:redis:" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() { _ = client.Delete(context.Background(), key) })

	if value, found, err := client.GetString(ctx, key); err != nil || found || value != "" {
		t.Fatalf("missing GetString() = %q,%v,%v", value, found, err)
	}
	if err := client.SetString(ctx, key, "42", time.Minute); err != nil {
		t.Fatalf("SetString() error = %v", err)
	}
	if value, found, err := client.GetString(ctx, key); err != nil || !found || value != "42" {
		t.Fatalf("stored GetString() = %q,%v,%v", value, found, err)
	}
	if err := client.Delete(ctx, key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if value, found, err := client.GetString(ctx, key); err != nil || found || value != "" {
		t.Fatalf("deleted GetString() = %q,%v,%v", value, found, err)
	}
}

func TestAtomicStringOperations(t *testing.T) {
	client := openIntegrationClient(t)
	ctx := context.Background()
	key := "test:redis:atomic:" + strconv.FormatInt(time.Now().UnixNano(), 10)
	firstDeleteKey := key + ":delete:1"
	secondDeleteKey := key + ":delete:2"
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), []string{key, firstDeleteKey, secondDeleteKey}) })

	installed, err := client.SetStringIfMissing(ctx, key, `{"state":"ready"}`, time.Minute)
	if err != nil || !installed {
		t.Fatalf("first SetStringIfMissing = %v,%v", installed, err)
	}
	installed, err = client.SetStringIfMissing(ctx, key, "replacement", time.Minute)
	if err != nil || installed {
		t.Fatalf("second SetStringIfMissing = %v,%v", installed, err)
	}
	ttl, found, err := client.TTL(ctx, key)
	if err != nil || !found || ttl <= 0 || ttl > time.Minute {
		t.Fatalf("TTL = %v,%v,%v", ttl, found, err)
	}

	if err := client.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if ttl, found, err := client.TTL(ctx, key); err != nil || found || ttl != 0 {
		t.Fatalf("missing TTL = %v,%v,%v", ttl, found, err)
	}
	if err := client.SetString(ctx, key, "persistent", 0); err != nil {
		t.Fatal(err)
	}
	if ttl, found, err := client.TTL(ctx, key); err != nil || !found || ttl != -1 {
		t.Fatalf("persistent TTL = %v,%v,%v", ttl, found, err)
	}

	if err := client.SetString(ctx, firstDeleteKey, "first", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := client.SetString(ctx, secondDeleteKey, "second", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteMany(ctx, []string{firstDeleteKey, secondDeleteKey}); err != nil {
		t.Fatal(err)
	}
	for _, deletedKey := range []string{firstDeleteKey, secondDeleteKey} {
		if _, found, err := client.GetString(ctx, deletedKey); err != nil || found {
			t.Fatalf("deleted key %q found=%v err=%v", deletedKey, found, err)
		}
	}
	if err := client.DeleteMany(ctx, nil); err != nil {
		t.Fatalf("empty DeleteMany() error = %v", err)
	}
}

func TestEvalStringComparesMutationToken(t *testing.T) {
	client := openIntegrationClient(t)
	ctx := context.Background()
	key := "test:redis:eval:" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() { _ = client.Delete(context.Background(), key) })

	const invalidating = `{"state":"invalidating","mutationToken":"token-a"}`
	const ready = `{"state":"ready","mutationToken":null}`
	if err := client.SetString(ctx, key, invalidating, time.Minute); err != nil {
		t.Fatal(err)
	}

	const compareAndPublish = `
local current = redis.call('GET', KEYS[1])
if not current then
  return 'missing'
end
local decoded = cjson.decode(current)
if decoded.mutationToken ~= ARGV[1] then
  return 'token-mismatch'
end
redis.call('SET', KEYS[1], ARGV[2])
return 'published'
`
	result, err := client.EvalString(ctx, compareAndPublish, []string{key}, "token-b", ready)
	if err != nil || result != "token-mismatch" {
		t.Fatalf("wrong-token EvalString() = %q,%v", result, err)
	}
	if value, found, err := client.GetString(ctx, key); err != nil || !found || value != invalidating {
		t.Fatalf("payload after wrong token = %q,%v,%v", value, found, err)
	}

	result, err = client.EvalString(ctx, compareAndPublish, []string{key}, "token-a", ready)
	if err != nil || result != "published" {
		t.Fatalf("matching-token EvalString() = %q,%v", result, err)
	}
	if value, found, err := client.GetString(ctx, key); err != nil || !found || value != ready {
		t.Fatalf("published payload = %q,%v,%v", value, found, err)
	}
}

func TestScanDeleteRemovesOnlyMatchingLegacyKeys(t *testing.T) {
	client := openIntegrationClient(t)
	ctx := context.Background()
	testID := strconv.FormatInt(time.Now().UnixNano(), 10)
	prefix := "auth:current-session:test:" + testID + ":"
	matchingKeys := []string{prefix + "1", prefix + "2", prefix + "3"}
	outsideKey := "auth:current-session:outside:" + testID
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), append(matchingKeys, outsideKey)) })

	for _, key := range append(matchingKeys, outsideKey) {
		if err := client.SetString(ctx, key, "value", time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if err := client.ScanDelete(ctx, prefix+"*"); err != nil {
		t.Fatal(err)
	}
	for _, key := range matchingKeys {
		if _, found, err := client.GetString(ctx, key); err != nil || found {
			t.Fatalf("matching key %q found=%v err=%v", key, found, err)
		}
	}
	if value, found, err := client.GetString(ctx, outsideKey); err != nil || !found || value != "value" {
		t.Fatalf("outside key = %q,%v,%v", value, found, err)
	}
}

func openIntegrationClient(t *testing.T) *projectredis.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("Redis integration test")
	}
	if err := godotenv.Load("../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("load server .env: %v", err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatalf("load worker config: %v", err)
	}
	client, err := projectredis.Open(context.Background(), settings.RedisURL)
	if err != nil {
		t.Fatalf("open Redis: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}
