package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Client struct {
	client *goredis.Client
}

func Open(ctx context.Context, redisURL string) (*Client, error) {
	options, err := goredis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse Redis URL: %w", err)
	}

	client := goredis.NewClient(options)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping Redis: %w", err)
	}
	return &Client{client: client}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *Client) GetString(ctx context.Context, key string) (string, bool, error) {
	value, err := c.client.Get(ctx, key).Result()
	if errors.Is(err, goredis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get Redis key: %w", err)
	}
	return value, true, nil
}

func (c *Client) SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	if err := c.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("set Redis key: %w", err)
	}
	return nil
}

func (c *Client) SetStringIfMissing(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	installed, err := c.client.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("set Redis key if missing: %w", err)
	}
	return installed, nil
}

func (c *Client) TTL(ctx context.Context, key string) (time.Duration, bool, error) {
	ttl, err := c.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, false, fmt.Errorf("get Redis key TTL: %w", err)
	}
	if ttl == -2 {
		return 0, false, nil
	}
	return ttl, true, nil
}

func (c *Client) EvalString(ctx context.Context, script string, keys []string, args ...any) (string, error) {
	value, err := c.client.Eval(ctx, script, keys, args...).Text()
	if err != nil {
		return "", fmt.Errorf("evaluate Redis script: %w", err)
	}
	return value, nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("delete Redis key: %w", err)
	}
	return nil
}

func (c *Client) DeleteMany(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	if _, err := c.client.Del(ctx, keys...).Result(); err != nil {
		return fmt.Errorf("delete Redis keys: %w", err)
	}
	return nil
}

func (c *Client) ScanDelete(ctx context.Context, pattern string) error {
	var cursor uint64
	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("scan Redis keys: %w", err)
		}
		if err := c.DeleteMany(ctx, keys); err != nil {
			return err
		}
		if nextCursor == 0 {
			return nil
		}
		cursor = nextCursor
	}
}

func (c *Client) Close() error {
	return c.client.Close()
}
