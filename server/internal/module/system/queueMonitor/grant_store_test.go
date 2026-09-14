package queuemonitor

import (
	"context"
	"errors"
	"testing"
	"time"
)

type redisGrantClientStub struct {
	key, value string
	ttl        time.Duration
	setErr     error
	getValue   string
	found      bool
	getErr     error
}

func (s *redisGrantClientStub) SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.key, s.value, s.ttl = key, value, ttl
	return s.setErr
}

func (s *redisGrantClientStub) GetString(ctx context.Context, key string) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	s.key = key
	return s.getValue, s.found, s.getErr
}

func TestRedisGrantStorePassesContextAndTTL(t *testing.T) {
	client := &redisGrantClientStub{}
	store := NewRedisGrantStore(client)
	if err := store.Put(context.Background(), "key", "value", GrantTTL); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if client.key != "key" || client.value != "value" || client.ttl != GrantTTL {
		t.Fatalf("recorded = %q %q %v", client.key, client.value, client.ttl)
	}
}

func TestRedisGrantStorePreservesMissingAndErrors(t *testing.T) {
	client := &redisGrantClientStub{}
	store := NewRedisGrantStore(client)
	if _, found, err := store.Get(context.Background(), "key"); err != nil || found {
		t.Fatalf("Get missing = %v, %v", found, err)
	}

	client.getErr = errors.New("redis unavailable")
	if _, _, err := store.Get(context.Background(), "key"); err == nil {
		t.Fatal("Get() accepted dependency failure")
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Put(cancelled, "key", "value", time.Minute); !errors.Is(err, context.Canceled) {
		t.Fatalf("Put cancelled error = %v", err)
	}
}
