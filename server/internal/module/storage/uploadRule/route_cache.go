package uploadrule

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
	"gorm.io/gorm"
)

const (
	// routeSnapshotSchemaVersion 与严格字段集共同构成 rule route 快照协议。
	routeSnapshotSchemaVersion = 1
	// routeSnapshotTTL 只是快照存活上限：route 字段创建后不可变，因此没有 generation、outbox 或 mutation lease。
	routeSnapshotTTL = 24 * time.Hour
	// routeCacheKeyPrefix 是固定 key pattern：storage:object-route:v2:rule:<ruleId>。
	routeCacheKeyPrefix = "storage:object-route:v2:rule:"

	// routeReadBudget 是热读失败闭合上限：followers 最多等待约 500ms。
	routeReadBudgetDefault = 500 * time.Millisecond
	routeReadWaitStep      = 20 * time.Millisecond
)

// ErrRouteSnapshotCorrupt 表示快照无法按协议解析或坐标不匹配：调用方必须进入单 leader 覆盖回源，不得把损坏内容返回给调用方。
var ErrRouteSnapshotCorrupt = errors.New("upload rule route snapshot is corrupt")

// errRouteFillDeferred 表示另一实例正持有回源准入：调用方有界等待后重试完整读流程。
var errRouteFillDeferred = errors.New("upload rule route fill is owned by another instance")

// ObjectRoute 是创建后不可变的物理坐标与访问模式；不含可编辑的名称、编码与启停状态。
type ObjectRoute struct {
	RuleID      int64
	PlatformID  int64
	CosConfigID int64
	AccessMode  string
}

// RouteCache 只访问 Redis 中的单键快照。
type RouteCache struct {
	client *projectredis.Client
	ttl    time.Duration
}

func NewRouteCache(client *projectredis.Client) *RouteCache {
	if client == nil {
		return nil
	}
	return &RouteCache{client: client, ttl: routeSnapshotTTL}
}

func routeCacheKey(ruleID int64) (string, error) {
	if ruleID < 1 {
		return "", fmt.Errorf("upload rule id is invalid")
	}
	return routeCacheKeyPrefix + strconv.FormatInt(ruleID, 10), nil
}

func (c *RouteCache) GetRoute(ctx context.Context, ruleID int64) (ObjectRoute, bool, error) {
	if c == nil || c.client == nil {
		return ObjectRoute{}, false, fmt.Errorf("upload rule route cache is not configured")
	}
	key, err := routeCacheKey(ruleID)
	if err != nil {
		return ObjectRoute{}, false, err
	}
	raw, found, err := c.client.GetString(ctx, key)
	if err != nil || !found {
		return ObjectRoute{}, false, err
	}
	payload, err := decodeRoutePayload(raw)
	if err != nil {
		return ObjectRoute{}, false, err
	}
	if payload.RuleID != ruleID {
		return ObjectRoute{}, false, fmt.Errorf("%w: rule route snapshot coordinates mismatch", ErrRouteSnapshotCorrupt)
	}
	return ObjectRoute{RuleID: payload.RuleID, PlatformID: payload.PlatformID, CosConfigID: payload.CosConfigID, AccessMode: payload.AccessMode}, true, nil
}

func (c *RouteCache) PutRoute(ctx context.Context, route ObjectRoute) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("upload rule route cache is not configured")
	}
	key, err := routeCacheKey(route.RuleID)
	if err != nil {
		return err
	}
	payload, err := encodeRoutePayload(route)
	if err != nil {
		return err
	}
	return c.client.SetString(ctx, key, payload, c.ttl)
}

// TryFill 为固定 key 申请跨实例单 leader 有界回源准入；miss 与损坏共用同一准入。
func (c *RouteCache) TryFill(ctx context.Context, ruleID int64) (*cachefill.Lease, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("upload rule route cache is not configured")
	}
	key, err := routeCacheKey(ruleID)
	if err != nil {
		return nil, err
	}
	return cachefill.Try(ctx, c.client.UniversalClient(), key, key+":fill")
}

type routePayload struct {
	SchemaVersion int    `json:"schemaVersion"`
	RuleID        int64  `json:"ruleId"`
	PlatformID    int64  `json:"platformId"`
	CosConfigID   int64  `json:"cosConfigId"`
	AccessMode    string `json:"accessMode"`
}

func (p routePayload) route() ObjectRoute {
	return ObjectRoute{RuleID: p.RuleID, PlatformID: p.PlatformID, CosConfigID: p.CosConfigID, AccessMode: p.AccessMode}
}

func validObjectRoute(route ObjectRoute) bool {
	return route.RuleID >= 1 && route.PlatformID >= 1 && route.CosConfigID >= 1 &&
		(route.AccessMode == "public" || route.AccessMode == "private")
}

func encodeRoutePayload(route ObjectRoute) (string, error) {
	if !validObjectRoute(route) {
		return "", fmt.Errorf("upload rule route payload is invalid")
	}
	payload, err := json.Marshal(routePayload{
		SchemaVersion: routeSnapshotSchemaVersion, RuleID: route.RuleID, PlatformID: route.PlatformID,
		CosConfigID: route.CosConfigID, AccessMode: route.AccessMode,
	})
	if err != nil {
		return "", fmt.Errorf("encode upload rule route snapshot: %w", err)
	}
	return string(payload), nil
}

func decodeRoutePayload(raw string) (routePayload, error) {
	var payload routePayload
	fields, err := decodeStrictRoutePayload(raw, &payload)
	if err != nil {
		return routePayload{}, err
	}
	if len(fields) != 5 {
		return routePayload{}, fmt.Errorf("%w: route snapshot field set is invalid", ErrRouteSnapshotCorrupt)
	}
	if payload.SchemaVersion != routeSnapshotSchemaVersion {
		return routePayload{}, fmt.Errorf("%w: route snapshot schema version is invalid", ErrRouteSnapshotCorrupt)
	}
	if !validObjectRoute(payload.route()) {
		return routePayload{}, fmt.Errorf("%w: route snapshot coordinates are invalid", ErrRouteSnapshotCorrupt)
	}
	return payload, nil
}

// decodeStrictRoutePayload 拒绝未知字段、重复字段与尾随内容；每个模块保留自己的 codec，不共享"万能"校验器。
func decodeStrictRoutePayload(raw string, target any) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("%w: decode route snapshot: %v", ErrRouteSnapshotCorrupt, err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return nil, fmt.Errorf("%w: route snapshot must be a JSON object", ErrRouteSnapshotCorrupt)
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("%w: decode route snapshot field: %v", ErrRouteSnapshotCorrupt, err)
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, fmt.Errorf("%w: route snapshot field name is invalid", ErrRouteSnapshotCorrupt)
		}
		if _, exists := fields[key]; exists {
			return nil, fmt.Errorf("%w: route snapshot field %q is duplicated", ErrRouteSnapshotCorrupt, key)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("%w: decode route snapshot field %q: %v", ErrRouteSnapshotCorrupt, key, err)
		}
		fields[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("%w: decode route snapshot: %v", ErrRouteSnapshotCorrupt, err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("%w: route snapshot has trailing data", ErrRouteSnapshotCorrupt)
	}
	strict := json.NewDecoder(strings.NewReader(raw))
	strict.DisallowUnknownFields()
	if err := strict.Decode(target); err != nil {
		return nil, fmt.Errorf("%w: decode route snapshot: %v", ErrRouteSnapshotCorrupt, err)
	}
	return fields, nil
}

// routeSource 是 route 的权威读取契约；*Repository 直接满足它，测试可注入可控事实。
type routeSource interface {
	RuleRoute(context.Context, int64) (ObjectRoute, error)
}

// loadRoute 是历史对象解析的热读：命中零 PostgreSQL；miss/损坏由单 leader 回源覆盖；
// Redis 故障与 500ms 预算耗尽一律 fail closed，不用 PostgreSQL 充当每请求兜底。
func (s *Service) loadRoute(ctx context.Context, ruleID int64) (ObjectRoute, error) {
	if s == nil || s.routeCache == nil || s.routeSource == nil {
		return ObjectRoute{}, dependency(fmt.Errorf("upload rule route read requires cache and repository"))
	}
	deadline := s.now().Add(s.routeReadBudget)
	for s.now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return ObjectRoute{}, dependency(err)
		}
		route, found, err := s.routeCache.GetRoute(ctx, ruleID)
		if err != nil && !errors.Is(err, ErrRouteSnapshotCorrupt) {
			// Redis 请求本身失败：不访问 PostgreSQL。
			return ObjectRoute{}, dependency(err)
		}
		if err == nil && found {
			return route, nil
		}
		if fillErr := s.fillRoute(ctx, ruleID); fillErr != nil {
			if errors.Is(fillErr, errRouteFillDeferred) {
				continue
			}
			return ObjectRoute{}, fillErr
		}
	}
	return ObjectRoute{}, dependency(fmt.Errorf("upload rule route read budget exhausted"))
}

func (s *Service) fillRoute(ctx context.Context, ruleID int64) error {
	lease, err := s.routeCache.TryFill(ctx, ruleID)
	if err != nil {
		return dependency(err)
	}
	if lease == nil {
		// 另一实例正在回源：有界等待后重试完整读流程。
		if waitErr := s.wait(ctx, s.routeWaitStep); waitErr != nil {
			return dependency(waitErr)
		}
		return errRouteFillDeferred
	}
	defer func() { _ = lease.Release(ctx) }()
	workCtx, cancel := lease.WorkContext(ctx)
	defer cancel()
	if _, found, err := s.routeCache.GetRoute(workCtx, ruleID); err != nil && !errors.Is(err, ErrRouteSnapshotCorrupt) {
		return dependency(err)
	} else if found {
		return nil
	}

	route, err := s.routeSource.RuleRoute(workCtx, ruleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return notFound(err)
		}
		return dependency(err)
	}
	if err := s.routeCache.PutRoute(workCtx, route); err != nil {
		return dependency(err)
	}
	return nil
}

func defaultWait(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
