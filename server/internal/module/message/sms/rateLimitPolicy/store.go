package rateLimitPolicy

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	projectredis "admin/server/internal/redis"
	"golang.org/x/sync/singleflight"
)

const (
	snapshotSchemaVersion = 1
	snapshotPrefix        = "sms:rate-limit:policies:v1:"
	loadLockPrefix        = "sms:rate-limit:policies:load-lock:v1:"
	mutationPrefix        = "sms:rate-limit:policies:mutation:v1:"
	loadLockTTL           = 5 * time.Second
	mutationLeaseTTL      = 6 * time.Second
	snapshotTTL           = 10 * time.Minute
)

type snapshotPolicy struct {
	Limit         int   `json:"limit"`
	WindowSeconds int   `json:"windowSeconds"`
	Revision      int64 `json:"revision"`
}

type policySnapshot struct {
	SchemaVersion int                       `json:"schemaVersion"`
	PlatformID    int64                     `json:"platformId"`
	PlatformCode  string                    `json:"platformCode"`
	PlatformName  string                    `json:"platformName"`
	Policies      map[string]snapshotPolicy `json:"policies"`
}

// RedisStore caches one platform catalog per key and invalidates it with a
// tokenized mutation lease. PostgreSQL remains the source of truth.
type RedisStore struct {
	redis *projectredis.Client
	group singleflight.Group
}

func NewStore(redis *projectredis.Client) *RedisStore { return &RedisStore{redis: redis} }

func snapshotKey(platformID int64) string {
	return fmt.Sprintf("%s%d", snapshotPrefix, platformID)
}

func loadLockKey(platformID int64) string {
	return fmt.Sprintf("%s%d", loadLockPrefix, platformID)
}

func mutationKey(platformID int64) string {
	return fmt.Sprintf("%s%d", mutationPrefix, platformID)
}

// Load returns the cached catalog, rebuilding it at most once across instances.
func (s *RedisStore) Load(ctx context.Context, platformID int64, load func(context.Context) (Catalog, error)) (Catalog, error) {
	if s == nil || s.redis == nil {
		return Catalog{}, fmt.Errorf("sms rate limit store is unavailable")
	}
	if catalog, found, err := s.read(ctx, platformID); err != nil {
		return Catalog{}, err
	} else if found {
		return catalog, nil
	}

	value, err, _ := s.group.Do(snapshotKey(platformID), func() (any, error) {
		if catalog, found, err := s.read(ctx, platformID); err != nil {
			return Catalog{}, err
		} else if found {
			return catalog, nil
		}
		if _, err := s.redis.SetStringIfMissing(ctx, loadLockKey(platformID), "1", loadLockTTL); err != nil {
			return Catalog{}, err
		}
		catalog, err := load(ctx)
		if err != nil {
			return Catalog{}, err
		}
		if err := s.write(ctx, platformID, catalog); err != nil {
			return Catalog{}, err
		}
		return catalog, nil
	})
	if err != nil {
		return Catalog{}, err
	}
	catalog, ok := value.(Catalog)
	if !ok {
		return Catalog{}, fmt.Errorf("sms rate limit store returned an invalid catalog")
	}
	return catalog, nil
}

func (s *RedisStore) read(ctx context.Context, platformID int64) (Catalog, bool, error) {
	raw, found, err := s.redis.GetString(ctx, snapshotKey(platformID))
	if err != nil {
		return Catalog{}, false, err
	}
	if !found {
		return Catalog{}, false, nil
	}
	snapshot, err := decodeSnapshot(raw)
	if err != nil {
		return Catalog{}, false, err
	}
	if snapshot.PlatformID != platformID || len(snapshot.Policies) != len(FixedPolicies()) {
		return Catalog{}, false, fmt.Errorf("sms rate limit snapshot does not match the fixed policy set")
	}
	catalog := Catalog{
		PlatformID: snapshot.PlatformID, PlatformCode: snapshot.PlatformCode,
		PlatformName: snapshot.PlatformName,
	}
	for _, fixed := range FixedPolicies() {
		policy, ok := snapshot.Policies[fixed.Key]
		if !ok {
			return Catalog{}, false, fmt.Errorf("sms rate limit snapshot is missing policy %q", fixed.Key)
		}
		catalog.Policies = append(catalog.Policies, Model{
			PlatformID: snapshot.PlatformID, Key: fixed.Key, Mode: fixed.Mode,
			Dimension: fixed.Dimension, Limit: policy.Limit,
			WindowSeconds: policy.WindowSeconds, Revision: policy.Revision,
		})
	}
	return catalog, true, nil
}

func (s *RedisStore) write(ctx context.Context, platformID int64, catalog Catalog) error {
	snapshot := policySnapshot{
		SchemaVersion: snapshotSchemaVersion,
		PlatformID:    catalog.PlatformID,
		PlatformCode:  catalog.PlatformCode,
		PlatformName:  catalog.PlatformName,
		Policies:      make(map[string]snapshotPolicy, len(catalog.Policies)),
	}
	for _, policy := range catalog.Policies {
		snapshot.Policies[policy.Key] = snapshotPolicy{
			Limit: policy.Limit, WindowSeconds: policy.WindowSeconds, Revision: policy.Revision,
		}
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("encode sms rate limit snapshot: %w", err)
	}
	return s.redis.SetString(ctx, snapshotKey(platformID), string(payload), snapshotTTL)
}

// Mutate runs one tokenized mutation: the snapshot is only invalidated after the
// database change is committed, and a stale owner can never delete a successor.
func (s *RedisStore) Mutate(ctx context.Context, platformID int64, change func(context.Context) error) error {
	if s == nil || s.redis == nil || change == nil {
		return fmt.Errorf("sms rate limit store is unavailable")
	}
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("create sms rate limit mutation token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	result, err := s.redis.EvalString(ctx, beginMutationScript, []string{mutationKey(platformID)}, token, int64(mutationLeaseTTL/time.Millisecond))
	if err != nil {
		return err
	}
	if result != "acquired" {
		return fmt.Errorf("sms rate limit mutation returned %q", result)
	}

	if err := change(ctx); err != nil {
		releaseContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_, releaseErr := s.redis.EvalString(releaseContext, releaseMutationScript, []string{mutationKey(platformID)}, token)
		if releaseErr != nil {
			return fmt.Errorf("sms rate limit mutation rollback: %w", errors.Join(err, releaseErr))
		}
		return err
	}

	publishContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	result, err = s.redis.EvalString(publishContext, publishMutationScript,
		[]string{mutationKey(platformID), snapshotKey(platformID)}, token)
	if err != nil {
		return err
	}
	if result != "published" {
		return fmt.Errorf("sms rate limit mutation publication returned %q", result)
	}
	return nil
}

func decodeSnapshot(raw string) (policySnapshot, error) {
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	var snapshot policySnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return policySnapshot{}, fmt.Errorf("decode sms rate limit snapshot: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return policySnapshot{}, fmt.Errorf("sms rate limit snapshot contains trailing data")
	}
	if snapshot.SchemaVersion != snapshotSchemaVersion {
		return policySnapshot{}, fmt.Errorf("sms rate limit snapshot schema is unsupported")
	}
	return snapshot, nil
}

const beginMutationScript = `
if redis.call('EXISTS', KEYS[1]) == 1 then return 'busy' end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
return 'acquired'
`

const releaseMutationScript = `
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 'stale' end
redis.call('DEL', KEYS[1])
return 'released'
`

const publishMutationScript = `
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 'stale' end
redis.call('DEL', KEYS[1])
redis.call('DEL', KEYS[2])
return 'published'
`
