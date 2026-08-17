package main

import (
	"context"
	"strings"
	"testing"
)

func TestCheckWorkerRedisReturnsFullStartupContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := checkWorkerRedis(ctx, "redis://127.0.0.1:1/0")
	if err == nil {
		t.Fatal("expected Redis startup check to fail")
	}
	for _, want := range []string{"check Worker Redis", "ping Redis"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want %q context", err, want)
		}
	}
}
