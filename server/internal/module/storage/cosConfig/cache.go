package cosconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/cacheFill"
	"admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/yesno"
)

const (
	runtimePayloadSchemaVersion = 1
	runtimeSnapshotTTL          = 5 * time.Minute
	runtimeSnapshotVariant      = "runtime"
)

// ErrSnapshotCorrupt 表示 runtime 快照无法按协议解析或坐标不匹配：调用方必须进入有界 repair，不得直接返回损坏内容。
var ErrSnapshotCorrupt = errors.New("COS config snapshot is corrupt")

// ErrGenerationAdvanced 表示回源期间 generation 已变化：丢弃结果并重试完整读流程。
var ErrGenerationAdvanced = errors.New("COS config cache generation advanced during fill")

// Cache 只访问 Redis：state 读取、generation 不可变快照与 cacheFill 准入。
type Cache struct {
	client *projectredis.Client
	store  *cachegeneration.Store
	ttl    time.Duration
}

func NewCache(client *projectredis.Client) *Cache {
	if client == nil {
		return nil
	}
	return &Cache{client: client, ttl: runtimeSnapshotTTL}
}

// SetStateStore 由显式装配注入只读 state store。
func (c *Cache) SetStateStore(store *cachegeneration.Store) {
	c.store = store
}

func (c *Cache) ReadState(ctx context.Context, id int64) (cachegeneration.State, bool, error) {
	if c == nil || c.store == nil {
		return cachegeneration.State{}, false, fmt.Errorf("COS config cache state store is not configured")
	}
	scope, err := runtimeScope(id)
	if err != nil {
		return cachegeneration.State{}, false, err
	}
	return c.store.Read(ctx, scope)
}

func (c *Cache) GetRuntime(ctx context.Context, id, generation int64) (RuntimeConfig, bool, error) {
	if c == nil || c.client == nil {
		return RuntimeConfig{}, false, fmt.Errorf("COS config cache is not configured")
	}
	scope, err := runtimeScope(id)
	if err != nil {
		return RuntimeConfig{}, false, err
	}
	snapshotKey, err := cachegeneration.SnapshotKey(scope, generation, runtimeSnapshotVariant)
	if err != nil {
		return RuntimeConfig{}, false, err
	}
	raw, found, err := c.client.GetString(ctx, snapshotKey)
	if err != nil || !found {
		return RuntimeConfig{}, false, err
	}
	payload, err := decodeRuntimePayload(raw)
	if err != nil {
		return RuntimeConfig{}, false, err
	}
	if payload.Generation != generation || payload.ConfigID != id {
		return RuntimeConfig{}, false, fmt.Errorf("%w: runtime snapshot coordinates mismatch", ErrSnapshotCorrupt)
	}
	return payload.runtime(), true, nil
}

func (c *Cache) PutRuntime(ctx context.Context, id, generation int64, config RuntimeConfig) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("COS config cache is not configured")
	}
	payload, err := encodeRuntimePayload(generation, id, config)
	if err != nil {
		return err
	}
	scope, err := runtimeScope(id)
	if err != nil {
		return err
	}
	snapshotKey, err := cachegeneration.SnapshotKey(scope, generation, runtimeSnapshotVariant)
	if err != nil {
		return err
	}
	return c.client.SetString(ctx, snapshotKey, payload, c.ttl)
}

// TryFill 为 (scope, generation, runtime) 申请跨实例单 leader 有界回源准入。
func (c *Cache) TryFill(ctx context.Context, id, generation int64) (*cachefill.Lease, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("COS config cache is not configured")
	}
	scope, err := runtimeScope(id)
	if err != nil {
		return nil, err
	}
	target, err := cachegeneration.FillKey(scope, generation, runtimeSnapshotVariant)
	if err != nil {
		return nil, err
	}
	return cachefill.Try(ctx, c.client.UniversalClient(), cachegeneration.StateKey(scope), target)
}

// TryRepair 为 state missing/corrupt 的权威修复申请跨实例单 leader 准入。
func (c *Cache) TryRepair(ctx context.Context, id int64) (*cachefill.Lease, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("COS config cache is not configured")
	}
	scope, err := runtimeScope(id)
	if err != nil {
		return nil, err
	}
	return cachefill.Try(ctx, c.client.UniversalClient(), cachegeneration.StateKey(scope), "repair:"+strconv.FormatInt(id, 10))
}

func runtimeScope(id int64) (cachegeneration.Scope, error) {
	if id < 1 {
		return cachegeneration.Scope{}, fmt.Errorf("COS config id is invalid")
	}
	return cachegeneration.NewScope(cosConfigGenerationNamespace, strconv.FormatInt(id, 10))
}

type runtimeVersionPayload struct {
	Version      int64   `json:"version"`
	Bucket       string  `json:"bucket"`
	Region       string  `json:"region"`
	Endpoint     *string `json:"endpoint"`
	BucketDomain *string `json:"bucketDomain"`
}

type runtimePayload struct {
	SchemaVersion       int                     `json:"schemaVersion"`
	Generation          int64                   `json:"generation"`
	ConfigID            int64                   `json:"configId"`
	AppID               string                  `json:"appId"`
	SecretIDCiphertext  string                  `json:"secretIdCiphertext"`
	SecretKeyCiphertext string                  `json:"secretKeyCiphertext"`
	CurrentVersion      int64                   `json:"currentVersion"`
	IsEnabled           yesno.Value             `json:"isEnabled"`
	Deleted             bool                    `json:"deleted"`
	Versions            []runtimeVersionPayload `json:"versions"`
}

func (p runtimePayload) runtime() RuntimeConfig {
	versions := make([]RuntimeVersion, 0, len(p.Versions))
	for _, version := range p.Versions {
		versions = append(versions, RuntimeVersion{Version: version.Version, Bucket: version.Bucket, Region: version.Region, Endpoint: copiedString(version.Endpoint), BucketDomain: copiedString(version.BucketDomain)})
	}
	return RuntimeConfig{
		ID: p.ConfigID, AppID: p.AppID, SecretIDCiphertext: p.SecretIDCiphertext, SecretKeyCiphertext: p.SecretKeyCiphertext,
		CurrentVersion: p.CurrentVersion, IsEnabled: p.IsEnabled, Deleted: p.Deleted, Versions: versions,
	}
}

func encodeRuntimePayload(generation, id int64, config RuntimeConfig) (string, error) {
	if generation < 1 || id < 1 || config.ID != id || config.CurrentVersion < 1 ||
		strings.TrimSpace(config.AppID) == "" || strings.TrimSpace(config.SecretIDCiphertext) == "" || strings.TrimSpace(config.SecretKeyCiphertext) == "" ||
		!yesno.IsValid(config.IsEnabled) || len(config.Versions) == 0 {
		return "", fmt.Errorf("COS config runtime payload is invalid")
	}
	versions := make([]runtimeVersionPayload, 0, len(config.Versions))
	expected := int64(1)
	for _, version := range config.Versions {
		if version.Version != expected || strings.TrimSpace(version.Bucket) == "" || strings.TrimSpace(version.Region) == "" {
			return "", fmt.Errorf("COS config runtime payload versions are invalid")
		}
		expected++
		versions = append(versions, runtimeVersionPayload{Version: version.Version, Bucket: version.Bucket, Region: version.Region, Endpoint: copiedString(version.Endpoint), BucketDomain: copiedString(version.BucketDomain)})
	}
	if expected-1 != config.CurrentVersion {
		return "", fmt.Errorf("COS config runtime payload current version is invalid")
	}
	payload, err := json.Marshal(runtimePayload{
		SchemaVersion: runtimePayloadSchemaVersion, Generation: generation, ConfigID: id,
		AppID: config.AppID, SecretIDCiphertext: config.SecretIDCiphertext, SecretKeyCiphertext: config.SecretKeyCiphertext,
		CurrentVersion: config.CurrentVersion, IsEnabled: config.IsEnabled, Deleted: config.Deleted, Versions: versions,
	})
	if err != nil {
		return "", fmt.Errorf("encode COS config runtime snapshot: %w", err)
	}
	return string(payload), nil
}

func decodeRuntimePayload(raw string) (runtimePayload, error) {
	var payload runtimePayload
	fields, err := decodeStrictRuntimePayload(raw, &payload)
	if err != nil {
		return runtimePayload{}, err
	}
	if len(fields) != 10 {
		return runtimePayload{}, fmt.Errorf("%w: runtime snapshot field set is invalid", ErrSnapshotCorrupt)
	}
	if payload.SchemaVersion != runtimePayloadSchemaVersion {
		return runtimePayload{}, fmt.Errorf("%w: runtime snapshot schema version is invalid", ErrSnapshotCorrupt)
	}
	if payload.Generation < 1 || payload.ConfigID < 1 || payload.CurrentVersion < 1 {
		return runtimePayload{}, fmt.Errorf("%w: runtime snapshot coordinates are invalid", ErrSnapshotCorrupt)
	}
	if strings.TrimSpace(payload.AppID) == "" || strings.TrimSpace(payload.SecretIDCiphertext) == "" || strings.TrimSpace(payload.SecretKeyCiphertext) == "" {
		return runtimePayload{}, fmt.Errorf("%w: runtime snapshot credentials are invalid", ErrSnapshotCorrupt)
	}
	if !yesno.IsValid(payload.IsEnabled) {
		return runtimePayload{}, fmt.Errorf("%w: runtime snapshot flags are invalid", ErrSnapshotCorrupt)
	}
	if len(payload.Versions) == 0 {
		return runtimePayload{}, fmt.Errorf("%w: runtime snapshot versions are missing", ErrSnapshotCorrupt)
	}
	expected := int64(1)
	for _, version := range payload.Versions {
		if version.Version != expected || strings.TrimSpace(version.Bucket) == "" || strings.TrimSpace(version.Region) == "" {
			return runtimePayload{}, fmt.Errorf("%w: runtime snapshot version rows are invalid", ErrSnapshotCorrupt)
		}
		expected++
	}
	if expected-1 != payload.CurrentVersion {
		return runtimePayload{}, fmt.Errorf("%w: runtime snapshot current version is missing", ErrSnapshotCorrupt)
	}
	return payload, nil
}

// decodeStrictRuntimePayload 拒绝未知字段、重复字段与尾随内容；每个模块保留自己的 codec，不共享"万能"校验器。
func decodeStrictRuntimePayload(raw string, target any) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("%w: decode runtime snapshot: %v", ErrSnapshotCorrupt, err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return nil, fmt.Errorf("%w: runtime snapshot must be a JSON object", ErrSnapshotCorrupt)
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("%w: decode runtime snapshot field: %v", ErrSnapshotCorrupt, err)
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, fmt.Errorf("%w: runtime snapshot field name is invalid", ErrSnapshotCorrupt)
		}
		if _, exists := fields[key]; exists {
			return nil, fmt.Errorf("%w: runtime snapshot field %q is duplicated", ErrSnapshotCorrupt, key)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("%w: decode runtime snapshot field %q: %v", ErrSnapshotCorrupt, key, err)
		}
		fields[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("%w: decode runtime snapshot: %v", ErrSnapshotCorrupt, err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("%w: runtime snapshot has trailing data", ErrSnapshotCorrupt)
	}
	strict := json.NewDecoder(strings.NewReader(raw))
	strict.DisallowUnknownFields()
	if err := strict.Decode(target); err != nil {
		return nil, fmt.Errorf("%w: decode runtime snapshot: %v", ErrSnapshotCorrupt, err)
	}
	return fields, nil
}

func copiedString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
