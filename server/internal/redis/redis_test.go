package redis_test

import (
	"context"
	"strings"
	"testing"

	projectredis "admin/server/internal/redis"
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
