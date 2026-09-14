package queuemonitor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type grantStore interface {
	Put(context.Context, string, string, time.Duration) error
	Get(context.Context, string) (string, bool, error)
}

type redisGrantStore struct{ client redisGrantClient }

func NewRedisGrantStore(client redisGrantClient) grantStore {
	if client == nil {
		return nil
	}
	return &redisGrantStore{client: client}
}

func (s *redisGrantStore) Put(ctx context.Context, key, value string, ttl time.Duration) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("queue monitor grant Redis client is unavailable")
	}
	return s.client.SetString(ctx, key, value, ttl)
}

func (s *redisGrantStore) Get(ctx context.Context, key string) (string, bool, error) {
	if s == nil || s.client == nil {
		return "", false, fmt.Errorf("queue monitor grant Redis client is unavailable")
	}
	return s.client.GetString(ctx, key)
}

func grantKey(credential string) string {
	sum := sha256.Sum256([]byte(credential))
	return grantKeyPrefix + hex.EncodeToString(sum[:])
}
