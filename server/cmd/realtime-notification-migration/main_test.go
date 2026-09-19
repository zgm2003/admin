package main

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"testing"
	"time"

	"admin/server/internal/config"
	projectredis "admin/server/internal/redis"
	"github.com/joho/godotenv"
)

var expectedRealtimeNotificationCleanupPatterns = []string{
	"config-cache:state:v1:system.setting:global",
	"config-cache:snapshot:v1:system.setting:global:*",
	"config-cache:fill:v1:system.setting:global:*",
	"authz:permission-state:v3:*",
	"authz:menu-state:v1:*",
	"authz:permission:v8:*",
	"realtime:ticket:v1:*",
}

func TestCleanupPatternsAreFixedForBothModes(t *testing.T) {
	for _, mode := range []string{modeForward, modeRollback} {
		patterns, err := cleanupPatterns(mode)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(patterns, expectedRealtimeNotificationCleanupPatterns) {
			t.Fatalf("mode=%s patterns=%v", mode, patterns)
		}
	}
}

func TestParseModeRejectsCustomPatternsAndArguments(t *testing.T) {
	for _, args := range [][]string{{"-pattern=custom:*"}, {"-mode=other"}, {"-mode=forward", "extra"}} {
		if _, err := parseMode(args); err == nil {
			t.Fatalf("parseMode(%v) succeeded", args)
		}
	}
}

func TestRunCleanupDeletesOnlyFixedPatternsAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	raw := loadRealtimeNotificationRedisURL(t)
	redisURL := isolatedRealtimeNotificationRedisURL(t, raw)
	client, err := projectredis.Open(ctx, redisURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	deleted := []string{
		"config-cache:state:v1:system.setting:global",
		"config-cache:snapshot:v1:system.setting:global:2:all",
		"config-cache:fill:v1:system.setting:global:2:all",
		"authz:permission-state:v3:" + suffix,
		"authz:menu-state:v1:" + suffix,
		"authz:permission:v8:" + suffix,
		"realtime:ticket:v1:" + suffix,
	}
	kept := []string{"auth:session:admin:" + suffix, "message:notification:fact:" + suffix, "realtime:user:v1:1:" + suffix}
	all := append(append([]string{}, deleted...), kept...)
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), all) })
	for _, key := range all {
		if err := client.SetString(ctx, key, "value", time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	for _, mode := range []string{modeForward, modeForward, modeRollback, modeRollback} {
		if err := run(ctx, redisURL, mode); err != nil {
			t.Fatalf("run %s: %v", mode, err)
		}
	}
	for _, key := range deleted {
		if _, found, err := client.GetString(ctx, key); err != nil || found {
			t.Fatalf("deleted key %q found=%v err=%v", key, found, err)
		}
	}
	for _, key := range kept {
		if value, found, err := client.GetString(ctx, key); err != nil || !found || value != "value" {
			t.Fatalf("kept key %q value=%q found=%v err=%v", key, value, found, err)
		}
	}
}

func isolatedRealtimeNotificationRedisURL(t *testing.T, raw string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/5"
	return parsed.String()
}

func loadRealtimeNotificationRedisURL(t *testing.T) string {
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
	return settings.RedisURL
}
