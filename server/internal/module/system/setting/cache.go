package setting

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	projectredis "admin/server/internal/redis"
)

type Cache struct {
	client *projectredis.Client
	ttl    time.Duration
}

func NewCache(client *projectredis.Client) *Cache {
	if client == nil {
		return nil
	}
	return &Cache{client: client, ttl: 5 * time.Minute}
}

func (c *Cache) key(settingKey string) string { return "system:setting:v1:" + settingKey }

func (c *Cache) Get(ctx context.Context, settingKey string) (Record, bool, error) {
	if c == nil || c.client == nil {
		return Record{}, false, nil
	}
	value, found, err := c.client.GetString(ctx, c.key(settingKey))
	if err != nil || !found {
		return Record{}, found, err
	}
	row, err := decodeRecord(value)
	if err != nil {
		return Record{}, false, fmt.Errorf("decode system setting cache: %w", err)
	}
	return row, true, nil
}

func decodeRecord(value string) (Record, error) {
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.DisallowUnknownFields()
	var row Record
	if err := decoder.Decode(&row); err != nil {
		return Record{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Record{}, fmt.Errorf("trailing data")
		}
		return Record{}, err
	}
	return row, nil
}

func (c *Cache) Set(ctx context.Context, row Record) error {
	if c == nil || c.client == nil {
		return nil
	}
	value, err := json.Marshal(row)
	if err != nil {
		return err
	}
	return c.client.SetString(ctx, c.key(row.Key), string(value), c.ttl)
}

func (c *Cache) Delete(ctx context.Context, settingKey string) error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Delete(ctx, c.key(settingKey))
}
