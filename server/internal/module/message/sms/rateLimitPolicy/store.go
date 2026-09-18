package rateLimitPolicy

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
	snapshotSchemaVersion  = 1
	snapshotTTL            = 10 * time.Minute
	smsRateLimitReadBudget = 500 * time.Millisecond
	smsRateLimitWaitStep   = 20 * time.Millisecond
)

var (
	smsRateLimitGenerationScope = cachegeneration.Scope{Namespace: "message.sms", ScopeKey: "global"}
	errSnapshotCorrupt          = errors.New("sms rate limit snapshot is corrupt")
	errGenerationAdvanced       = errors.New("sms cache generation advanced during rate limit fill")
)

type snapshotPolicy struct {
	Limit         int `json:"limit"`
	WindowSeconds int `json:"windowSeconds"`
}

type policySnapshot struct {
	SchemaVersion int                       `json:"schemaVersion"`
	Generation    int64                     `json:"generation"`
	PlatformID    int64                     `json:"platformId"`
	PlatformCode  string                    `json:"platformCode"`
	PlatformName  string                    `json:"platformName"`
	Policies      map[string]snapshotPolicy `json:"policies"`
}

type RedisStore struct {
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

func NewStore(redis *projectredis.Client) *RedisStore {
	return &RedisStore{
		redis: redis, scope: smsRateLimitGenerationScope,
		ttl: snapshotTTL, readBudget: smsRateLimitReadBudget, waitStep: smsRateLimitWaitStep,
		now: time.Now, wait: waitForSMSRateLimit,
	}
}

func (s *RedisStore) SetGenerations(repository *cachegeneration.Repository, store *cachegeneration.Store) {
	s.generations = repository
	s.states = store
}

func (s *RedisStore) ValidateDependencies() error { return s.configured() }

func (s *RedisStore) Load(ctx context.Context, platformID int64, load func(context.Context) (Catalog, error)) (Catalog, error) {
	variant, err := smsRateLimitVariant(platformID)
	if err != nil {
		return Catalog{}, err
	}
	if err := s.configured(); err != nil || load == nil {
		if err == nil {
			err = fmt.Errorf("sms rate limit loader is unavailable")
		}
		return Catalog{}, err
	}
	deadline := s.now().Add(s.readBudget)
	for s.now().Before(deadline) {
		state, retry, err := s.readyState(ctx, deadline, platformID, variant, load)
		if err != nil {
			return Catalog{}, err
		}
		if retry {
			continue
		}
		catalog, found, err := s.read(ctx, state.Generation, platformID, variant)
		if err != nil && !errors.Is(err, errSnapshotCorrupt) {
			return Catalog{}, err
		}
		if err == nil && found {
			return catalog, nil
		}
		if err := s.fill(ctx, state.Generation, platformID, variant, load); err != nil {
			if errors.Is(err, errGenerationAdvanced) {
				continue
			}
			return Catalog{}, err
		}
	}
	return Catalog{}, fmt.Errorf("sms rate limit cache read budget exhausted")
}

func smsRateLimitVariant(platformID int64) (string, error) {
	if platformID < 1 {
		return "", fmt.Errorf("sms rate limit platform is invalid")
	}
	return fmt.Sprintf("ratelimit:%d", platformID), nil
}

func (s *RedisStore) readyState(ctx context.Context, deadline time.Time, platformID int64, variant string, load func(context.Context) (Catalog, error)) (cachegeneration.State, bool, error) {
	if err := ctx.Err(); err != nil {
		return cachegeneration.State{}, false, err
	}
	state, found, err := s.states.Read(ctx, s.scope)
	if err != nil {
		if errors.Is(err, cachegeneration.ErrStateCorrupt) {
			if repairErr := s.recoverState(ctx, platformID, variant, load); repairErr != nil {
				if errors.Is(repairErr, errGenerationAdvanced) {
					return cachegeneration.State{}, true, nil
				}
				return cachegeneration.State{}, false, errors.Join(err, repairErr)
			}
			return cachegeneration.State{}, true, nil
		}
		return cachegeneration.State{}, false, err
	}
	if !found {
		if repairErr := s.recoverState(ctx, platformID, variant, load); repairErr != nil {
			if errors.Is(repairErr, errGenerationAdvanced) {
				return cachegeneration.State{}, true, nil
			}
			return cachegeneration.State{}, false, errors.Join(cachegeneration.ErrStateMissing, repairErr)
		}
		return cachegeneration.State{}, true, nil
	}
	if state.State == cachegeneration.StateInvalidating {
		if s.now().Add(s.waitStep).After(deadline) {
			return cachegeneration.State{}, false, fmt.Errorf("sms cache state is invalidating")
		}
		if err := s.wait(ctx, s.waitStep); err != nil {
			return cachegeneration.State{}, false, err
		}
		return cachegeneration.State{}, true, nil
	}
	if state.State != cachegeneration.StateReady {
		return cachegeneration.State{}, false, fmt.Errorf("sms cache state is invalid")
	}
	return state, false, nil
}

func (s *RedisStore) fill(ctx context.Context, generation, platformID int64, variant string, load func(context.Context) (Catalog, error)) error {
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
	catalog, err := load(workContext)
	if err != nil {
		return err
	}
	if err := s.confirmReadyGeneration(workContext, generation); err != nil {
		return err
	}
	return s.write(workContext, generation, variant, catalog)
}

func (s *RedisStore) recoverState(ctx context.Context, platformID int64, variant string, load func(context.Context) (Catalog, error)) error {
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
	catalog, err := load(workContext)
	if err != nil {
		return err
	}
	if err := s.confirmReadyGeneration(workContext, generation); err != nil {
		return err
	}
	return s.write(workContext, generation, variant, catalog)
}

func (s *RedisStore) tryFill(ctx context.Context, generation int64, variant string) (*cachefill.Lease, error) {
	target, err := cachegeneration.FillKey(s.scope, generation, variant)
	if err != nil {
		return nil, err
	}
	return cachefill.Try(ctx, s.redis.UniversalClient(), cachegeneration.StateKey(s.scope), target)
}

func (s *RedisStore) read(ctx context.Context, generation, platformID int64, variant string) (Catalog, bool, error) {
	key, err := cachegeneration.SnapshotKey(s.scope, generation, variant)
	if err != nil {
		return Catalog{}, false, err
	}
	raw, found, err := s.redis.GetString(ctx, key)
	if err != nil || !found {
		return Catalog{}, found, err
	}
	snapshot, err := decodeSnapshot(raw, generation, platformID)
	if err != nil {
		return Catalog{}, false, err
	}
	return catalogFromSnapshot(snapshot), true, nil
}

func (s *RedisStore) write(ctx context.Context, generation int64, variant string, catalog Catalog) error {
	snapshot, err := snapshotFromCatalog(generation, catalog)
	if err != nil {
		return err
	}
	payload, err := encodeSnapshot(snapshot)
	if err != nil {
		return err
	}
	key, err := cachegeneration.SnapshotKey(s.scope, generation, variant)
	if err != nil {
		return err
	}
	return s.redis.SetString(ctx, key, payload, s.ttl)
}

func (s *RedisStore) confirmReadyGeneration(ctx context.Context, generation int64) error {
	state, found, err := s.states.Read(ctx, s.scope)
	if err != nil {
		return err
	}
	if !found || state.State != cachegeneration.StateReady || state.Generation != generation {
		return fmt.Errorf("%w: expected generation %d", errGenerationAdvanced, generation)
	}
	return nil
}

func (s *RedisStore) configured() error {
	if s == nil || s.redis == nil || s.generations == nil || s.states == nil {
		return fmt.Errorf("sms rate limit cache generation dependencies are not configured")
	}
	return s.scope.Validate()
}

func snapshotFromCatalog(generation int64, catalog Catalog) (policySnapshot, error) {
	if generation < 1 || catalog.PlatformID < 1 || len(catalog.Policies) != len(FixedPolicies()) {
		return policySnapshot{}, fmt.Errorf("sms rate limit catalog is invalid")
	}
	snapshot := policySnapshot{
		SchemaVersion: snapshotSchemaVersion, Generation: generation,
		PlatformID: catalog.PlatformID, PlatformCode: catalog.PlatformCode, PlatformName: catalog.PlatformName,
		Policies: make(map[string]snapshotPolicy, len(catalog.Policies)),
	}
	for _, policy := range catalog.Policies {
		fixed, found := FixedPolicyByKey(policy.Key)
		if !found || policy.Mode != fixed.Mode || policy.Dimension != fixed.Dimension ||
			policy.Limit < minLimit || policy.Limit > maxLimit ||
			policy.WindowSeconds < minWindowSeconds || policy.WindowSeconds > maxWindowSeconds {
			return policySnapshot{}, fmt.Errorf("sms rate limit policy %q is invalid", policy.Key)
		}
		if _, exists := snapshot.Policies[policy.Key]; exists {
			return policySnapshot{}, fmt.Errorf("sms rate limit policy %q is duplicated", policy.Key)
		}
		snapshot.Policies[policy.Key] = snapshotPolicy{Limit: policy.Limit, WindowSeconds: policy.WindowSeconds}
	}
	if err := validateSnapshot(snapshot); err != nil {
		return policySnapshot{}, err
	}
	return snapshot, nil
}

func catalogFromSnapshot(snapshot policySnapshot) Catalog {
	catalog := Catalog{
		PlatformID: snapshot.PlatformID, PlatformCode: snapshot.PlatformCode, PlatformName: snapshot.PlatformName,
		Policies: make([]Model, 0, len(FixedPolicies())),
	}
	for _, fixed := range FixedPolicies() {
		policy := snapshot.Policies[fixed.Key]
		catalog.Policies = append(catalog.Policies, Model{
			PlatformID: snapshot.PlatformID, Key: fixed.Key, Mode: fixed.Mode, Dimension: fixed.Dimension,
			Limit: policy.Limit, WindowSeconds: policy.WindowSeconds,
		})
	}
	return catalog
}

func encodeSnapshot(snapshot policySnapshot) (string, error) {
	if err := validateSnapshot(snapshot); err != nil {
		return "", err
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("encode sms rate limit snapshot: %w", err)
	}
	return string(payload), nil
}

func decodeSnapshot(raw string, generation, platformID int64) (policySnapshot, error) {
	if err := rejectDuplicateSnapshotKeys(raw); err != nil {
		return policySnapshot{}, fmt.Errorf("%w: %v", errSnapshotCorrupt, err)
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	var snapshot policySnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return policySnapshot{}, fmt.Errorf("%w: decode payload: %v", errSnapshotCorrupt, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return policySnapshot{}, fmt.Errorf("%w: trailing data", errSnapshotCorrupt)
	}
	if snapshot.Generation != generation || snapshot.PlatformID != platformID {
		return policySnapshot{}, fmt.Errorf("%w: coordinates are invalid", errSnapshotCorrupt)
	}
	if err := validateSnapshot(snapshot); err != nil {
		return policySnapshot{}, fmt.Errorf("%w: %v", errSnapshotCorrupt, err)
	}
	return snapshot, nil
}

func validateSnapshot(snapshot policySnapshot) error {
	if snapshot.SchemaVersion != snapshotSchemaVersion || snapshot.Generation < 1 || snapshot.PlatformID < 1 {
		return fmt.Errorf("sms rate limit snapshot coordinates are invalid")
	}
	if len(snapshot.Policies) != len(FixedPolicies()) {
		return fmt.Errorf("sms rate limit snapshot policy set is incomplete")
	}
	for _, fixed := range FixedPolicies() {
		policy, found := snapshot.Policies[fixed.Key]
		if !found || policy.Limit < minLimit || policy.Limit > maxLimit ||
			policy.WindowSeconds < minWindowSeconds || policy.WindowSeconds > maxWindowSeconds {
			return fmt.Errorf("sms rate limit snapshot policy %q is invalid", fixed.Key)
		}
	}
	return nil
}

func rejectDuplicateSnapshotKeys(raw string) error {
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	if err := scanSnapshotJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("sms rate limit snapshot contains trailing data")
	}
	return nil
}

func scanSnapshotJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("sms rate limit snapshot key is invalid")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("sms rate limit snapshot contains duplicate field %q", key)
			}
			seen[key] = struct{}{}
			if err := scanSnapshotJSONValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanSnapshotJSONValue(decoder); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("sms rate limit snapshot delimiter is invalid")
	}
	_, err = decoder.Token()
	return err
}

func waitForSMSRateLimit(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
