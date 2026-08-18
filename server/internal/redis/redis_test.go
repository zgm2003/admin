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
