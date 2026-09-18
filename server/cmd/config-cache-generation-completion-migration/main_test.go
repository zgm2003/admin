package main

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"admin/server/internal/config"
	projectredis "admin/server/internal/redis"
	"github.com/joho/godotenv"
)

func TestRunForwardCleanupRemovesOnlyRetiredConfigurationCacheKeys(t *testing.T) {
	ctx := context.Background()
	redisURL := completionIsolatedRedisURL(t, loadCompletionIntegrationSettings(t).RedisURL)
	client := openCompletionIntegrationClient(t, redisURL)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	deleted := []string{
		"system:dictionary:generation:v1",
		"system:dictionary:mutation:v1",
		"system:dictionary:options:v1:" + suffix,
		"mail:runtime:v2:" + suffix,
		"mail:runtime:generation:v2",
		"mail:runtime:mutation:v2",
		"mail:runtime:load-lock:v2",
		"mail:verify-code-readiness:v3:" + suffix,
		"mail:verify-code-readiness:load-lock:v3:" + suffix,
		"mail:rate-limit:policies:v3:" + suffix,
		"mail:rate-limit:policies:load-lock:v3:" + suffix,
		"sms:runtime:generation:v1:" + suffix,
		"sms:runtime:mutation:v1:" + suffix,
		"sms:runtime:v1:" + suffix,
		"sms:readiness:v1:" + suffix,
		"sms:rate-limit:policies:v1:" + suffix,
		"sms:rate-limit:policies:load-lock:v1:" + suffix,
		"sms:rate-limit:policies:mutation:v1:" + suffix,
	}
	kept := []string{
		"config-cache:state:v1:system.dictionary:global",
		"config-cache:snapshot:v1:message.mail:global:2:runtime:login",
		"config-cache:state:v1:system.setting:global",
		"config-cache:snapshot:v1:storage.cosconfig:10:2:runtime",
		"permission:access:state:v1:10",
		"auth:session:admin:" + suffix,
		"captcha:slide:" + suffix,
		"mail:rate-limit:counter:" + suffix,
		"sms:rate-limit:counter:" + suffix,
	}
	seedCompletionKeys(t, ctx, client, append(append([]string{}, deleted...), kept...))

	if err := run(ctx, redisURL, modeForward); err != nil {
		t.Fatal(err)
	}
	assertCompletionKeysMissing(t, ctx, client, deleted)
	assertCompletionKeysPresent(t, ctx, client, kept)
}

func TestRunRollbackCleanupRemovesOnlyNewStateAndSnapshots(t *testing.T) {
	ctx := context.Background()
	redisURL := completionIsolatedRedisURL(t, loadCompletionIntegrationSettings(t).RedisURL)
	client := openCompletionIntegrationClient(t, redisURL)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	deleted := []string{
		"config-cache:state:v1:system.dictionary:global",
		"config-cache:snapshot:v1:system.dictionary:global:2:options:abc",
		"config-cache:state:v1:message.mail:global",
		"config-cache:snapshot:v1:message.mail:global:2:runtime:login",
		"config-cache:state:v1:message.sms:global",
		"config-cache:snapshot:v1:message.sms:global:2:readiness:login",
	}
	kept := []string{
		"config-cache:fill:v1:system.dictionary:global:2:options:abc",
		"config-cache:fill:v1:message.mail:global:2:runtime:login",
		"config-cache:fill:v1:message.sms:global:2:readiness:login",
		"config-cache:state:v1:system.setting:global",
		"config-cache:state:v1:storage.cosconfig:10",
		"storage:object-route:v2:rule:20",
		"permission:access:state:v1:10",
		"auth:session:admin:" + suffix,
		"captcha:slide:" + suffix,
		"sms:rate-limit:counter:" + suffix,
	}
	seedCompletionKeys(t, ctx, client, append(append([]string{}, deleted...), kept...))

	if err := run(ctx, redisURL, modeRollback); err != nil {
		t.Fatal(err)
	}
	assertCompletionKeysMissing(t, ctx, client, deleted)
	assertCompletionKeysPresent(t, ctx, client, kept)
}

func TestRunCleanupIsIdempotent(t *testing.T) {
	ctx := context.Background()
	redisURL := completionIsolatedRedisURL(t, loadCompletionIntegrationSettings(t).RedisURL)
	for _, mode := range []string{modeForward, modeForward, modeRollback, modeRollback} {
		if err := run(ctx, redisURL, mode); err != nil {
			t.Fatalf("run mode %s: %v", mode, err)
		}
	}
}

func TestRunCleanupRejectsInvalidModeURLAndCustomPattern(t *testing.T) {
	ctx := context.Background()
	redisURL := completionIsolatedRedisURL(t, loadCompletionIntegrationSettings(t).RedisURL)
	if err := run(ctx, redisURL, "everything"); err == nil {
		t.Fatal("run accepted an unknown mode")
	}
	if err := run(ctx, "not-a-redis-url", modeForward); err == nil {
		t.Fatal("run accepted an invalid Redis URL")
	}
	if _, err := parseMode([]string{"-pattern=custom:*"}); err == nil {
		t.Fatal("custom pattern flag was accepted")
	}
	if _, err := parseMode([]string{"-mode=forward", "extra"}); err == nil {
		t.Fatal("positional arguments were accepted")
	}
}

func seedCompletionKeys(t *testing.T, ctx context.Context, client *projectredis.Client, keys []string) {
	t.Helper()
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), keys) })
	for _, key := range keys {
		if err := client.SetString(ctx, key, "value", time.Minute); err != nil {
			t.Fatal(err)
		}
	}
}

func assertCompletionKeysMissing(t *testing.T, ctx context.Context, client *projectredis.Client, keys []string) {
	t.Helper()
	for _, key := range keys {
		if _, found, err := client.GetString(ctx, key); err != nil || found {
			t.Fatalf("key %q found=%v err=%v", key, found, err)
		}
	}
}

func assertCompletionKeysPresent(t *testing.T, ctx context.Context, client *projectredis.Client, keys []string) {
	t.Helper()
	for _, key := range keys {
		if value, found, err := client.GetString(ctx, key); err != nil || !found || value != "value" {
			t.Fatalf("key %q value=%q found=%v err=%v", key, value, found, err)
		}
	}
}

func openCompletionIntegrationClient(t *testing.T, redisURL string) *projectredis.Client {
	t.Helper()
	client, err := projectredis.Open(context.Background(), redisURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func completionIsolatedRedisURL(t *testing.T, raw string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/2"
	return parsed.String()
}

func loadCompletionIntegrationSettings(t *testing.T) config.Worker {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test")
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	if err := godotenv.Load(filepath.Join(repoRoot, "server", ".env")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	return settings
}
