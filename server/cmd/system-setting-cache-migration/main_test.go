package main

import (
	"context"
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

func TestRunRemovesOnlyLegacySystemSettingCacheKeys(t *testing.T) {
	settings := loadIntegrationSettings(t)
	ctx := context.Background()
	client := openIntegrationClient(t, settings.RedisURL)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	legacyKeys := []string{
		"system:setting:v1:test:" + suffix + ":a",
		"system:setting:v1:test:" + suffix + ":b",
	}
	outsideKeys := []string{
		"system:setting:v2:test:" + suffix,
		"other:cache:test:" + suffix,
	}
	t.Cleanup(func() {
		_ = client.DeleteMany(context.Background(), append(append([]string{}, legacyKeys...), outsideKeys...))
	})

	for _, key := range append(append([]string{}, legacyKeys...), outsideKeys...) {
		if err := client.SetString(ctx, key, "value", time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if err := run(ctx, settings.RedisURL); err != nil {
		t.Fatal(err)
	}
	for _, key := range legacyKeys {
		if _, found, err := client.GetString(ctx, key); err != nil || found {
			t.Fatalf("legacy key %q found=%v err=%v", key, found, err)
		}
	}
	for _, key := range outsideKeys {
		if value, found, err := client.GetString(ctx, key); err != nil || !found || value != "value" {
			t.Fatalf("outside key %q = %q,%v,%v", key, value, found, err)
		}
	}
}

func TestRunIsIdempotent(t *testing.T) {
	settings := loadIntegrationSettings(t)
	ctx := context.Background()
	if err := run(ctx, settings.RedisURL); err != nil {
		t.Fatal(err)
	}
	if err := run(ctx, settings.RedisURL); err != nil {
		t.Fatal(err)
	}
}

func TestRunRejectsInvalidRedisURL(t *testing.T) {
	if err := run(context.Background(), "not-a-redis-url"); err == nil {
		t.Fatal("run accepted an invalid Redis URL")
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
