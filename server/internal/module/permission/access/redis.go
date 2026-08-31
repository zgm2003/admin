package permission

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"admin/server/internal/module/auth/client"
	"admin/server/internal/module/permission/state"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/yesno"
)

const accessSnapshotSchemaVersion = 4

type CachedSnapshot struct {
	SchemaVersion   int        `json:"schemaVersion"`
	UserID          int64      `json:"userId"`
	PlatformID      int64      `json:"platformId"`
	Platform        string     `json:"platform"`
	PolicyVersion   int64      `json:"policyVersion"`
	Version         int64      `json:"version"`
	RoleCodes       []string   `json:"roleCodes"`
	MenuTree        []MenuNode `json:"menuTree"`
	PermissionCodes []string   `json:"permissionCodes"`
}

type SnapshotCache struct {
	redis *projectredis.Client
}

func NewSnapshotCache(redis *projectredis.Client) *SnapshotCache {
	return &SnapshotCache{redis: redis}
}

func SnapshotKey(platform string, policyVersion, userID, version int64) string {
	return "authz:permission:" + platform + ":" + strconv.FormatInt(policyVersion, 10) + ":" +
		strconv.FormatInt(userID, 10) + ":" + strconv.FormatInt(version, 10)
}

func (c *SnapshotCache) Read(ctx context.Context, platformID int64, platform string, policyVersion, userID, version int64) (CachedSnapshot, bool, error) {
	key := SnapshotKey(platform, policyVersion, userID, version)
	raw, found, err := c.redis.GetString(ctx, key)
	if err != nil || !found {
		return CachedSnapshot{}, found, err
	}
	snapshot, err := decodeCachedSnapshot(raw)
	if err != nil {
		return CachedSnapshot{}, true, err
	}
	if snapshot.PlatformID != platformID || snapshot.Platform != platform || snapshot.PolicyVersion != policyVersion || snapshot.UserID != userID || snapshot.Version != version {
		return CachedSnapshot{}, true, fmt.Errorf("cached access snapshot identity is invalid")
	}
	return snapshot, true, nil
}

func (c *SnapshotCache) PublishIfCurrent(ctx context.Context, snapshot CachedSnapshot, ttl time.Duration) (bool, error) {
	if err := validateCachedSnapshot(snapshot); err != nil {
		return false, err
	}
	if ttl <= 0 {
		return false, fmt.Errorf("access snapshot TTL must be positive")
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return false, fmt.Errorf("encode access snapshot: %w", err)
	}
	result, err := c.redis.EvalString(ctx, publishPermissionSnapshotScript, []string{
		SnapshotKey(snapshot.Platform, snapshot.PolicyVersion, snapshot.UserID, snapshot.Version),
		permissionstate.StateKey(snapshot.UserID),
	}, string(payload), snapshot.Version, ttl.Milliseconds())
	if err != nil {
		return false, err
	}
	switch result {
	case "published":
		return true, nil
	case "missing", "updating", "changed":
		return false, nil
	default:
		return false, fmt.Errorf("publish access snapshot returned %q", result)
	}
}

func newCachedSnapshot(userID, platformID int64, platform string, policyVersion int64, snapshot Snapshot) CachedSnapshot {
	return CachedSnapshot{
		SchemaVersion: accessSnapshotSchemaVersion, UserID: userID, PlatformID: platformID, Platform: platform,
		PolicyVersion: policyVersion, Version: snapshot.Version,
		RoleCodes: append([]string(nil), snapshot.RoleCodes...), MenuTree: cloneMenuTree(snapshot.MenuTree),
		PermissionCodes: append([]string(nil), snapshot.PermissionCodes...),
	}
}

func snapshotFromCache(cached CachedSnapshot, cacheResult string) Snapshot {
	return Snapshot{
		RoleCodes: append([]string(nil), cached.RoleCodes...), MenuTree: cloneMenuTree(cached.MenuTree),
		PermissionCodes: append([]string(nil), cached.PermissionCodes...), Version: cached.Version, CacheResult: cacheResult,
	}
}

func cachedSnapshot(userID, platformID int64, platform string, policyVersion int64, snapshot Snapshot) CachedSnapshot {
	return newCachedSnapshot(userID, platformID, platform, policyVersion, snapshot)
}

func decodeCachedSnapshot(raw string) (CachedSnapshot, error) {
	expected := []string{"schemaVersion", "userId", "platformId", "platform", "policyVersion", "version", "roleCodes", "menuTree", "permissionCodes"}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return CachedSnapshot{}, fmt.Errorf("decode access snapshot fields: %w", err)
	}
	if len(fields) != len(expected) {
		return CachedSnapshot{}, fmt.Errorf("access snapshot JSON field set is invalid")
	}
	for _, key := range expected {
		if _, ok := fields[key]; !ok {
			return CachedSnapshot{}, fmt.Errorf("access snapshot field %q is missing", key)
		}
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	var snapshot CachedSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return CachedSnapshot{}, fmt.Errorf("decode access snapshot: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return CachedSnapshot{}, fmt.Errorf("access snapshot has trailing data")
	}
	if err := validateCachedSnapshot(snapshot); err != nil {
		return CachedSnapshot{}, err
	}
	return snapshot, nil
}

func validateCachedSnapshot(snapshot CachedSnapshot) error {
	if snapshot.SchemaVersion != accessSnapshotSchemaVersion || snapshot.UserID < 1 || snapshot.PlatformID < 1 || snapshot.PolicyVersion < 1 || snapshot.Version < 1 {
		return fmt.Errorf("cached access snapshot identity is invalid")
	}
	if authclient.ValidatePlatform(snapshot.Platform) != nil {
		return fmt.Errorf("cached access snapshot platform is invalid")
	}
	if snapshot.RoleCodes == nil || snapshot.PermissionCodes == nil || snapshot.MenuTree == nil {
		return fmt.Errorf("cached access snapshot arrays must not be null")
	}
	if err := validateSortedUniqueStrings(snapshot.RoleCodes, "role code"); err != nil {
		return err
	}
	if err := validateSortedUniqueStrings(snapshot.PermissionCodes, "permission code"); err != nil {
		return err
	}
	codes := make(map[string]struct{})
	paths := make(map[string]struct{})
	if err := validateCachedMenuNodes(snapshot.MenuTree, codes, paths); err != nil {
		return err
	}
	return nil
}

func validateSortedUniqueStrings(values []string, name string) error {
	for index, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("cached access snapshot has an empty %s", name)
		}
		if index > 0 && values[index-1] >= value {
			return fmt.Errorf("cached access snapshot %ss are not sorted unique", name)
		}
	}
	return nil
}

func validateCachedMenuNodes(nodes []MenuNode, codes, paths map[string]struct{}) error {
	for _, node := range nodes {
		if node.Children == nil || strings.TrimSpace(node.Code) == "" || !validAccessI18nKey(node.I18nKey) ||
			!yesno.IsValid(yesno.Value(node.IsHidden)) || (node.Icon != nil && !validAccessIcon(*node.Icon)) {
			return fmt.Errorf("cached access menu node is invalid")
		}
		if _, exists := codes[node.Code]; exists {
			return fmt.Errorf("cached access menu code %q is duplicated", node.Code)
		}
		codes[node.Code] = struct{}{}
		switch node.MenuType {
		case MenuDirectory:
			if node.Path != nil || node.ComponentPath != nil {
				return fmt.Errorf("cached directory menu has render fields")
			}
		case MenuPage:
			if node.Path == nil || node.ComponentPath == nil || !validAccessPath(*node.Path) || !validAccessComponentPath(*node.ComponentPath) {
				return fmt.Errorf("cached page menu is incomplete")
			}
			if _, exists := paths[*node.Path]; exists {
				return fmt.Errorf("cached page path %q is duplicated", *node.Path)
			}
			paths[*node.Path] = struct{}{}
		default:
			return fmt.Errorf("cached menu tree contains invalid type %q", node.MenuType)
		}
		if err := validateCachedMenuNodes(node.Children, codes, paths); err != nil {
			return err
		}
	}
	return nil
}

func cloneMenuTree(nodes []MenuNode) []MenuNode {
	result := make([]MenuNode, 0, len(nodes))
	for _, node := range nodes {
		copyNode := node
		copyNode.Children = cloneMenuTree(node.Children)
		result = append(result, copyNode)
	}
	return result
}

func sortUniqueStrings(values []string) ([]string, error) {
	result := append(make([]string, 0, len(values)), values...)
	sort.Strings(result)
	deduplicated := result[:0]
	for _, value := range result {
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("access string value is empty")
		}
		if len(deduplicated) == 0 || deduplicated[len(deduplicated)-1] != value {
			deduplicated = append(deduplicated, value)
		}
	}
	return deduplicated, nil
}

const publishPermissionSnapshotScript = `
local state = redis.call('GET', KEYS[2])
if not state then return 'missing' end
local decoded = cjson.decode(state)
if decoded.state == 'invalidating' then return 'updating' end
if decoded.state ~= 'ready' or decoded.version ~= tonumber(ARGV[2]) then return 'changed' end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[3])
return 'published'
`
