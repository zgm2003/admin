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

func TestRunForwardCleanupRemovesOnlyLegacyStorageAndSystemSettingKeys(t *testing.T) {
	ctx := context.Background()
	redisURL := isolatedRedisURL(t, loadIntegrationSettings(t).RedisURL)
	client := openIntegrationClient(t, redisURL)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)

	deleted := []string{
		"storage:cos-config:v1:test:" + suffix,
		"storage:upload-rule:v1:test:" + suffix,
		"storage:object-url:v1:test:" + suffix,
		"config-cache:state:v1:system.setting:global",
		"config-cache:snapshot:v1:system.setting:global:9:brand",
		"config-cache:fill:v1:system.setting:global:brand",
	}
	kept := []string{
		"storage:cos-config:v2:test:" + suffix,
		"config-cache:state:v1:storage.cosconfig:10",
		"config-cache:snapshot:v1:storage.cosconfig:10:runtime",
		"storage:object-route:v2:rule:20",
		"config-cache:state:v1:system.setting:other-scope",
		"session:test:" + suffix,
		"other:key:" + suffix,
	}
	seedKeys(t, ctx, client, append(append([]string{}, deleted...), kept...))

	if err := run(ctx, redisURL, modeForward); err != nil {
		t.Fatal(err)
	}
	assertKeysMissing(t, ctx, client, deleted)
	assertKeysPresent(t, ctx, client, kept)
}

func TestRunRollbackCleanupRemovesOnlyNewProtocolKeys(t *testing.T) {
	ctx := context.Background()
	redisURL := isolatedRedisURL(t, loadIntegrationSettings(t).RedisURL)
	client := openIntegrationClient(t, redisURL)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)

	deleted := []string{
		"config-cache:state:v1:storage.cosconfig:10",
		"config-cache:snapshot:v1:storage.cosconfig:10:runtime",
		"config-cache:fill:v1:storage.cosconfig:10:runtime",
		"storage:object-route:v2:rule:20",
		"config-cache:state:v1:system.setting:global",
		"config-cache:snapshot:v1:system.setting:global:9:brand",
		"config-cache:fill:v1:system.setting:global:brand",
	}
	kept := []string{
		"storage:cos-config:v1:test:" + suffix,
		"config-cache:state:v1:storage.other:1",
		"session:test:" + suffix,
		"other:key:" + suffix,
	}
	seedKeys(t, ctx, client, append(append([]string{}, deleted...), kept...))

	if err := run(ctx, redisURL, modeRollback); err != nil {
		t.Fatal(err)
	}
	assertKeysMissing(t, ctx, client, deleted)
	assertKeysPresent(t, ctx, client, kept)
}

func TestRunIsIdempotent(t *testing.T) {
	ctx := context.Background()
	redisURL := isolatedRedisURL(t, loadIntegrationSettings(t).RedisURL)
	if err := run(ctx, redisURL, modeForward); err != nil {
		t.Fatal(err)
	}
	if err := run(ctx, redisURL, modeForward); err != nil {
		t.Fatal(err)
	}
	if err := run(ctx, redisURL, modeRollback); err != nil {
		t.Fatal(err)
	}
}

func TestRunRejectsInvalidModeAndRedisURL(t *testing.T) {
	ctx := context.Background()
	redisURL := isolatedRedisURL(t, loadIntegrationSettings(t).RedisURL)
	if err := run(ctx, redisURL, "clean-everything"); err == nil {
		t.Fatal("run accepted an unknown mode")
	}
	if err := run(ctx, "not-a-redis-url", modeForward); err == nil {
		t.Fatal("run accepted an invalid Redis URL")
	}
}

func TestParseModeRejectsUnexpectedArguments(t *testing.T) {
	mode, err := parseMode(nil)
	if err != nil || mode != modeForward {
		t.Fatalf("default mode=%q err=%v", mode, err)
	}
	if _, err := parseMode([]string{"-mode=rollback"}); err != nil {
		t.Fatalf("rollback mode rejected: %v", err)
	}
	if _, err := parseMode([]string{"-mode=forward", "extra"}); err == nil {
		t.Fatal("positional arguments were accepted")
	}
	if _, err := parseMode([]string{"-mode=unknown"}); err == nil {
		t.Fatal("unknown mode was accepted")
	}
	if _, err := parseMode([]string{"-pattern=custom:*"}); err == nil {
		t.Fatal("custom pattern flag was accepted")
	}
}

func seedKeys(t *testing.T, ctx context.Context, client *projectredis.Client, keys []string) {
	t.Helper()
	t.Cleanup(func() {
		_ = client.DeleteMany(context.Background(), keys)
	})
	for _, key := range keys {
		if err := client.SetString(ctx, key, "value", time.Minute); err != nil {
			t.Fatal(err)
		}
	}
}

func assertKeysMissing(t *testing.T, ctx context.Context, client *projectredis.Client, keys []string) {
	t.Helper()
	for _, key := range keys {
		if _, found, err := client.GetString(ctx, key); err != nil || found {
			t.Fatalf("key %q found=%v err=%v", key, found, err)
		}
	}
}

func assertKeysPresent(t *testing.T, ctx context.Context, client *projectredis.Client, keys []string) {
	t.Helper()
	for _, key := range keys {
		if value, found, err := client.GetString(ctx, key); err != nil || !found || value != "value" {
			t.Fatalf("key %q value=%q found=%v err=%v", key, value, found, err)
		}
	}
}

func openIntegrationClient(t *testing.T, redisURL string) *projectredis.Client {
	t.Helper()
	client, err := projectredis.Open(context.Background(), redisURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// isolatedRedisURL 把集成 Redis 指向独立 DB 序号，避免清理命令在测试中删掉正在运行的 system.setting 缓存。
func isolatedRedisURL(t *testing.T, raw string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/1"
	return parsed.String()
}

// loadIntegrationSettings resolves the repository root instead of a hard-coded
// relative path, mirroring the existing PostgreSQL integration test pattern.
func loadIntegrationSettings(t *testing.T) config.Worker {
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
