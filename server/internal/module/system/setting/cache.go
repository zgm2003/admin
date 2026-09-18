package setting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/cacheFill"
	"admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/yesno"
)

const (
	cachePayloadSchemaVersion = 1
	cacheSnapshotTTL          = 5 * time.Minute
	brandSnapshotVariant      = "brand"
)

// ErrSnapshotCorrupt 表示快照 payload 无法按协议解析或坐标不匹配；
// 调用方不得直接返回损坏内容，必须进入有界 repair。
var ErrSnapshotCorrupt = errors.New("setting snapshot is corrupt")

// Cache 只访问 Redis：state 读取、带 generation 的不可变快照与 cacheFill 准入。
type Cache struct {
	client *projectredis.Client
	store  *cachegeneration.Store
	scope  cachegeneration.Scope
	ttl    time.Duration
}

func NewCache(client *projectredis.Client) *Cache {
	if client == nil {
		return nil
	}
	return &Cache{client: client, scope: settingGenerationScope, ttl: cacheSnapshotTTL}
}

// SetStateStore 由显式装配注入只读 state store。
func (c *Cache) SetStateStore(store *cachegeneration.Store) {
	c.store = store
}

func (c *Cache) ValidateDependencies() error {
	if c == nil || c.client == nil || c.store == nil {
		return fmt.Errorf("setting cache state store is not configured")
	}
	return c.scope.Validate()
}

func (c *Cache) ReadState(ctx context.Context) (cachegeneration.State, bool, error) {
	if c == nil || c.store == nil {
		return cachegeneration.State{}, false, fmt.Errorf("setting cache state store is not configured")
	}
	return c.store.Read(ctx, c.scope)
}

func (c *Cache) GetRecord(ctx context.Context, generation int64, key string) (Record, bool, error) {
	if c == nil || c.client == nil {
		return Record{}, false, fmt.Errorf("setting cache is not configured")
	}
	snapshotKey, err := cachegeneration.SnapshotKey(c.scope, generation, recordVariant(key))
	if err != nil {
		return Record{}, false, err
	}
	raw, found, err := c.client.GetString(ctx, snapshotKey)
	if err != nil || !found {
		return Record{}, false, err
	}
	payload, err := decodeRecordPayload(raw)
	if err != nil {
		return Record{}, false, err
	}
	if payload.Generation != generation || payload.Key != key {
		return Record{}, false, fmt.Errorf("%w: snapshot coordinates mismatch", ErrSnapshotCorrupt)
	}
	return payload.record(), true, nil
}

func (c *Cache) PutRecord(ctx context.Context, generation int64, row Record) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("setting cache is not configured")
	}
	payload, err := encodeRecordPayload(generation, row)
	if err != nil {
		return err
	}
	snapshotKey, err := cachegeneration.SnapshotKey(c.scope, generation, recordVariant(row.Key))
	if err != nil {
		return err
	}
	return c.client.SetString(ctx, snapshotKey, payload, c.ttl)
}

func (c *Cache) GetBrand(ctx context.Context, generation int64) (BrandSettings, bool, error) {
	if c == nil || c.client == nil {
		return BrandSettings{}, false, fmt.Errorf("setting cache is not configured")
	}
	snapshotKey, err := cachegeneration.SnapshotKey(c.scope, generation, brandSnapshotVariant)
	if err != nil {
		return BrandSettings{}, false, err
	}
	raw, found, err := c.client.GetString(ctx, snapshotKey)
	if err != nil || !found {
		return BrandSettings{}, false, err
	}
	payload, err := decodeBrandPayload(raw)
	if err != nil {
		return BrandSettings{}, false, err
	}
	if payload.Generation != generation {
		return BrandSettings{}, false, fmt.Errorf("%w: snapshot coordinates mismatch", ErrSnapshotCorrupt)
	}
	return BrandSettings{TitleZhCN: payload.TitleZhCN, TitleEnUS: payload.TitleEnUS, DefaultAvatar: payload.DefaultAvatar}, true, nil
}

func (c *Cache) PutBrand(ctx context.Context, generation int64, brand BrandSettings) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("setting cache is not configured")
	}
	payload, err := encodeBrandPayload(generation, brand)
	if err != nil {
		return err
	}
	snapshotKey, err := cachegeneration.SnapshotKey(c.scope, generation, brandSnapshotVariant)
	if err != nil {
		return err
	}
	return c.client.SetString(ctx, snapshotKey, payload, c.ttl)
}

// TryFill 为 (scope, generation, variant) 申请跨实例单 leader 有界回源准入。
func (c *Cache) TryFill(ctx context.Context, generation int64, variant string) (*cachefill.Lease, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("setting cache is not configured")
	}
	target, err := cachegeneration.FillKey(c.scope, generation, variant)
	if err != nil {
		return nil, err
	}
	return cachefill.Try(ctx, c.client.UniversalClient(), cachegeneration.StateKey(c.scope), target)
}

// TryRepair 为 state missing/corrupt 的权威修复申请跨实例单 leader 准入。
func (c *Cache) TryRepair(ctx context.Context, variant string) (*cachefill.Lease, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("setting cache is not configured")
	}
	return cachefill.Try(ctx, c.client.UniversalClient(), cachegeneration.StateKey(c.scope), "repair:"+variant)
}

func recordVariant(key string) string { return "setting:" + key }

type recordPayload struct {
	SchemaVersion int         `json:"schemaVersion"`
	Generation    int64       `json:"generation"`
	ID            int64       `json:"id"`
	Key           string      `json:"key"`
	Value         string      `json:"value"`
	ValueType     int         `json:"valueType"`
	Description   string      `json:"description"`
	IsEnabled     yesno.Value `json:"isEnabled"`
	IsBuiltin     yesno.Value `json:"isBuiltin"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
}

func (p recordPayload) record() Record {
	return Record{
		ID: p.ID, Key: p.Key, Value: p.Value, ValueType: p.ValueType, Description: p.Description,
		IsEnabled: p.IsEnabled, IsBuiltin: p.IsBuiltin, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func encodeRecordPayload(generation int64, row Record) (string, error) {
	if generation < 1 || row.Key == "" {
		return "", fmt.Errorf("setting snapshot payload is invalid")
	}
	payload, err := json.Marshal(recordPayload{
		SchemaVersion: cachePayloadSchemaVersion, Generation: generation,
		ID: row.ID, Key: row.Key, Value: row.Value, ValueType: row.ValueType, Description: row.Description,
		IsEnabled: row.IsEnabled, IsBuiltin: row.IsBuiltin, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	if err != nil {
		return "", fmt.Errorf("encode setting snapshot: %w", err)
	}
	return string(payload), nil
}

func decodeRecordPayload(raw string) (recordPayload, error) {
	var payload recordPayload
	fields, err := decodeStrictPayload(raw, &payload)
	if err != nil {
		return recordPayload{}, err
	}
	if len(fields) != 11 {
		return recordPayload{}, fmt.Errorf("%w: setting snapshot field set is invalid", ErrSnapshotCorrupt)
	}
	if payload.SchemaVersion != cachePayloadSchemaVersion {
		return recordPayload{}, fmt.Errorf("%w: setting snapshot schema version is invalid", ErrSnapshotCorrupt)
	}
	if payload.Generation < 1 || payload.Key == "" {
		return recordPayload{}, fmt.Errorf("%w: setting snapshot coordinates are invalid", ErrSnapshotCorrupt)
	}
	if payload.ValueType < ValueTypeString || payload.ValueType > ValueTypeJSON {
		return recordPayload{}, fmt.Errorf("%w: setting snapshot value type is invalid", ErrSnapshotCorrupt)
	}
	if !yesno.IsValid(payload.IsEnabled) || !yesno.IsValid(payload.IsBuiltin) {
		return recordPayload{}, fmt.Errorf("%w: setting snapshot flags are invalid", ErrSnapshotCorrupt)
	}
	return payload, nil
}

type brandPayload struct {
	SchemaVersion int    `json:"schemaVersion"`
	Generation    int64  `json:"generation"`
	TitleZhCN     string `json:"titleZhCN"`
	TitleEnUS     string `json:"titleEnUS"`
	DefaultAvatar string `json:"defaultAvatar"`
}

func encodeBrandPayload(generation int64, brand BrandSettings) (string, error) {
	if generation < 1 {
		return "", fmt.Errorf("setting brand snapshot payload is invalid")
	}
	payload, err := json.Marshal(brandPayload{
		SchemaVersion: cachePayloadSchemaVersion, Generation: generation,
		TitleZhCN: brand.TitleZhCN, TitleEnUS: brand.TitleEnUS, DefaultAvatar: brand.DefaultAvatar,
	})
	if err != nil {
		return "", fmt.Errorf("encode setting brand snapshot: %w", err)
	}
	return string(payload), nil
}

func decodeBrandPayload(raw string) (brandPayload, error) {
	var payload brandPayload
	fields, err := decodeStrictPayload(raw, &payload)
	if err != nil {
		return brandPayload{}, err
	}
	if len(fields) != 5 {
		return brandPayload{}, fmt.Errorf("%w: setting brand snapshot field set is invalid", ErrSnapshotCorrupt)
	}
	if payload.SchemaVersion != cachePayloadSchemaVersion {
		return brandPayload{}, fmt.Errorf("%w: setting brand snapshot schema version is invalid", ErrSnapshotCorrupt)
	}
	if payload.Generation < 1 {
		return brandPayload{}, fmt.Errorf("%w: setting brand snapshot coordinates are invalid", ErrSnapshotCorrupt)
	}
	return payload, nil
}

// decodeStrictPayload 拒绝未知字段、重复字段与尾随内容。
func decodeStrictPayload(raw string, target any) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("%w: decode setting snapshot: %v", ErrSnapshotCorrupt, err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return nil, fmt.Errorf("%w: setting snapshot must be a JSON object", ErrSnapshotCorrupt)
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("%w: decode setting snapshot field: %v", ErrSnapshotCorrupt, err)
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, fmt.Errorf("%w: setting snapshot field name is invalid", ErrSnapshotCorrupt)
		}
		if _, exists := fields[key]; exists {
			return nil, fmt.Errorf("%w: setting snapshot field %q is duplicated", ErrSnapshotCorrupt, key)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("%w: decode setting snapshot field %q: %v", ErrSnapshotCorrupt, key, err)
		}
		fields[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("%w: decode setting snapshot: %v", ErrSnapshotCorrupt, err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("%w: setting snapshot has trailing data", ErrSnapshotCorrupt)
	}

	strict := json.NewDecoder(strings.NewReader(raw))
	strict.DisallowUnknownFields()
	if err := strict.Decode(target); err != nil {
		return nil, fmt.Errorf("%w: decode setting snapshot: %v", ErrSnapshotCorrupt, err)
	}
	return fields, nil
}
