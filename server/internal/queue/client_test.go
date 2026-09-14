package queue_test

import (
	"strings"
	"testing"

	"admin/server/internal/queue"
)

func TestNewClientRejectsNonRedisURL(t *testing.T) {
	_, err := queue.NewClient("http://127.0.0.1:6379")
	if err == nil {
		t.Fatal("expected invalid Redis URL to fail")
	}
	if !strings.Contains(err.Error(), "Redis URL") {
		t.Fatalf("error = %q", err)
	}
}

func TestNewServerRejectsNonRedisURL(t *testing.T) {
	_, err := queue.NewServer("http://127.0.0.1:6379")
	if err == nil {
		t.Fatal("expected invalid Redis URL to fail")
	}
	if !strings.Contains(err.Error(), "Redis URL") {
		t.Fatalf("error = %q", err)
	}
}

func TestRedisConnOptParsesRedisURL(t *testing.T) {
	option, err := queue.RedisConnOpt("redis://:secret@127.0.0.1:6379/3")
	if err != nil {
		t.Fatal(err)
	}
	if option == nil {
		t.Fatal("RedisConnOpt() returned nil")
	}
}
