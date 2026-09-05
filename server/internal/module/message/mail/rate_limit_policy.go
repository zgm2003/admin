package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	projectredis "admin/server/internal/redis"
	"golang.org/x/sync/singleflight"
)

const (
	rateLimitPolicySchemaVersion = 1
	rateLimitPolicySnapshotKey   = "mail:rate-limit:policies:v1"
	rateLimitPolicyLoadLockKey   = "mail:rate-limit:policies:load-lock:v1"
	rateLimitPolicyLoadLockTTL   = 5 * time.Second
	rateLimitPolicyRetryInterval = 50 * time.Millisecond
)

const (
	rateLimitPolicyStateReady        = "ready"
	rateLimitPolicyStateInvalidating = "invalidating"
)

var fixedRateLimitPolicyKeys = []string{
	"business_email_minute",
	"business_email_10m",
	"business_ip_minute",
	"business_scene_minute",
	"admin_test_user_10m",
	"admin_test_ip_minute",
	"admin_test_email_10m",
}

type fixedRateLimitSpec struct {
	Key           string
	Mode          string
	Dimension     string
	Limit         int
	WindowSeconds int
}

var fixedRateLimitSpecs = []fixedRateLimitSpec{
	{Key: "business_email_minute", Mode: "business", Dimension: "platform_scene_email", Limit: 1, WindowSeconds: 60},
	{Key: "business_email_10m", Mode: "business", Dimension: "platform_scene_email", Limit: 5, WindowSeconds: 600},
	{Key: "business_ip_minute", Mode: "business", Dimension: "platform_ip", Limit: 10, WindowSeconds: 60},
	{Key: "business_scene_minute", Mode: "business", Dimension: "platform_scene", Limit: 30, WindowSeconds: 60},
	{Key: "admin_test_user_10m", Mode: "admin_test", Dimension: "admin_user", Limit: 5, WindowSeconds: 600},
	{Key: "admin_test_ip_minute", Mode: "admin_test", Dimension: "ip", Limit: 10, WindowSeconds: 60},
	{Key: "admin_test_email_10m", Mode: "admin_test", Dimension: "email", Limit: 3, WindowSeconds: 600},
}

func fixedRateLimitSpecByKey(key string) (fixedRateLimitSpec, bool) {
	for _, spec := range fixedRateLimitSpecs {
		if spec.Key == key {
			return spec, true
		}
	}
	return fixedRateLimitSpec{}, false
}

func FixedRateLimitPolicies() []RateLimitPolicy {
	result := make([]RateLimitPolicy, 0, len(fixedRateLimitSpecs))
	for _, spec := range fixedRateLimitSpecs {
		result = append(result, RateLimitPolicy{
			Key:           spec.Key,
			Mode:          spec.Mode,
			Dimension:     spec.Dimension,
			Limit:         spec.Limit,
			WindowSeconds: spec.WindowSeconds,
		})
	}
	return result
}

func ValidateRateLimitPolicyInput(input RateLimitPolicyInput) error {
	if _, ok := fixedRateLimitSpecByKey(input.Key); !ok {
		return fmt.Errorf("unknown rate limit policy key")
	}
	if input.Limit < 1 || input.Limit > 100000 {
		return fmt.Errorf("rate limit count must be between 1 and 100000")
	}
	if input.WindowSeconds < 1 || input.WindowSeconds > 86400 {
		return fmt.Errorf("rate limit window must be between 1 and 86400 seconds")
	}
	return nil
}

// encodeRateLimitSnapshot and decodeRateLimitSnapshot are the only Redis
// payload boundaries for the policy snapshot.
func encodeRateLimitSnapshot(snapshot RateLimitSnapshot) (string, error) {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("encode rate limit policy snapshot: %w", err)
	}
	return string(payload), nil
}

func decodeRateLimitSnapshot(raw string) (RateLimitSnapshot, error) {
	if err := rejectDuplicateJSONKeys([]byte(raw)); err != nil {
		return RateLimitSnapshot{}, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return RateLimitSnapshot{}, fmt.Errorf("decode rate limit policy snapshot fields: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	var snapshot RateLimitSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return RateLimitSnapshot{}, fmt.Errorf("decode rate limit policy snapshot: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return RateLimitSnapshot{}, fmt.Errorf("rate limit policy snapshot contains trailing data")
	}
	stateField, ok := fields["state"]
	if !ok {
		return RateLimitSnapshot{}, fmt.Errorf("rate limit policy snapshot state is missing")
	}
	var state string
	if err := json.Unmarshal(stateField, &state); err != nil {
		return RateLimitSnapshot{}, fmt.Errorf("rate limit policy snapshot state is invalid: %w", err)
	}
	if state == rateLimitPolicyStateInvalidating {
		if _, exists := fields["policies"]; exists {
			return RateLimitSnapshot{}, fmt.Errorf("invalidating rate limit policy snapshot must not contain policies")
		}
	} else if state == rateLimitPolicyStateReady {
		if _, exists := fields["policies"]; !exists {
			return RateLimitSnapshot{}, fmt.Errorf("ready rate limit policy snapshot must contain policies")
		}
	}
	if err := validateRateLimitSnapshot(snapshot); err != nil {
		return RateLimitSnapshot{}, err
	}
	return snapshot, nil
}

func validateRateLimitSnapshot(snapshot RateLimitSnapshot) error {
	if snapshot.SchemaVersion != rateLimitPolicySchemaVersion {
		return fmt.Errorf("rate limit policy snapshot schema version is invalid")
	}
	if snapshot.Version < 1 {
		return fmt.Errorf("rate limit policy snapshot version is invalid")
	}
	switch snapshot.State {
	case rateLimitPolicyStateReady:
		if snapshot.MutationToken != nil {
			return fmt.Errorf("ready rate limit policy snapshot must not carry a mutation token")
		}
		if err := validateRateLimitSnapshotPolicies(snapshot.Policies); err != nil {
			return err
		}
	case rateLimitPolicyStateInvalidating:
		if snapshot.MutationToken == nil || *snapshot.MutationToken == "" {
			return fmt.Errorf("invalidating rate limit policy snapshot must carry a mutation token")
		}
		if len(snapshot.Policies) != 0 {
			return fmt.Errorf("invalidating rate limit policy snapshot must not carry policies")
		}
	default:
		return fmt.Errorf("rate limit policy snapshot state is invalid")
	}
	return nil
}

func validateRateLimitSnapshotPolicies(policies map[string]rateLimitPolicyValue) error {
	if len(policies) != len(fixedRateLimitPolicyKeys) {
		return fmt.Errorf("rate limit policy snapshot must contain exactly %d policies", len(fixedRateLimitPolicyKeys))
	}
	for _, key := range fixedRateLimitPolicyKeys {
		value, ok := policies[key]
		if !ok {
			return fmt.Errorf("rate limit policy snapshot is missing policy %q", key)
		}
		if value.Limit < 1 || value.Limit > 100000 || value.WindowSeconds < 1 || value.WindowSeconds > 86400 {
			return fmt.Errorf("rate limit policy %q values are out of range", key)
		}
		if value.UpdatedAt.IsZero() {
			return fmt.Errorf("rate limit policy %q has a zero timestamp", key)
		}
	}
	return nil
}

func snapshotFromCatalog(catalog RateLimitCatalog) (RateLimitSnapshot, error) {
	policies := make(map[string]rateLimitPolicyValue, len(catalog.Policies))
	byKey := make(map[string]RateLimitPolicy, len(catalog.Policies))
	for _, policy := range catalog.Policies {
		byKey[policy.Key] = policy
	}
	for _, key := range fixedRateLimitPolicyKeys {
		policy, ok := byKey[key]
		if !ok {
			return RateLimitSnapshot{}, fmt.Errorf("rate limit policy %q is missing", key)
		}
		policies[key] = rateLimitPolicyValue{
			Limit:         policy.Limit,
			WindowSeconds: policy.WindowSeconds,
			UpdatedAt:     policy.UpdatedAt,
		}
	}
	return RateLimitSnapshot{
		SchemaVersion: rateLimitPolicySchemaVersion,
		State:         rateLimitPolicyStateReady,
		Version:       catalog.Version,
		Policies:      policies,
	}, nil
}

func catalogFromSnapshot(snapshot RateLimitSnapshot) RateLimitCatalog {
	policies := make([]RateLimitPolicy, 0, len(fixedRateLimitPolicyKeys))
	for _, key := range fixedRateLimitPolicyKeys {
		value := snapshot.Policies[key]
		spec, _ := fixedRateLimitSpecByKey(key)
		policies = append(policies, RateLimitPolicy{
			Key:           key,
			Mode:          spec.Mode,
			Dimension:     spec.Dimension,
			Limit:         value.Limit,
			WindowSeconds: value.WindowSeconds,
			UpdatedAt:     value.UpdatedAt,
		})
	}
	return RateLimitCatalog{Version: snapshot.Version, Policies: policies}
}

// rejectDuplicateJSONKeys performs a token-level duplicate-key scan so a
// hostile or corrupt payload cannot smuggle a second key past the decoder.
func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return fmt.Errorf("rate limit policy snapshot must be a JSON object")
	}
	return scanJSONObject(decoder, map[string]struct{}{})
}

// scanJSONObject scans an object whose opening '{' has already been consumed.
func scanJSONObject(decoder *json.Decoder, seen map[string]struct{}) error {
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("rate limit policy snapshot key is invalid")
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("rate limit policy snapshot contains duplicate key %q", key)
		}
		seen[key] = struct{}{}
		if err := scanJSONValue(decoder); err != nil {
			return err
		}
	}
	_, err := decoder.Token()
	return err
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delim, ok := token.(json.Delim); ok {
		switch delim {
		case '{':
			return scanJSONObject(decoder, map[string]struct{}{})
		case '[':
			for decoder.More() {
				if err := scanJSONValue(decoder); err != nil {
					return err
				}
			}
			_, err := decoder.Token()
			return err
		default:
			return fmt.Errorf("rate limit policy snapshot contains invalid nesting")
		}
	}
	return nil
}

// rateLimitPolicyStore is the concrete Redis-backed policy store. The
// management Service uses the Repository directly for an authoritative GET;
// this Store is the send hot path and the publication boundary for updates.
type rateLimitPolicyStore struct {
	repository *Repository
	redis      *projectredis.Client
	group      singleflight.Group
}

func NewRateLimitPolicyStore(repository *Repository, redis *projectredis.Client) *rateLimitPolicyStore {
	return &rateLimitPolicyStore{repository: repository, redis: redis}
}

func (s *rateLimitPolicyStore) Load(ctx context.Context) (RateLimitCatalog, error) {
	if s == nil || s.redis == nil {
		return RateLimitCatalog{}, dependency(fmt.Errorf("mail rate limit policy Redis store unavailable"))
	}
	snapshot, found, err := s.readSnapshot(ctx)
	if err != nil {
		return RateLimitCatalog{}, err
	}
	if found {
		return catalogFromSnapshot(snapshot), nil
	}
	result := s.group.DoChan(rateLimitPolicySnapshotKey, func() (any, error) {
		// A shared rebuild must outlive the caller that happens to win
		// singleflight, but it still needs a hard upper bound.
		sharedContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), rateLimitPolicyLoadLockTTL)
		defer cancel()
		return s.rebuild(sharedContext)
	})
	var sharedResult singleflight.Result
	select {
	case <-ctx.Done():
		return RateLimitCatalog{}, ctx.Err()
	case sharedResult = <-result:
	}
	if sharedResult.Err != nil {
		return RateLimitCatalog{}, sharedResult.Err
	}
	catalog, ok := sharedResult.Val.(RateLimitCatalog)
	if !ok {
		return RateLimitCatalog{}, fmt.Errorf("rate limit policy rebuild returned an invalid value")
	}
	return catalog, nil
}

func (s *rateLimitPolicyStore) readSnapshot(ctx context.Context) (RateLimitSnapshot, bool, error) {
	if s == nil || s.redis == nil {
		return RateLimitSnapshot{}, false, dependency(fmt.Errorf("mail rate limit policy Redis store unavailable"))
	}
	raw, found, err := s.redis.GetString(ctx, rateLimitPolicySnapshotKey)
	if err != nil {
		return RateLimitSnapshot{}, false, dependency(err)
	}
	if !found {
		return RateLimitSnapshot{}, false, nil
	}
	snapshot, err := decodeRateLimitSnapshot(raw)
	if err != nil {
		return RateLimitSnapshot{}, true, dependency(err)
	}
	if snapshot.State == rateLimitPolicyStateInvalidating {
		return RateLimitSnapshot{}, true, dependency(fmt.Errorf("rate limit policy snapshot is invalidating"))
	}
	return snapshot, true, nil
}

func (s *rateLimitPolicyStore) rebuild(ctx context.Context) (RateLimitCatalog, error) {
	if s == nil || s.redis == nil || s.repository == nil {
		return RateLimitCatalog{}, dependency(fmt.Errorf("mail rate limit policy rebuild dependencies unavailable"))
	}
	token, err := randomRateLimitToken()
	if err != nil {
		return RateLimitCatalog{}, dependency(err)
	}
	acquired, err := s.redis.SetStringIfMissing(ctx, rateLimitPolicyLoadLockKey, token, 5*time.Second)
	if err != nil {
		return RateLimitCatalog{}, dependency(err)
	}
	if acquired {
		defer func() {
			releaseContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
			defer cancel()
			_ = s.releaseLoadLock(releaseContext, token)
		}()
		catalog, dbErr := s.repository.ListRateLimitPolicies(ctx)
		if dbErr != nil {
			return RateLimitCatalog{}, dependency(dbErr)
		}
		snapshot, snapErr := snapshotFromCatalog(catalog)
		if snapErr != nil {
			return RateLimitCatalog{}, dependency(snapErr)
		}
		payload, encErr := encodeRateLimitSnapshot(snapshot)
		if encErr != nil {
			return RateLimitCatalog{}, dependency(encErr)
		}
		published, publishErr := s.redis.EvalString(ctx, rebuildRateLimitSnapshotScript,
			[]string{rateLimitPolicySnapshotKey}, catalog.Version, payload)
		if publishErr != nil {
			_ = s.releaseLoadLock(ctx, token)
			return RateLimitCatalog{}, dependency(publishErr)
		}
		if published == "newer" {
			current, found, readErr := s.readSnapshot(ctx)
			if readErr != nil {
				return RateLimitCatalog{}, readErr
			}
			if !found {
				return RateLimitCatalog{}, dependency(fmt.Errorf("rate limit policy snapshot disappeared during rebuild"))
			}
			return catalogFromSnapshot(current), nil
		}
		if published == "busy" {
			return RateLimitCatalog{}, dependency(fmt.Errorf("rate limit policy snapshot is being updated"))
		}
		if published != "published" {
			return RateLimitCatalog{}, dependency(fmt.Errorf("rate limit policy snapshot rebuild returned %q", published))
		}
		return catalog, nil
	}
	deadline := time.Now().Add(rateLimitPolicyLoadLockTTL)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		if remaining > rateLimitPolicyRetryInterval {
			remaining = rateLimitPolicyRetryInterval
		}
		if err := waitRateLimitRetry(ctx, remaining); err != nil {
			return RateLimitCatalog{}, dependency(err)
		}
		snapshot, found, readErr := s.readSnapshot(ctx)
		if readErr != nil {
			return RateLimitCatalog{}, readErr
		}
		if found {
			return catalogFromSnapshot(snapshot), nil
		}
	}
	return RateLimitCatalog{}, dependency(fmt.Errorf("rate limit policy snapshot rebuild timed out"))
}

func waitRateLimitRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

const releaseRateLimitLoadLockScript = `
local current = redis.call('GET', KEYS[1])
if current == ARGV[1] then redis.call('DEL', KEYS[1]) end
return 1
`

func (s *rateLimitPolicyStore) releaseLoadLock(ctx context.Context, token string) error {
	_, err := s.redis.EvalString(ctx, releaseRateLimitLoadLockScript, []string{rateLimitPolicyLoadLockKey}, token)
	return err
}

func (s *rateLimitPolicyStore) Update(ctx context.Context, input RateLimitPolicyInput) (RateLimitCatalog, error) {
	if err := ValidateRateLimitPolicyInput(input); err != nil {
		return RateLimitCatalog{}, invalid(err)
	}
	if s == nil || s.redis == nil || s.repository == nil {
		return RateLimitCatalog{}, dependency(fmt.Errorf("mail rate limit policy update dependencies unavailable"))
	}
	prior, found, err := s.readSnapshot(ctx)
	if err != nil {
		return RateLimitCatalog{}, err
	}
	if !found {
		if _, loadErr := s.Load(ctx); loadErr != nil {
			return RateLimitCatalog{}, loadErr
		}
		prior, found, err = s.readSnapshot(ctx)
		if err != nil {
			return RateLimitCatalog{}, err
		}
		if !found {
			return RateLimitCatalog{}, dependency(fmt.Errorf("rate limit policy snapshot is unavailable"))
		}
	}

	token, err := randomRateLimitToken()
	if err != nil {
		return RateLimitCatalog{}, dependency(err)
	}
	invalidating, err := encodeRateLimitSnapshot(RateLimitSnapshot{
		SchemaVersion: rateLimitPolicySchemaVersion,
		State:         rateLimitPolicyStateInvalidating,
		Version:       prior.Version,
		MutationToken: &token,
	})
	if err != nil {
		return RateLimitCatalog{}, dependency(err)
	}
	acquired, err := s.redis.EvalString(ctx, beginRateLimitMutationScript,
		[]string{rateLimitPolicySnapshotKey}, prior.Version, invalidating, int64((30*time.Second)/time.Millisecond))
	if err != nil {
		return RateLimitCatalog{}, dependency(err)
	}
	if acquired != "acquired" {
		return RateLimitCatalog{}, dependency(fmt.Errorf("rate limit policy is being updated by another request"))
	}

	catalog, dbErr := s.repository.UpdateRateLimitPolicy(ctx, input)
	if dbErr != nil {
		if rollbackErr := s.rollbackReady(ctx, prior, token); rollbackErr != nil {
			return RateLimitCatalog{}, dependency(fmt.Errorf("rollback rate limit policy snapshot: %w", rollbackErr))
		}
		return RateLimitCatalog{}, wrapRepo(dbErr)
	}

	ready, snapErr := snapshotFromCatalog(catalog)
	if snapErr != nil {
		return RateLimitCatalog{}, dependency(snapErr)
	}
	readyPayload, encErr := encodeRateLimitSnapshot(ready)
	if encErr != nil {
		return RateLimitCatalog{}, dependency(encErr)
	}
	result, pubErr := s.redis.EvalString(ctx, publishRateLimitMutationScript,
		[]string{rateLimitPolicySnapshotKey}, token, catalog.Version, readyPayload)
	if pubErr != nil {
		return RateLimitCatalog{}, dependency(pubErr)
	}
	if result == "newer" {
		current, _, readErr := s.readSnapshot(ctx)
		if readErr != nil {
			return RateLimitCatalog{}, readErr
		}
		if current.Version < catalog.Version {
			return RateLimitCatalog{}, dependency(fmt.Errorf("rate limit policy publication lost a version race"))
		}
		return catalog, nil
	}
	if result != "published" {
		return RateLimitCatalog{}, dependency(fmt.Errorf("rate limit policy publication failed"))
	}
	return catalog, nil
}

func (s *rateLimitPolicyStore) rollbackReady(ctx context.Context, prior RateLimitSnapshot, token string) error {
	payload, err := encodeRateLimitSnapshot(prior)
	if err != nil {
		return err
	}
	result, err := s.redis.EvalString(ctx, publishRateLimitMutationScript,
		[]string{rateLimitPolicySnapshotKey}, token, prior.Version, payload)
	if err != nil {
		return err
	}
	return validateRateLimitPolicyPublicationResult(result)
}

func validateRateLimitPolicyPublicationResult(result string) error {
	if result == "published" || result == "newer" {
		return nil
	}
	return fmt.Errorf("rate limit policy publication returned %q", result)
}

// beginRateLimitMutationScript atomically replaces a ready snapshot whose
// version matches ARGV[1] with the invalidating payload (ARGV[2]).
const beginRateLimitMutationScript = `
local current = redis.call('GET', KEYS[1])
if not current then return 'missing' end
local decoded = cjson.decode(current)
if decoded.state ~= 'ready' then return 'not-ready' end
if decoded.version ~= tonumber(ARGV[1]) then return 'changed' end
redis.call('SET', KEYS[1], ARGV[2], 'PX', ARGV[3])
return 'acquired'
`

// publishRateLimitMutationScript verifies the lease token, refuses to
// overwrite a newer ready version, then atomically publishes the ready state.
const publishRateLimitMutationScript = `
local current = redis.call('GET', KEYS[1])
if not current then return 'missing' end
local decoded = cjson.decode(current)
local newVersion = tonumber(ARGV[2])
if decoded.state == 'ready' and decoded.version >= newVersion then return 'newer' end
if decoded.state == 'invalidating' and decoded.mutationToken ~= ARGV[1] then return 'token-mismatch' end
redis.call('SET', KEYS[1], ARGV[3])
return 'published'
`

// rebuildRateLimitSnapshotScript publishes a DB snapshot only when it is not
// older than the ready snapshot already present in Redis.
const rebuildRateLimitSnapshotScript = `
local current = redis.call('GET', KEYS[1])
if current then
  local ok, decoded = pcall(cjson.decode, current)
  if ok and decoded.state == 'invalidating' then return 'busy' end
  if ok and decoded.state == 'ready' and decoded.version >= tonumber(ARGV[1]) then return 'newer' end
end
redis.call('SET', KEYS[1], ARGV[2])
return 'published'
`

func randomRateLimitToken() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate rate limit policy mutation token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}
