package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	mailtemplate "admin/server/internal/module/message/mail/template"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/cacheFill"
	"admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/yesno"

	"gorm.io/gorm"
)

const (
	mailCacheSchemaVersion = 1
	mailSnapshotTTL        = 10 * time.Minute
	mailReadBudget         = 500 * time.Millisecond
	mailWriteBudget        = 500 * time.Millisecond
	mailWaitStep           = 20 * time.Millisecond
)

var (
	mailGenerationScope       = cachegeneration.Scope{Namespace: "message.mail", ScopeKey: "global"}
	ErrRuntimeSnapshotCorrupt = errors.New("mail runtime snapshot is corrupt")
	ErrMailGenerationAdvanced = errors.New("mail cache generation advanced during fill")
)

func CacheGenerationScope() cachegeneration.Scope { return mailGenerationScope }

type runtimeSnapshot struct {
	SchemaVersion int             `json:"schemaVersion"`
	Generation    int64           `json:"generation"`
	Config        runtimeConfig   `json:"config"`
	Templates     []Template      `json:"templates"`
	Rules         []RecipientRule `json:"rules"`
}

// runtimeConfig intentionally carries encrypted credentials. It is an
// internal Redis DTO and is never serialized by an HTTP handler.
type runtimeConfig struct {
	ID                  int64       `json:"id"`
	SecretIDCiphertext  string      `json:"secretIdCiphertext"`
	SecretKeyCiphertext string      `json:"secretKeyCiphertext"`
	Region              string      `json:"region"`
	Endpoint            *string     `json:"endpoint"`
	FromEmail           string      `json:"fromEmail"`
	FromName            string      `json:"fromName"`
	ReplyTo             *string     `json:"replyTo"`
	TTLMinutes          int16       `json:"ttlMinutes"`
	IsEnabled           yesno.Value `json:"isEnabled"`
	CreatedAt           time.Time   `json:"createdAt"`
	UpdatedAt           time.Time   `json:"updatedAt"`
}

func runtimeConfigFromModel(value Config) runtimeConfig {
	return runtimeConfig{
		ID: value.ID, SecretIDCiphertext: value.SecretIDCiphertext,
		SecretKeyCiphertext: value.SecretKeyCiphertext, Region: value.Region,
		Endpoint: value.Endpoint, FromEmail: value.FromEmail, FromName: value.FromName,
		ReplyTo: value.ReplyTo, TTLMinutes: value.TTLMinutes, IsEnabled: value.IsEnabled,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func (c runtimeConfig) valid() error {
	if c.ID < 1 || c.SecretIDCiphertext == "" || c.SecretKeyCiphertext == "" ||
		c.Region == "" || c.FromEmail == "" || c.FromName == "" ||
		c.TTLMinutes < 1 || !yesno.IsValid(c.IsEnabled) || c.UpdatedAt.IsZero() {
		return fmt.Errorf("mail runtime config is invalid")
	}
	return nil
}

type runtimeRepository interface {
	FindConfig(context.Context) (Config, error)
	FindTemplateByScene(context.Context, string) (Template, error)
	ListTemplates(context.Context) ([]Template, error)
	ListRecipientRules(context.Context) ([]RecipientRule, error)
}

type runtimeStore struct {
	repository  runtimeRepository
	redis       *projectredis.Client
	generations *cachegeneration.Repository
	states      *cachegeneration.Store
	scope       cachegeneration.Scope
	logger      *slog.Logger
	ttl         time.Duration

	renewInterval time.Duration
	readBudget    time.Duration
	writeBudget   time.Duration
	waitStep      time.Duration
	now           func() time.Time
	wait          func(context.Context, time.Duration) error
}

func newRuntimeStore(repository runtimeRepository, redis *projectredis.Client) *runtimeStore {
	return &runtimeStore{
		repository: repository, redis: redis, scope: mailGenerationScope,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)), ttl: mailSnapshotTTL,
		renewInterval: cachegeneration.MutationRenewInterval,
		readBudget:    mailReadBudget, writeBudget: mailWriteBudget, waitStep: mailWaitStep,
		now: time.Now, wait: waitForMailCache,
	}
}

func NewRuntimeStore(stores *Stores, redis *projectredis.Client) *runtimeStore {
	return newRuntimeStore(stores, redis)
}

func (s *runtimeStore) SetGenerations(repository *cachegeneration.Repository, store *cachegeneration.Store) {
	s.generations = repository
	s.states = store
}

func (s *runtimeStore) SetLogger(logger *slog.Logger) {
	if logger != nil {
		s.logger = logger
	}
}

func (s *runtimeStore) ValidateDependencies() error { return s.configured() }

func runtimeVariant(scene string) (string, error) {
	if _, ok := mailtemplate.FindFixed(scene); !ok {
		return "", fmt.Errorf("mail runtime scene is invalid")
	}
	return "runtime:" + scene, nil
}

func (s *runtimeStore) Load(ctx context.Context, scene string) (runtimeSnapshot, error) {
	variant, err := runtimeVariant(scene)
	if err != nil {
		return runtimeSnapshot{}, err
	}
	if err := s.configured(); err != nil {
		return runtimeSnapshot{}, err
	}
	deadline := s.now().Add(s.readBudget)
	for s.now().Before(deadline) {
		state, retry, err := s.readyState(ctx, deadline, func() error { return s.recoverRuntimeState(ctx, scene, variant) })
		if err != nil {
			return runtimeSnapshot{}, err
		}
		if retry {
			continue
		}
		snapshot, found, err := s.readRuntimeSnapshot(ctx, state.Generation, scene, variant)
		if err != nil && !errors.Is(err, ErrRuntimeSnapshotCorrupt) {
			return runtimeSnapshot{}, err
		}
		if err == nil && found {
			return snapshot, nil
		}
		if err := s.fillRuntime(ctx, state.Generation, scene, variant); err != nil {
			if errors.Is(err, ErrMailGenerationAdvanced) {
				continue
			}
			return runtimeSnapshot{}, err
		}
	}
	return runtimeSnapshot{}, fmt.Errorf("mail runtime cache read budget exhausted")
}

func (s *runtimeStore) LoadReadiness(ctx context.Context, scene string) (VerifyCodeReadiness, error) {
	variant, err := readinessVariant(scene)
	if err != nil {
		return VerifyCodeReadiness{}, err
	}
	if err := s.configured(); err != nil {
		return VerifyCodeReadiness{}, err
	}
	deadline := s.now().Add(s.readBudget)
	for s.now().Before(deadline) {
		state, retry, err := s.readyState(ctx, deadline, func() error { return s.recoverReadinessState(ctx, scene, variant) })
		if err != nil {
			return VerifyCodeReadiness{}, err
		}
		if retry {
			continue
		}
		readiness, found, err := s.readReadinessSnapshot(ctx, state.Generation, variant)
		if err != nil && !errors.Is(err, ErrReadinessSnapshotCorrupt) {
			return VerifyCodeReadiness{}, err
		}
		if err == nil && found {
			return readiness, nil
		}
		if err := s.fillReadiness(ctx, state.Generation, scene, variant); err != nil {
			if errors.Is(err, ErrMailGenerationAdvanced) {
				continue
			}
			return VerifyCodeReadiness{}, err
		}
	}
	return VerifyCodeReadiness{}, fmt.Errorf("mail readiness cache read budget exhausted")
}

func (s *runtimeStore) Current(ctx context.Context, scene string) (VerifyCodeReadiness, error) {
	return s.LoadReadiness(ctx, scene)
}

func (s *runtimeStore) readyState(ctx context.Context, deadline time.Time, repair func() error) (cachegeneration.State, bool, error) {
	if err := ctx.Err(); err != nil {
		return cachegeneration.State{}, false, err
	}
	state, found, err := s.states.Read(ctx, s.scope)
	if err != nil {
		if errors.Is(err, cachegeneration.ErrStateCorrupt) {
			if repairErr := repair(); repairErr != nil {
				if errors.Is(repairErr, ErrMailGenerationAdvanced) {
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
			if errors.Is(repairErr, ErrMailGenerationAdvanced) {
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

func (s *runtimeStore) fillRuntime(ctx context.Context, generation int64, scene, variant string) error {
	lease, err := s.tryFill(ctx, generation, variant)
	if err != nil {
		return err
	}
	if lease == nil {
		return s.wait(ctx, s.waitStep)
	}
	defer func() { _ = lease.Release(ctx) }()
	workCtx, cancel := lease.WorkContext(ctx)
	defer cancel()
	if _, found, err := s.readRuntimeSnapshot(workCtx, generation, scene, variant); err == nil && found {
		return nil
	}
	snapshot, err := s.loadRuntimeFromDatabase(workCtx, generation, scene)
	if err != nil {
		return err
	}
	if err := s.confirmReadyGeneration(workCtx, generation); err != nil {
		return err
	}
	return s.putRuntimeSnapshot(workCtx, variant, snapshot)
}

func (s *runtimeStore) fillReadiness(ctx context.Context, generation int64, scene, variant string) error {
	lease, err := s.tryFill(ctx, generation, variant)
	if err != nil {
		return err
	}
	if lease == nil {
		return s.wait(ctx, s.waitStep)
	}
	defer func() { _ = lease.Release(ctx) }()
	workCtx, cancel := lease.WorkContext(ctx)
	defer cancel()
	if _, found, err := s.readReadinessSnapshot(workCtx, generation, variant); err == nil && found {
		return nil
	}
	readiness, err := s.loadReadinessFromDatabase(workCtx, scene)
	if err != nil {
		return err
	}
	if err := s.confirmReadyGeneration(workCtx, generation); err != nil {
		return err
	}
	return s.putReadinessSnapshot(workCtx, variant, newVerifyCodeReadinessSnapshot(generation, readiness))
}

func (s *runtimeStore) recoverRuntimeState(ctx context.Context, scene, variant string) error {
	lease, err := s.tryRepair(ctx, variant)
	if err != nil {
		return err
	}
	if lease == nil {
		return s.wait(ctx, s.waitStep)
	}
	defer func() { _ = lease.Release(ctx) }()
	workCtx, cancel := lease.WorkContext(ctx)
	defer cancel()
	generation, err := s.generations.Current(workCtx, s.scope)
	if err != nil {
		return err
	}
	if _, err := s.states.Reconcile(workCtx, s.scope, generation); err != nil {
		return err
	}
	snapshot, err := s.loadRuntimeFromDatabase(workCtx, generation, scene)
	if err != nil {
		return err
	}
	if err := s.confirmReadyGeneration(workCtx, generation); err != nil {
		return err
	}
	return s.putRuntimeSnapshot(workCtx, variant, snapshot)
}

func (s *runtimeStore) recoverReadinessState(ctx context.Context, scene, variant string) error {
	lease, err := s.tryRepair(ctx, variant)
	if err != nil {
		return err
	}
	if lease == nil {
		return s.wait(ctx, s.waitStep)
	}
	defer func() { _ = lease.Release(ctx) }()
	workCtx, cancel := lease.WorkContext(ctx)
	defer cancel()
	generation, err := s.generations.Current(workCtx, s.scope)
	if err != nil {
		return err
	}
	if _, err := s.states.Reconcile(workCtx, s.scope, generation); err != nil {
		return err
	}
	readiness, err := s.loadReadinessFromDatabase(workCtx, scene)
	if err != nil {
		return err
	}
	if err := s.confirmReadyGeneration(workCtx, generation); err != nil {
		return err
	}
	return s.putReadinessSnapshot(workCtx, variant, newVerifyCodeReadinessSnapshot(generation, readiness))
}

func (s *runtimeStore) tryFill(ctx context.Context, generation int64, variant string) (*cachefill.Lease, error) {
	target, err := cachegeneration.FillKey(s.scope, generation, variant)
	if err != nil {
		return nil, err
	}
	return cachefill.Try(ctx, s.redis.UniversalClient(), cachegeneration.StateKey(s.scope), target)
}

func (s *runtimeStore) tryRepair(ctx context.Context, variant string) (*cachefill.Lease, error) {
	return cachefill.Try(ctx, s.redis.UniversalClient(), cachegeneration.StateKey(s.scope), "repair:"+variant)
}

func (s *runtimeStore) readRuntimeSnapshot(ctx context.Context, generation int64, scene, variant string) (runtimeSnapshot, bool, error) {
	key, err := cachegeneration.SnapshotKey(s.scope, generation, variant)
	if err != nil {
		return runtimeSnapshot{}, false, err
	}
	raw, found, err := s.redis.GetString(ctx, key)
	if err != nil || !found {
		return runtimeSnapshot{}, found, err
	}
	snapshot, err := decodeRuntimeSnapshot(raw)
	if err != nil {
		return runtimeSnapshot{}, false, err
	}
	if snapshot.Generation != generation {
		return runtimeSnapshot{}, false, fmt.Errorf("%w: generation does not match", ErrRuntimeSnapshotCorrupt)
	}
	if err := validateRuntimeSnapshot(snapshot, scene); err != nil {
		return runtimeSnapshot{}, false, fmt.Errorf("%w: %v", ErrRuntimeSnapshotCorrupt, err)
	}
	return snapshot, true, nil
}

func (s *runtimeStore) putRuntimeSnapshot(ctx context.Context, variant string, snapshot runtimeSnapshot) error {
	payload, err := encodeRuntimeSnapshot(snapshot)
	if err != nil {
		return err
	}
	key, err := cachegeneration.SnapshotKey(s.scope, snapshot.Generation, variant)
	if err != nil {
		return err
	}
	return s.redis.SetString(ctx, key, payload, s.ttl)
}

func (s *runtimeStore) readReadinessSnapshot(ctx context.Context, generation int64, variant string) (VerifyCodeReadiness, bool, error) {
	key, err := cachegeneration.SnapshotKey(s.scope, generation, variant)
	if err != nil {
		return VerifyCodeReadiness{}, false, err
	}
	raw, found, err := s.redis.GetString(ctx, key)
	if err != nil || !found {
		return VerifyCodeReadiness{}, found, err
	}
	snapshot, err := decodeVerifyCodeReadinessSnapshot(raw)
	if err != nil {
		return VerifyCodeReadiness{}, false, err
	}
	if snapshot.Generation != generation {
		return VerifyCodeReadiness{}, false, corruptReadiness("generation does not match")
	}
	return readinessFromSnapshot(snapshot), true, nil
}

func (s *runtimeStore) putReadinessSnapshot(ctx context.Context, variant string, snapshot verifyCodeReadinessSnapshot) error {
	payload, err := encodeVerifyCodeReadinessSnapshot(snapshot)
	if err != nil {
		return err
	}
	key, err := cachegeneration.SnapshotKey(s.scope, snapshot.Generation, variant)
	if err != nil {
		return err
	}
	return s.redis.SetString(ctx, key, payload, s.ttl)
}

func (s *runtimeStore) loadRuntimeFromDatabase(ctx context.Context, generation int64, scene string) (runtimeSnapshot, error) {
	config, err := s.repository.FindConfig(ctx)
	if err != nil {
		return runtimeSnapshot{}, err
	}
	templates, err := s.repository.ListTemplates(ctx)
	if err != nil {
		return runtimeSnapshot{}, err
	}
	rules, err := s.repository.ListRecipientRules(ctx)
	if err != nil {
		return runtimeSnapshot{}, err
	}
	snapshot := runtimeSnapshot{
		SchemaVersion: mailCacheSchemaVersion, Generation: generation,
		Config: runtimeConfigFromModel(config), Templates: templates, Rules: rules,
	}
	if err := validateRuntimeSnapshot(snapshot, scene); err != nil {
		return runtimeSnapshot{}, err
	}
	return snapshot, nil
}

func (s *runtimeStore) loadReadinessFromDatabase(ctx context.Context, scene string) (VerifyCodeReadiness, error) {
	config, err := s.repository.FindConfig(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return VerifyCodeReadiness{}, nil
	}
	if err != nil {
		return VerifyCodeReadiness{}, err
	}
	if config.IsEnabled != yesno.Yes || config.SecretIDCiphertext == "" || config.SecretKeyCiphertext == "" {
		return VerifyCodeReadiness{}, nil
	}
	template, err := s.repository.FindTemplateByScene(ctx, scene)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return VerifyCodeReadiness{}, nil
	}
	if err != nil {
		return VerifyCodeReadiness{}, err
	}
	if template.IsEnabled != yesno.Yes {
		return VerifyCodeReadiness{}, nil
	}
	return VerifyCodeReadiness{Ready: true, TTLMinutes: int(config.TTLMinutes)}, nil
}

func (s *runtimeStore) confirmReadyGeneration(ctx context.Context, generation int64) error {
	state, found, err := s.states.Read(ctx, s.scope)
	if err != nil {
		return err
	}
	if !found || state.State != cachegeneration.StateReady || state.Generation != generation {
		return fmt.Errorf("%w: expected generation %d", ErrMailGenerationAdvanced, generation)
	}
	return nil
}

func (s *runtimeStore) Mutate(ctx context.Context, change func(context.Context, int64) (cachegeneration.MutationResult, error)) error {
	if err := s.configured(); err != nil || change == nil {
		if err == nil {
			err = fmt.Errorf("mail runtime mutation callback is missing")
		}
		return err
	}
	deadline := s.now().Add(s.writeBudget)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := s.mutateOnce(ctx, change)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, cachegeneration.ErrUpdating), errors.Is(err, cachegeneration.ErrGenerationChanged):
			if !s.now().Before(deadline) {
				return fmt.Errorf("mail mutation budget exhausted: %w", err)
			}
			if waitErr := s.wait(ctx, s.waitStep); waitErr != nil {
				return waitErr
			}
		default:
			return err
		}
	}
}

func (s *runtimeStore) mutateOnce(ctx context.Context, change func(context.Context, int64) (cachegeneration.MutationResult, error)) error {
	base, err := s.generations.Current(ctx, s.scope)
	if err != nil {
		return err
	}
	lease, err := s.acquireLease(ctx, base)
	if err != nil {
		return err
	}
	mutationCtx, stopRenewal := s.startRenewal(ctx, lease)
	result, applyErr := change(mutationCtx, base)
	stopRenewal()
	if applyErr != nil {
		return s.finishFailedMutation(ctx, lease, base, applyErr)
	}
	if !result.Changed {
		return lease.Rollback(ctx)
	}
	if err := lease.Commit(ctx, result.Generation); err != nil {
		s.logGenerationFailure("publish mail generation failed", result.Generation, result.OutboxID, err)
		return nil
	}
	if _, err := s.generations.MarkPublishedIfUnclaimed(ctx, result.OutboxID, s.now().UTC()); err != nil {
		s.logGenerationFailure("mark mail outbox published failed", result.Generation, result.OutboxID, err)
	}
	return nil
}

func (s *runtimeStore) acquireLease(ctx context.Context, base int64) (*cachegeneration.Lease, error) {
	lease, err := s.states.Acquire(ctx, s.scope, base)
	if err == nil {
		return lease, nil
	}
	if !errors.Is(err, cachegeneration.ErrStateMissing) && !errors.Is(err, cachegeneration.ErrStateCorrupt) {
		return nil, err
	}
	if _, reconcileErr := s.states.Reconcile(ctx, s.scope, base); reconcileErr != nil {
		return nil, errors.Join(err, reconcileErr)
	}
	return s.states.Acquire(ctx, s.scope, base)
}

func (s *runtimeStore) startRenewal(parent context.Context, lease *cachegeneration.Lease) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(s.renewInterval)
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

func (s *runtimeStore) finishFailedMutation(ctx context.Context, lease *cachegeneration.Lease, base int64, applyErr error) error {
	current, readErr := s.generations.Current(ctx, s.scope)
	if readErr != nil {
		return errors.Join(applyErr, readErr)
	}
	if current > base {
		if err := lease.Commit(ctx, current); err != nil {
			s.logGenerationFailure("publish mail generation after uncertain commit failed", current, 0, err)
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

func (s *runtimeStore) configured() error {
	if s == nil || s.repository == nil || s.redis == nil || s.generations == nil || s.states == nil {
		return fmt.Errorf("mail cache generation dependencies are not configured")
	}
	return s.scope.Validate()
}

func validateRuntimeSnapshot(snapshot runtimeSnapshot, scene string) error {
	if snapshot.SchemaVersion != mailCacheSchemaVersion || snapshot.Generation < 1 {
		return fmt.Errorf("mail runtime snapshot coordinates are invalid")
	}
	if err := snapshot.Config.valid(); err != nil {
		return err
	}
	if _, ok := mailtemplate.FindFixed(scene); !ok {
		return fmt.Errorf("mail runtime scene is invalid")
	}
	foundTemplate := false
	for _, template := range snapshot.Templates {
		if template.Scene == scene {
			foundTemplate = true
			break
		}
	}
	if !foundTemplate {
		return fmt.Errorf("mail runtime template for scene %q is missing", scene)
	}
	return nil
}

func encodeRuntimeSnapshot(snapshot runtimeSnapshot) (string, error) {
	if snapshot.SchemaVersion != mailCacheSchemaVersion || snapshot.Generation < 1 {
		return "", fmt.Errorf("%w: coordinates are invalid", ErrRuntimeSnapshotCorrupt)
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("encode mail runtime snapshot: %w", err)
	}
	return string(payload), nil
}

func decodeRuntimeSnapshot(raw string) (runtimeSnapshot, error) {
	if err := rejectDuplicateJSONKeys([]byte(raw)); err != nil {
		return runtimeSnapshot{}, fmt.Errorf("%w: %v", ErrRuntimeSnapshotCorrupt, err)
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	var snapshot runtimeSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return runtimeSnapshot{}, fmt.Errorf("%w: %v", ErrRuntimeSnapshotCorrupt, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return runtimeSnapshot{}, fmt.Errorf("%w: trailing data", ErrRuntimeSnapshotCorrupt)
	}
	if snapshot.SchemaVersion != mailCacheSchemaVersion || snapshot.Generation < 1 {
		return runtimeSnapshot{}, fmt.Errorf("%w: coordinates are invalid", ErrRuntimeSnapshotCorrupt)
	}
	return snapshot, nil
}

func waitForMailCache(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *runtimeStore) logGenerationFailure(message string, generation, outboxID int64, err error) {
	s.logger.Error(message,
		"namespace", s.scope.Namespace,
		"scopeKey", s.scope.ScopeKey,
		"generation", generation,
		"outboxId", outboxID,
		"errorClass", cachegeneration.ErrorClass(err),
	)
}
