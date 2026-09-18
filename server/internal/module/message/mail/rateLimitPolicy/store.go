package ratelimitpolicy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/cacheFill"
	cachegeneration "admin/server/internal/shared/cacheGeneration"
)

const (
	rateLimitPolicySchemaVersion = 1
	rateLimitPolicySnapshotTTL   = 10 * time.Minute
	rateLimitPolicyReadBudget    = 500 * time.Millisecond
	rateLimitPolicyWaitStep      = 20 * time.Millisecond
)

var (
	mailRateLimitGenerationScope = cachegeneration.Scope{Namespace: "message.mail", ScopeKey: "global"}
	errRateLimitSnapshotCorrupt  = errors.New("mail rate limit policy snapshot is corrupt")
	errRateLimitGenerationMoved  = errors.New("mail cache generation advanced during rate limit fill")
)

var fixedRateLimitPolicyKeys = []string{
	"business_email_minute",
	"business_email_10m",
}

type fixedRateLimitSpec struct {
	Key           string
	Mode          string
	Dimension     string
	Limit         int
	WindowSeconds int
}

var fixedRateLimitSpecs = []fixedRateLimitSpec{
	{Key: "business_email_minute", Mode: "business", Dimension: "platform_email", Limit: 1, WindowSeconds: 60},
	{Key: "business_email_10m", Mode: "business", Dimension: "platform_email", Limit: 5, WindowSeconds: 600},
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
			Key: spec.Key, Mode: spec.Mode, Dimension: spec.Dimension,
			Limit: spec.Limit, WindowSeconds: spec.WindowSeconds,
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

func ValidateInput(input Input) error { return ValidateRateLimitPolicyInput(input) }

func encodeRateLimitSnapshot(snapshot RateLimitSnapshot) (string, error) {
	if err := validateRateLimitSnapshot(snapshot); err != nil {
		return "", err
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("encode rate limit policy snapshot: %w", err)
	}
	return string(payload), nil
}

func decodeRateLimitSnapshot(raw string, generation, platformID int64) (RateLimitSnapshot, error) {
	if err := rejectDuplicateJSONKeys([]byte(raw)); err != nil {
		return RateLimitSnapshot{}, fmt.Errorf("%w: %v", errRateLimitSnapshotCorrupt, err)
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	var snapshot RateLimitSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return RateLimitSnapshot{}, fmt.Errorf("%w: decode payload: %v", errRateLimitSnapshotCorrupt, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return RateLimitSnapshot{}, fmt.Errorf("%w: trailing data", errRateLimitSnapshotCorrupt)
	}
	if snapshot.Generation != generation || snapshot.PlatformID != platformID {
		return RateLimitSnapshot{}, fmt.Errorf("%w: coordinates are invalid", errRateLimitSnapshotCorrupt)
	}
	if err := validateRateLimitSnapshot(snapshot); err != nil {
		return RateLimitSnapshot{}, fmt.Errorf("%w: %v", errRateLimitSnapshotCorrupt, err)
	}
	return snapshot, nil
}

func validateRateLimitSnapshot(snapshot RateLimitSnapshot) error {
	if snapshot.SchemaVersion != rateLimitPolicySchemaVersion || snapshot.Generation < 1 || snapshot.PlatformID < 1 {
		return fmt.Errorf("rate limit policy snapshot coordinates are invalid")
	}
	return validateRateLimitSnapshotPolicies(snapshot.Policies)
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

func snapshotFromCatalog(generation int64, catalog RateLimitCatalog) (RateLimitSnapshot, error) {
	policies := make(map[string]rateLimitPolicyValue, len(catalog.Policies))
	byKey := make(map[string]RateLimitPolicy, len(catalog.Policies))
	for _, policy := range catalog.Policies {
		if _, exists := byKey[policy.Key]; exists {
			return RateLimitSnapshot{}, fmt.Errorf("rate limit policy %q is duplicated", policy.Key)
		}
		byKey[policy.Key] = policy
	}
	for _, key := range fixedRateLimitPolicyKeys {
		policy, ok := byKey[key]
		if !ok {
			return RateLimitSnapshot{}, fmt.Errorf("rate limit policy %q is missing", key)
		}
		policies[key] = rateLimitPolicyValue{
			Limit: policy.Limit, WindowSeconds: policy.WindowSeconds, UpdatedAt: policy.UpdatedAt,
		}
	}
	snapshot := RateLimitSnapshot{
		SchemaVersion: rateLimitPolicySchemaVersion,
		Generation:    generation, PlatformID: catalog.PlatformID, Policies: policies,
	}
	if err := validateRateLimitSnapshot(snapshot); err != nil {
		return RateLimitSnapshot{}, err
	}
	return snapshot, nil
}

func catalogFromSnapshot(snapshot RateLimitSnapshot) RateLimitCatalog {
	policies := make([]RateLimitPolicy, 0, len(fixedRateLimitPolicyKeys))
	for _, key := range fixedRateLimitPolicyKeys {
		value := snapshot.Policies[key]
		spec, _ := fixedRateLimitSpecByKey(key)
		policies = append(policies, RateLimitPolicy{
			PlatformID: snapshot.PlatformID, Key: key, Mode: spec.Mode, Dimension: spec.Dimension,
			Limit: value.Limit, WindowSeconds: value.WindowSeconds, UpdatedAt: value.UpdatedAt,
		})
	}
	return RateLimitCatalog{PlatformID: snapshot.PlatformID, Policies: policies}
}

type policyLoader interface {
	List(context.Context, int64) (Catalog, error)
}

type rateLimitPolicyStore struct {
	repository  policyLoader
	redis       *projectredis.Client
	generations *cachegeneration.Repository
	states      *cachegeneration.Store
	scope       cachegeneration.Scope
	ttl         time.Duration
	readBudget  time.Duration
	waitStep    time.Duration
	now         func() time.Time
	wait        func(context.Context, time.Duration) error
}

func newRateLimitStore(repository policyLoader, redis *projectredis.Client) *rateLimitPolicyStore {
	return &rateLimitPolicyStore{
		repository: repository, redis: redis, scope: mailRateLimitGenerationScope,
		ttl: rateLimitPolicySnapshotTTL, readBudget: rateLimitPolicyReadBudget, waitStep: rateLimitPolicyWaitStep,
		now: time.Now, wait: waitRateLimitRetry,
	}
}

func NewRateLimitPolicyStore(repository *Repository, redis *projectredis.Client) *rateLimitPolicyStore {
	return newRateLimitStore(repository, redis)
}

func NewStore(repository *Repository, redis *projectredis.Client) *rateLimitPolicyStore {
	return NewRateLimitPolicyStore(repository, redis)
}

func (s *rateLimitPolicyStore) SetGenerations(repository *cachegeneration.Repository, store *cachegeneration.Store) {
	s.generations = repository
	s.states = store
}

func (s *rateLimitPolicyStore) ValidateDependencies() error { return s.configured() }

func (s *rateLimitPolicyStore) Load(ctx context.Context, platformID int64) (RateLimitCatalog, error) {
	variant, err := rateLimitVariant(platformID)
	if err != nil {
		return RateLimitCatalog{}, dependency(err)
	}
	if err := s.configured(); err != nil {
		return RateLimitCatalog{}, dependency(err)
	}
	deadline := s.now().Add(s.readBudget)
	for s.now().Before(deadline) {
		state, retry, err := s.readyState(ctx, deadline, variant, platformID)
		if err != nil {
			return RateLimitCatalog{}, dependency(err)
		}
		if retry {
			continue
		}
		catalog, found, err := s.read(ctx, state.Generation, platformID, variant)
		if err != nil && !errors.Is(err, errRateLimitSnapshotCorrupt) {
			return RateLimitCatalog{}, dependency(err)
		}
		if err == nil && found {
			return catalog, nil
		}
		if err := s.fill(ctx, state.Generation, platformID, variant); err != nil {
			if errors.Is(err, errRateLimitGenerationMoved) {
				continue
			}
			return RateLimitCatalog{}, dependency(err)
		}
	}
	return RateLimitCatalog{}, dependency(fmt.Errorf("mail rate limit cache read budget exhausted"))
}

func rateLimitVariant(platformID int64) (string, error) {
	if platformID < 1 {
		return "", fmt.Errorf("mail rate limit policy platform is invalid")
	}
	return fmt.Sprintf("ratelimit:%d", platformID), nil
}

func (s *rateLimitPolicyStore) readyState(ctx context.Context, deadline time.Time, variant string, platformID int64) (cachegeneration.State, bool, error) {
	if err := ctx.Err(); err != nil {
		return cachegeneration.State{}, false, err
	}
	state, found, err := s.states.Read(ctx, s.scope)
	if err != nil {
		if errors.Is(err, cachegeneration.ErrStateCorrupt) {
			if repairErr := s.recoverState(ctx, variant, platformID); repairErr != nil {
				if errors.Is(repairErr, errRateLimitGenerationMoved) {
					return cachegeneration.State{}, true, nil
				}
				return cachegeneration.State{}, false, errors.Join(err, repairErr)
			}
			return cachegeneration.State{}, true, nil
		}
		return cachegeneration.State{}, false, err
	}
	if !found {
		if repairErr := s.recoverState(ctx, variant, platformID); repairErr != nil {
			if errors.Is(repairErr, errRateLimitGenerationMoved) {
				return cachegeneration.State{}, true, nil
			}
			return cachegeneration.State{}, false, errors.Join(cachegeneration.ErrStateMissing, repairErr)
		}
		return cachegeneration.State{}, true, nil
	}
	if state.State == cachegeneration.StateInvalidating {
		if s.now().Add(s.waitStep).After(deadline) {
			return cachegeneration.State{}, false, fmt.Errorf("mail cache state is invalidating")
		}
		if err := s.wait(ctx, s.waitStep); err != nil {
			return cachegeneration.State{}, false, err
		}
		return cachegeneration.State{}, true, nil
	}
	if state.State != cachegeneration.StateReady {
		return cachegeneration.State{}, false, fmt.Errorf("mail cache state is invalid")
	}
	return state, false, nil
}

func (s *rateLimitPolicyStore) fill(ctx context.Context, generation, platformID int64, variant string) error {
	lease, err := s.tryFill(ctx, generation, variant)
	if err != nil {
		return err
	}
	if lease == nil {
		return s.wait(ctx, s.waitStep)
	}
	defer func() { _ = lease.Release(ctx) }()
	workContext, cancel := lease.WorkContext(ctx)
	defer cancel()
	if _, found, err := s.read(workContext, generation, platformID, variant); err == nil && found {
		return nil
	}
	catalog, err := s.repository.List(workContext, platformID)
	if err != nil {
		return err
	}
	if err := s.confirmReadyGeneration(workContext, generation); err != nil {
		return err
	}
	return s.write(workContext, generation, variant, catalog)
}

func (s *rateLimitPolicyStore) recoverState(ctx context.Context, variant string, platformID int64) error {
	lease, err := cachefill.Try(ctx, s.redis.UniversalClient(), cachegeneration.StateKey(s.scope), "repair:"+variant)
	if err != nil {
		return err
	}
	if lease == nil {
		return s.wait(ctx, s.waitStep)
	}
	defer func() { _ = lease.Release(ctx) }()
	workContext, cancel := lease.WorkContext(ctx)
	defer cancel()
	generation, err := s.generations.Current(workContext, s.scope)
	if err != nil {
		return err
	}
	if _, err := s.states.Reconcile(workContext, s.scope, generation); err != nil {
		return err
	}
	catalog, err := s.repository.List(workContext, platformID)
	if err != nil {
		return err
	}
	if err := s.confirmReadyGeneration(workContext, generation); err != nil {
		return err
	}
	return s.write(workContext, generation, variant, catalog)
}

func (s *rateLimitPolicyStore) tryFill(ctx context.Context, generation int64, variant string) (*cachefill.Lease, error) {
	target, err := cachegeneration.FillKey(s.scope, generation, variant)
	if err != nil {
		return nil, err
	}
	return cachefill.Try(ctx, s.redis.UniversalClient(), cachegeneration.StateKey(s.scope), target)
}

func (s *rateLimitPolicyStore) read(ctx context.Context, generation, platformID int64, variant string) (RateLimitCatalog, bool, error) {
	key, err := cachegeneration.SnapshotKey(s.scope, generation, variant)
	if err != nil {
		return RateLimitCatalog{}, false, err
	}
	raw, found, err := s.redis.GetString(ctx, key)
	if err != nil || !found {
		return RateLimitCatalog{}, found, err
	}
	snapshot, err := decodeRateLimitSnapshot(raw, generation, platformID)
	if err != nil {
		return RateLimitCatalog{}, false, err
	}
	return catalogFromSnapshot(snapshot), true, nil
}

func (s *rateLimitPolicyStore) write(ctx context.Context, generation int64, variant string, catalog RateLimitCatalog) error {
	snapshot, err := snapshotFromCatalog(generation, catalog)
	if err != nil {
		return err
	}
	payload, err := encodeRateLimitSnapshot(snapshot)
	if err != nil {
		return err
	}
	key, err := cachegeneration.SnapshotKey(s.scope, generation, variant)
	if err != nil {
		return err
	}
	return s.redis.SetString(ctx, key, payload, s.ttl)
}

func (s *rateLimitPolicyStore) confirmReadyGeneration(ctx context.Context, generation int64) error {
	state, found, err := s.states.Read(ctx, s.scope)
	if err != nil {
		return err
	}
	if !found || state.State != cachegeneration.StateReady || state.Generation != generation {
		return fmt.Errorf("%w: expected generation %d", errRateLimitGenerationMoved, generation)
	}
	return nil
}

func (s *rateLimitPolicyStore) configured() error {
	if s == nil || s.repository == nil || s.redis == nil || s.generations == nil || s.states == nil {
		return fmt.Errorf("mail rate limit cache generation dependencies are not configured")
	}
	return s.scope.Validate()
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

func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return fmt.Errorf("rate limit policy snapshot must be a JSON object")
	}
	if err := scanJSONObject(decoder, map[string]struct{}{}); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("rate limit policy snapshot contains trailing data")
	}
	return nil
}

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
	if delimiter, ok := token.(json.Delim); ok {
		switch delimiter {
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
