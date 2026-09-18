package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"admin/server/internal/module/message/sms/recipientRule"
	"admin/server/internal/module/message/sms/template"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/cacheFill"
	"admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/yesno"
)

const (
	runtimeCacheSchemaVersion = 1
	runtimeSnapshotVariant    = "runtime"
	runtimeSnapshotTTL        = 10 * time.Minute
	readinessTTL              = 10 * time.Minute
	smsReadBudget             = 500 * time.Millisecond
	smsWriteBudget            = 500 * time.Millisecond
	smsWaitStep               = 20 * time.Millisecond
)

var (
	smsGenerationScope        = cachegeneration.Scope{Namespace: "message.sms", ScopeKey: "global"}
	ErrRuntimeSnapshotCorrupt = errors.New("sms runtime snapshot is corrupt")
	ErrReadinessCorrupt       = errors.New("sms readiness snapshot is corrupt")
	ErrSMSGenerationAdvanced  = errors.New("sms cache generation advanced during fill")
)

func CacheGenerationScope() cachegeneration.Scope { return smsGenerationScope }

// ConfiguredConfig is the encrypted runtime view of the SMS configuration.
// Credentials stay encrypted in Redis and are decrypted only immediately before
// a provider call.
type ConfiguredConfig struct {
	SecretIDCiphertext  string
	SecretKeyCiphertext string
	SDKAppID            string
	SignName            string
	Region              string
	Endpoint            string
	TTLMinutes          int
	IsEnabled           yesno.Value
}

type TemplateFact struct {
	Name              string
	TencentTemplateID string
	VariableKeys      []string
	ExampleVariables  map[string]string
	IsEnabled         yesno.Value
}

type RuleFact struct {
	ID                int64
	Scope             string
	Action            string
	PatternCiphertext string
	PatternHint       string
	IsEnabled         yesno.Value
}

// RuntimeFacts is everything the sending path needs, loaded from PostgreSQL.
type RuntimeFacts struct {
	Configured bool
	Config     ConfiguredConfig
	Templates  map[string]TemplateFact
	Rules      []RuleFact
}

type runtimeSnapshot struct {
	SchemaVersion       int                    `json:"schemaVersion"`
	Generation          int64                  `json:"generation"`
	Configured          bool                   `json:"configured"`
	SecretIDCiphertext  string                 `json:"secretIdCiphertext"`
	SecretKeyCiphertext string                 `json:"secretKeyCiphertext"`
	SDKAppID            string                 `json:"smsSdkAppId"`
	SignName            string                 `json:"signName"`
	Region              string                 `json:"region"`
	Endpoint            string                 `json:"endpoint"`
	TTLMinutes          int                    `json:"ttlMinutes"`
	IsEnabled           int16                  `json:"isEnabled"`
	Templates           map[string]templateRow `json:"templates"`
	Rules               []ruleRow              `json:"rules"`
}

type templateRow struct {
	Name              string            `json:"name"`
	TencentTemplateID string            `json:"tencentTemplateId"`
	VariableKeys      []string          `json:"variableKeys"`
	ExampleVariables  map[string]string `json:"exampleVariables"`
	IsEnabled         int16             `json:"isEnabled"`
}

type ruleRow struct {
	ID                int64  `json:"id"`
	Scope             string `json:"scope"`
	Action            string `json:"action"`
	PatternCiphertext string `json:"patternCiphertext"`
	PatternHint       string `json:"patternHint"`
	IsEnabled         int16  `json:"isEnabled"`
}

type readinessPayload struct {
	SchemaVersion int   `json:"schemaVersion"`
	Generation    int64 `json:"generation"`
	Ready         bool  `json:"ready"`
	TTLMinutes    int   `json:"ttlMinutes"`
}

// RuntimeCache owns only the mechanical generation protocol and SMS snapshot
// codecs. Business loaders remain explicit at the call site.
type RuntimeCache struct {
	redis       *projectredis.Client
	generations *cachegeneration.Repository
	states      *cachegeneration.Store
	scope       cachegeneration.Scope
	logger      *slog.Logger

	runtimeTTL    time.Duration
	readinessTTL  time.Duration
	renewInterval time.Duration
	readBudget    time.Duration
	writeBudget   time.Duration
	waitStep      time.Duration
	now           func() time.Time
	wait          func(context.Context, time.Duration) error
}

func NewRuntimeCache(redis *projectredis.Client) *RuntimeCache {
	return &RuntimeCache{
		redis: redis, scope: smsGenerationScope,
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		runtimeTTL: runtimeSnapshotTTL, readinessTTL: readinessTTL,
		renewInterval: cachegeneration.MutationRenewInterval,
		readBudget:    smsReadBudget, writeBudget: smsWriteBudget, waitStep: smsWaitStep,
		now: time.Now, wait: waitForSMSCache,
	}
}

func (c *RuntimeCache) SetGenerations(repository *cachegeneration.Repository, store *cachegeneration.Store) {
	c.generations = repository
	c.states = store
}

func (c *RuntimeCache) SetLogger(logger *slog.Logger) {
	if logger != nil {
		c.logger = logger
	}
}

func (c *RuntimeCache) ValidateDependencies() error { return c.configured() }

// Load returns the current immutable runtime variant. Redis is mandatory on the
// hot path; PostgreSQL is admitted only behind a bounded cross-instance fill.
func (c *RuntimeCache) Load(ctx context.Context, load func(context.Context) (RuntimeFacts, error)) (RuntimeFacts, error) {
	if err := c.configured(); err != nil || load == nil {
		if err == nil {
			err = fmt.Errorf("sms runtime loader is unavailable")
		}
		return RuntimeFacts{}, err
	}
	deadline := c.now().Add(c.readBudget)
	for c.now().Before(deadline) {
		state, retry, err := c.readyState(ctx, deadline, func() error {
			return c.recoverRuntimeState(ctx, load)
		})
		if err != nil {
			return RuntimeFacts{}, err
		}
		if retry {
			continue
		}
		facts, found, err := c.read(ctx, state.Generation)
		if err != nil && !errors.Is(err, ErrRuntimeSnapshotCorrupt) {
			return RuntimeFacts{}, err
		}
		if err == nil && found {
			return facts, nil
		}
		if err := c.fillRuntime(ctx, state.Generation, load); err != nil {
			if errors.Is(err, ErrSMSGenerationAdvanced) {
				continue
			}
			return RuntimeFacts{}, err
		}
	}
	return RuntimeFacts{}, fmt.Errorf("sms runtime cache read budget exhausted")
}

func (c *RuntimeCache) LoadReadiness(ctx context.Context, scene string, compute func(context.Context) (VerifyCodeReadiness, error)) (VerifyCodeReadiness, error) {
	variant, err := readinessVariant(scene)
	if err != nil {
		return VerifyCodeReadiness{}, err
	}
	if err := c.configured(); err != nil || compute == nil {
		if err == nil {
			err = fmt.Errorf("sms readiness loader is unavailable")
		}
		return VerifyCodeReadiness{}, err
	}
	deadline := c.now().Add(c.readBudget)
	for c.now().Before(deadline) {
		state, retry, err := c.readyState(ctx, deadline, func() error {
			return c.recoverReadinessState(ctx, scene, variant, compute)
		})
		if err != nil {
			return VerifyCodeReadiness{}, err
		}
		if retry {
			continue
		}
		readiness, found, err := c.readReadiness(ctx, state.Generation, variant)
		if err != nil && !errors.Is(err, ErrReadinessCorrupt) {
			return VerifyCodeReadiness{}, err
		}
		if err == nil && found {
			return readiness, nil
		}
		if err := c.fillReadiness(ctx, state.Generation, variant, compute); err != nil {
			if errors.Is(err, ErrSMSGenerationAdvanced) {
				continue
			}
			return VerifyCodeReadiness{}, err
		}
	}
	return VerifyCodeReadiness{}, fmt.Errorf("sms readiness cache read budget exhausted")
}

func readinessVariant(scene string) (string, error) {
	if _, found := sceneOf(scene); !found {
		return "", fmt.Errorf("sms readiness scene is invalid")
	}
	return "readiness:" + scene, nil
}

func (c *RuntimeCache) readyState(ctx context.Context, deadline time.Time, repair func() error) (cachegeneration.State, bool, error) {
	if err := ctx.Err(); err != nil {
		return cachegeneration.State{}, false, err
	}
	state, found, err := c.states.Read(ctx, c.scope)
	if err != nil {
		if errors.Is(err, cachegeneration.ErrStateCorrupt) {
			if repairErr := repair(); repairErr != nil {
				if errors.Is(repairErr, ErrSMSGenerationAdvanced) {
					return cachegeneration.State{}, true, nil
				}
				return cachegeneration.State{}, false, errors.Join(err, repairErr)
			}
			return cachegeneration.State{}, true, nil
		}
		return cachegeneration.State{}, false, err
	}
	if !found {
		if repairErr := repair(); repairErr != nil {
			if errors.Is(repairErr, ErrSMSGenerationAdvanced) {
				return cachegeneration.State{}, true, nil
			}
			return cachegeneration.State{}, false, errors.Join(cachegeneration.ErrStateMissing, repairErr)
		}
		return cachegeneration.State{}, true, nil
	}
	if state.State == cachegeneration.StateInvalidating {
		if c.now().Add(c.waitStep).After(deadline) {
			return cachegeneration.State{}, false, fmt.Errorf("sms cache state is invalidating")
		}
		if err := c.wait(ctx, c.waitStep); err != nil {
			return cachegeneration.State{}, false, err
		}
		return cachegeneration.State{}, true, nil
	}
	if state.State != cachegeneration.StateReady {
		return cachegeneration.State{}, false, fmt.Errorf("sms cache state is invalid")
	}
	return state, false, nil
}

func (c *RuntimeCache) fillRuntime(ctx context.Context, generation int64, load func(context.Context) (RuntimeFacts, error)) error {
	lease, err := c.tryFill(ctx, generation, runtimeSnapshotVariant)
	if err != nil {
		return err
	}
	if lease == nil {
		return c.wait(ctx, c.waitStep)
	}
	defer func() { _ = lease.Release(ctx) }()
	workContext, cancel := lease.WorkContext(ctx)
	defer cancel()
	if _, found, err := c.read(workContext, generation); err == nil && found {
		return nil
	}
	facts, err := load(workContext)
	if err != nil {
		return err
	}
	if err := c.confirmReadyGeneration(workContext, generation); err != nil {
		return err
	}
	return c.write(workContext, generation, facts)
}

func (c *RuntimeCache) fillReadiness(ctx context.Context, generation int64, variant string, compute func(context.Context) (VerifyCodeReadiness, error)) error {
	lease, err := c.tryFill(ctx, generation, variant)
	if err != nil {
		return err
	}
	if lease == nil {
		return c.wait(ctx, c.waitStep)
	}
	defer func() { _ = lease.Release(ctx) }()
	workContext, cancel := lease.WorkContext(ctx)
	defer cancel()
	if _, found, err := c.readReadiness(workContext, generation, variant); err == nil && found {
		return nil
	}
	readiness, err := compute(workContext)
	if err != nil {
		return err
	}
	if err := c.confirmReadyGeneration(workContext, generation); err != nil {
		return err
	}
	return c.writeReadiness(workContext, generation, variant, readiness)
}

func (c *RuntimeCache) recoverRuntimeState(ctx context.Context, load func(context.Context) (RuntimeFacts, error)) error {
	lease, err := c.tryRepair(ctx, runtimeSnapshotVariant)
	if err != nil {
		return err
	}
	if lease == nil {
		return c.wait(ctx, c.waitStep)
	}
	defer func() { _ = lease.Release(ctx) }()
	workContext, cancel := lease.WorkContext(ctx)
	defer cancel()
	generation, err := c.generations.Current(workContext, c.scope)
	if err != nil {
		return err
	}
	if _, err := c.states.Reconcile(workContext, c.scope, generation); err != nil {
		return err
	}
	facts, err := load(workContext)
	if err != nil {
		return err
	}
	if err := c.confirmReadyGeneration(workContext, generation); err != nil {
		return err
	}
	return c.write(workContext, generation, facts)
}

func (c *RuntimeCache) recoverReadinessState(ctx context.Context, scene, variant string, compute func(context.Context) (VerifyCodeReadiness, error)) error {
	lease, err := c.tryRepair(ctx, variant)
	if err != nil {
		return err
	}
	if lease == nil {
		return c.wait(ctx, c.waitStep)
	}
	defer func() { _ = lease.Release(ctx) }()
	workContext, cancel := lease.WorkContext(ctx)
	defer cancel()
	generation, err := c.generations.Current(workContext, c.scope)
	if err != nil {
		return err
	}
	if _, err := c.states.Reconcile(workContext, c.scope, generation); err != nil {
		return err
	}
	readiness, err := compute(workContext)
	if err != nil {
		return err
	}
	if err := c.confirmReadyGeneration(workContext, generation); err != nil {
		return err
	}
	_ = scene
	return c.writeReadiness(workContext, generation, variant, readiness)
}

func (c *RuntimeCache) tryFill(ctx context.Context, generation int64, variant string) (*cachefill.Lease, error) {
	target, err := cachegeneration.FillKey(c.scope, generation, variant)
	if err != nil {
		return nil, err
	}
	return cachefill.Try(ctx, c.redis.UniversalClient(), cachegeneration.StateKey(c.scope), target)
}

func (c *RuntimeCache) tryRepair(ctx context.Context, variant string) (*cachefill.Lease, error) {
	return cachefill.Try(ctx, c.redis.UniversalClient(), cachegeneration.StateKey(c.scope), "repair:"+variant)
}

func (c *RuntimeCache) read(ctx context.Context, generation int64) (RuntimeFacts, bool, error) {
	key, err := cachegeneration.SnapshotKey(c.scope, generation, runtimeSnapshotVariant)
	if err != nil {
		return RuntimeFacts{}, false, err
	}
	raw, found, err := c.redis.GetString(ctx, key)
	if err != nil || !found {
		return RuntimeFacts{}, found, err
	}
	snapshot, err := decodeRuntimeSnapshot(raw, generation)
	if err != nil {
		return RuntimeFacts{}, false, err
	}
	return factsOf(snapshot), true, nil
}

func (c *RuntimeCache) write(ctx context.Context, generation int64, facts RuntimeFacts) error {
	payload, err := encodeRuntimeSnapshot(generation, facts)
	if err != nil {
		return err
	}
	key, err := cachegeneration.SnapshotKey(c.scope, generation, runtimeSnapshotVariant)
	if err != nil {
		return err
	}
	return c.redis.SetString(ctx, key, payload, c.runtimeTTL)
}

func (c *RuntimeCache) readReadiness(ctx context.Context, generation int64, variant string) (VerifyCodeReadiness, bool, error) {
	key, err := cachegeneration.SnapshotKey(c.scope, generation, variant)
	if err != nil {
		return VerifyCodeReadiness{}, false, err
	}
	raw, found, err := c.redis.GetString(ctx, key)
	if err != nil || !found {
		return VerifyCodeReadiness{}, found, err
	}
	return decodeReadiness(raw, generation)
}

func (c *RuntimeCache) writeReadiness(ctx context.Context, generation int64, variant string, readiness VerifyCodeReadiness) error {
	payload, err := encodeReadiness(generation, readiness)
	if err != nil {
		return err
	}
	key, err := cachegeneration.SnapshotKey(c.scope, generation, variant)
	if err != nil {
		return err
	}
	return c.redis.SetString(ctx, key, payload, c.readinessTTL)
}

func (c *RuntimeCache) confirmReadyGeneration(ctx context.Context, generation int64) error {
	state, found, err := c.states.Read(ctx, c.scope)
	if err != nil {
		return err
	}
	if !found || state.State != cachegeneration.StateReady || state.Generation != generation {
		return fmt.Errorf("%w: expected generation %d", ErrSMSGenerationAdvanced, generation)
	}
	return nil
}

// Mutate serializes one business transaction against the shared SMS scope.
// The callback owns PostgreSQL business writes plus AdvanceTx/outbox.
func (c *RuntimeCache) Mutate(ctx context.Context, change func(context.Context, int64) (cachegeneration.MutationResult, error)) error {
	if err := c.configured(); err != nil || change == nil {
		if err == nil {
			err = fmt.Errorf("sms runtime mutation callback is missing")
		}
		return err
	}
	deadline := c.now().Add(c.writeBudget)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := c.mutateOnce(ctx, change)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, cachegeneration.ErrUpdating), errors.Is(err, cachegeneration.ErrGenerationChanged):
			if !c.now().Before(deadline) {
				return fmt.Errorf("sms mutation budget exhausted: %w", err)
			}
			if waitErr := c.wait(ctx, c.waitStep); waitErr != nil {
				return waitErr
			}
		default:
			return err
		}
	}
}

func (c *RuntimeCache) mutateOnce(ctx context.Context, change func(context.Context, int64) (cachegeneration.MutationResult, error)) error {
	base, err := c.generations.Current(ctx, c.scope)
	if err != nil {
		return err
	}
	lease, err := c.acquireLease(ctx, base)
	if err != nil {
		return err
	}
	mutationContext, stopRenewal := c.startRenewal(ctx, lease)
	result, applyErr := change(mutationContext, base)
	stopRenewal()
	if applyErr != nil {
		return c.finishFailedMutation(ctx, lease, base, applyErr)
	}
	if !result.Changed {
		return lease.Rollback(ctx)
	}
	if err := lease.Commit(ctx, result.Generation); err != nil {
		c.logGenerationFailure("publish sms generation failed", result.Generation, result.OutboxID, err)
		return nil
	}
	if _, err := c.generations.MarkPublishedIfUnclaimed(ctx, result.OutboxID, c.now().UTC()); err != nil {
		c.logGenerationFailure("mark sms outbox published failed", result.Generation, result.OutboxID, err)
	}
	return nil
}

func (c *RuntimeCache) acquireLease(ctx context.Context, base int64) (*cachegeneration.Lease, error) {
	lease, err := c.states.Acquire(ctx, c.scope, base)
	if err == nil {
		return lease, nil
	}
	if !errors.Is(err, cachegeneration.ErrStateMissing) && !errors.Is(err, cachegeneration.ErrStateCorrupt) {
		return nil, err
	}
	if _, reconcileErr := c.states.Reconcile(ctx, c.scope, base); reconcileErr != nil {
		return nil, errors.Join(err, reconcileErr)
	}
	return c.states.Acquire(ctx, c.scope, base)
}

func (c *RuntimeCache) startRenewal(parent context.Context, lease *cachegeneration.Lease) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(c.renewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := lease.Renew(ctx); err != nil {
					cancel(err)
					return
				}
			}
		}
	}()
	return ctx, func() {
		cancel(nil)
		<-done
	}
}

func (c *RuntimeCache) finishFailedMutation(ctx context.Context, lease *cachegeneration.Lease, base int64, applyErr error) error {
	current, readErr := c.generations.Current(ctx, c.scope)
	if readErr != nil {
		return errors.Join(applyErr, readErr)
	}
	if current > base {
		if err := lease.Commit(ctx, current); err != nil {
			c.logGenerationFailure("publish sms generation after uncertain commit failed", current, 0, err)
		}
		if errors.Is(applyErr, cachegeneration.ErrMutationRolledBack) {
			return applyErr
		}
		return nil
	}
	if err := lease.Rollback(ctx); err != nil {
		return errors.Join(applyErr, err)
	}
	return applyErr
}

func (c *RuntimeCache) configured() error {
	if c == nil || c.redis == nil || c.generations == nil || c.states == nil {
		return fmt.Errorf("sms cache generation dependencies are not configured")
	}
	return c.scope.Validate()
}

func encodeReadiness(generation int64, readiness VerifyCodeReadiness) (string, error) {
	if generation < 1 {
		return "", fmt.Errorf("%w: generation is invalid", ErrReadinessCorrupt)
	}
	if err := validateReadinessValue(readiness); err != nil {
		return "", fmt.Errorf("%w: %v", ErrReadinessCorrupt, err)
	}
	payload, err := json.Marshal(readinessPayload{
		SchemaVersion: runtimeCacheSchemaVersion, Generation: generation,
		Ready: readiness.Ready, TTLMinutes: readiness.TTLMinutes,
	})
	if err != nil {
		return "", fmt.Errorf("encode sms readiness: %w", err)
	}
	return string(payload), nil
}

func decodeReadiness(raw string, generation int64) (VerifyCodeReadiness, bool, error) {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return VerifyCodeReadiness{}, false, fmt.Errorf("%w: %v", ErrReadinessCorrupt, err)
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	var payload readinessPayload
	if err := decoder.Decode(&payload); err != nil {
		return VerifyCodeReadiness{}, false, fmt.Errorf("%w: decode payload: %v", ErrReadinessCorrupt, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return VerifyCodeReadiness{}, false, fmt.Errorf("%w: trailing data", ErrReadinessCorrupt)
	}
	if payload.SchemaVersion != runtimeCacheSchemaVersion || payload.Generation != generation || generation < 1 {
		return VerifyCodeReadiness{}, false, fmt.Errorf("%w: coordinates are invalid", ErrReadinessCorrupt)
	}
	readiness := VerifyCodeReadiness{Ready: payload.Ready, TTLMinutes: payload.TTLMinutes}
	if err := validateReadinessValue(readiness); err != nil {
		return VerifyCodeReadiness{}, false, fmt.Errorf("%w: %v", ErrReadinessCorrupt, err)
	}
	return readiness, true, nil
}

func validateReadinessValue(readiness VerifyCodeReadiness) error {
	if readiness.Ready && (readiness.TTLMinutes < 1 || readiness.TTLMinutes > 60) {
		return fmt.Errorf("sms readiness ttl is invalid")
	}
	if !readiness.Ready && readiness.TTLMinutes != 0 {
		return fmt.Errorf("sms readiness disabled ttl is invalid")
	}
	return nil
}

func encodeRuntimeSnapshot(generation int64, facts RuntimeFacts) (string, error) {
	snapshot := snapshotOf(generation, facts)
	if err := validateRuntimeSnapshot(snapshot); err != nil {
		return "", fmt.Errorf("%w: %v", ErrRuntimeSnapshotCorrupt, err)
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("encode sms runtime snapshot: %w", err)
	}
	return string(payload), nil
}

func snapshotOf(generation int64, facts RuntimeFacts) runtimeSnapshot {
	snapshot := runtimeSnapshot{
		SchemaVersion: runtimeCacheSchemaVersion, Generation: generation,
		Configured:         facts.Configured,
		SecretIDCiphertext: facts.Config.SecretIDCiphertext, SecretKeyCiphertext: facts.Config.SecretKeyCiphertext,
		SDKAppID: facts.Config.SDKAppID, SignName: facts.Config.SignName,
		Region: facts.Config.Region, Endpoint: facts.Config.Endpoint,
		TTLMinutes: facts.Config.TTLMinutes, IsEnabled: int16(facts.Config.IsEnabled),
		Templates: make(map[string]templateRow, len(facts.Templates)),
		Rules:     make([]ruleRow, 0, len(facts.Rules)),
	}
	for scene, value := range facts.Templates {
		snapshot.Templates[scene] = templateRow{
			Name: value.Name, TencentTemplateID: value.TencentTemplateID,
			VariableKeys: append([]string(nil), value.VariableKeys...), ExampleVariables: cloneStringMap(value.ExampleVariables),
			IsEnabled: int16(value.IsEnabled),
		}
	}
	for _, rule := range facts.Rules {
		snapshot.Rules = append(snapshot.Rules, ruleRow{
			ID: rule.ID, Scope: rule.Scope, Action: rule.Action,
			PatternCiphertext: rule.PatternCiphertext, PatternHint: rule.PatternHint,
			IsEnabled: int16(rule.IsEnabled),
		})
	}
	return snapshot
}

func decodeRuntimeSnapshot(raw string, generation int64) (runtimeSnapshot, error) {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return runtimeSnapshot{}, fmt.Errorf("%w: %v", ErrRuntimeSnapshotCorrupt, err)
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	var snapshot runtimeSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return runtimeSnapshot{}, fmt.Errorf("%w: decode payload: %v", ErrRuntimeSnapshotCorrupt, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return runtimeSnapshot{}, fmt.Errorf("%w: trailing data", ErrRuntimeSnapshotCorrupt)
	}
	if snapshot.SchemaVersion != runtimeCacheSchemaVersion || snapshot.Generation != generation || generation < 1 {
		return runtimeSnapshot{}, fmt.Errorf("%w: coordinates are invalid", ErrRuntimeSnapshotCorrupt)
	}
	if err := validateRuntimeSnapshot(snapshot); err != nil {
		return runtimeSnapshot{}, fmt.Errorf("%w: %v", ErrRuntimeSnapshotCorrupt, err)
	}
	return snapshot, nil
}

func validateRuntimeSnapshot(snapshot runtimeSnapshot) error {
	if snapshot.SchemaVersion != runtimeCacheSchemaVersion || snapshot.Generation < 1 {
		return fmt.Errorf("sms runtime snapshot coordinates are invalid")
	}
	if snapshot.Configured {
		if snapshot.SecretIDCiphertext == "" || snapshot.SecretKeyCiphertext == "" ||
			strings.TrimSpace(snapshot.SDKAppID) == "" || strings.TrimSpace(snapshot.SignName) == "" ||
			strings.TrimSpace(snapshot.Region) == "" || snapshot.TTLMinutes < 1 || snapshot.TTLMinutes > 60 ||
			!yesno.IsValid(yesno.Value(snapshot.IsEnabled)) {
			return fmt.Errorf("sms runtime config is invalid")
		}
	} else if snapshot.SecretIDCiphertext != "" || snapshot.SecretKeyCiphertext != "" || snapshot.SDKAppID != "" ||
		snapshot.SignName != "" || snapshot.Region != "" || snapshot.Endpoint != "" ||
		snapshot.TTLMinutes != 0 || snapshot.IsEnabled != int16(yesno.No) {
		return fmt.Errorf("sms runtime unconfigured state is invalid")
	}

	catalog := template.FixedCatalog()
	if len(snapshot.Templates) != len(catalog) {
		return fmt.Errorf("sms runtime template catalog is incomplete")
	}
	for _, fixed := range catalog {
		row, found := snapshot.Templates[fixed.Scene]
		if !found || strings.TrimSpace(row.Name) == "" || !yesno.IsValid(yesno.Value(row.IsEnabled)) ||
			!sameStrings(row.VariableKeys, fixed.VariableKeys) || !validRuntimeVariables(row.ExampleVariables, fixed.VariableKeys) {
			return fmt.Errorf("sms runtime template %q is invalid", fixed.Scene)
		}
		if yesno.Value(row.IsEnabled) == yesno.Yes && !isDecimal(row.TencentTemplateID) {
			return fmt.Errorf("sms runtime template %q provider id is invalid", fixed.Scene)
		}
	}

	seenRules := make(map[int64]struct{}, len(snapshot.Rules))
	for _, row := range snapshot.Rules {
		if row.ID < 1 || (row.Scope != recipientRule.ScopePhone && row.Scope != recipientRule.ScopePrefix) ||
			(row.Action != recipientRule.ActionAllow && row.Action != recipientRule.ActionDeny) ||
			row.PatternCiphertext == "" || strings.TrimSpace(row.PatternHint) == "" ||
			!yesno.IsValid(yesno.Value(row.IsEnabled)) {
			return fmt.Errorf("sms runtime recipient rule is invalid")
		}
		if _, exists := seenRules[row.ID]; exists {
			return fmt.Errorf("sms runtime recipient rule is duplicated")
		}
		seenRules[row.ID] = struct{}{}
	}
	return nil
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validRuntimeVariables(values map[string]string, keys []string) bool {
	if len(values) != len(keys) {
		return false
	}
	for _, key := range keys {
		if strings.TrimSpace(values[key]) == "" {
			return false
		}
	}
	return true
}

func isDecimal(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func rejectDuplicateJSONKeys(raw string) error {
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("sms runtime snapshot contains trailing data")
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
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
				return fmt.Errorf("sms runtime snapshot key is invalid")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("sms runtime snapshot contains duplicate field %q", key)
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected sms runtime snapshot delimiter %q", delimiter)
	}
	_, err = decoder.Token()
	return err
}

func factsOf(snapshot runtimeSnapshot) RuntimeFacts {
	facts := RuntimeFacts{
		Configured: snapshot.Configured,
		Config: ConfiguredConfig{
			SecretIDCiphertext: snapshot.SecretIDCiphertext, SecretKeyCiphertext: snapshot.SecretKeyCiphertext,
			SDKAppID: snapshot.SDKAppID, SignName: snapshot.SignName,
			Region: snapshot.Region, Endpoint: snapshot.Endpoint,
			TTLMinutes: snapshot.TTLMinutes, IsEnabled: yesno.Value(snapshot.IsEnabled),
		},
		Templates: make(map[string]TemplateFact, len(snapshot.Templates)),
		Rules:     make([]RuleFact, 0, len(snapshot.Rules)),
	}
	for scene, row := range snapshot.Templates {
		facts.Templates[scene] = TemplateFact{
			Name: row.Name, TencentTemplateID: row.TencentTemplateID,
			VariableKeys: append([]string(nil), row.VariableKeys...), ExampleVariables: cloneStringMap(row.ExampleVariables),
			IsEnabled: yesno.Value(row.IsEnabled),
		}
	}
	for _, row := range snapshot.Rules {
		facts.Rules = append(facts.Rules, RuleFact{
			ID: row.ID, Scope: row.Scope, Action: row.Action,
			PatternCiphertext: row.PatternCiphertext, PatternHint: row.PatternHint,
			IsEnabled: yesno.Value(row.IsEnabled),
		})
	}
	return facts
}

func cloneStringMap(values map[string]string) map[string]string {
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func waitForSMSCache(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *RuntimeCache) logGenerationFailure(message string, generation, outboxID int64, err error) {
	c.logger.Error(message,
		"namespace", c.scope.Namespace,
		"scopeKey", c.scope.ScopeKey,
		"generation", generation,
		"outboxId", outboxID,
		"errorClass", cachegeneration.ErrorClass(err),
	)
}
