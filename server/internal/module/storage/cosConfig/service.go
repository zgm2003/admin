package cosconfig

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"admin/server/internal/secretkey"
	cachegeneration "admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	"admin/server/internal/storage/cos"
	"gorm.io/gorm"
)

const (
	// cosConfigReadBudget 是热读失败闭合上限：followers 最多等待约 500ms。
	cosConfigReadBudget = 500 * time.Millisecond
	// cosConfigWriteBudget 是内部 mutation 竞争的重试预算；每次等待 20ms，预算耗尽返回依赖繁忙。
	cosConfigWriteBudget = 500 * time.Millisecond
	cosConfigWaitStep    = 20 * time.Millisecond
)

// repository 是 Service 需要的最小持久化契约；真实的 *Repository 直接满足它，测试可注入可控事实。
type repository interface {
	Count(context.Context, ListQuery) (int64, error)
	List(context.Context, ListQuery) ([]Current, error)
	FindByID(context.Context, int64) (Current, error)
	Create(context.Context, *Model, Version) (cachegeneration.MutationResult, error)
	Update(context.Context, int64, UpdateValues, int64, time.Time) (cachegeneration.MutationResult, error)
	UpdateStatus(context.Context, int64, yesno.Value, int64, time.Time) (cachegeneration.MutationResult, error)
	MarkDeleted(context.Context, int64, int64, time.Time) (cachegeneration.MutationResult, error)
	CountRuleReferences(context.Context, int64) (int64, error)
	RuntimeFacts(context.Context, int64) (RuntimeConfig, error)
}

type Service struct {
	repository  repository
	generations *cachegeneration.Repository
	states      *cachegeneration.Store
	cache       *Cache
	keys        *secretkey.KeyRing
	tester      cos.ConnectionTester
	logger      *slog.Logger

	renewInterval time.Duration
	readBudget    time.Duration
	writeBudget   time.Duration
	waitStep      time.Duration

	now  func() time.Time
	wait func(context.Context, time.Duration) error
}

func NewService(repository repository, keys *secretkey.KeyRing, tester cos.ConnectionTester) *Service {
	return &Service{
		repository:    repository,
		keys:          keys,
		tester:        tester,
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		renewInterval: cachegeneration.MutationRenewInterval,
		readBudget:    cosConfigReadBudget,
		writeBudget:   cosConfigWriteBudget,
		waitStep:      cosConfigWaitStep,
		now:           time.Now,
		wait:          defaultWait,
	}
}

// SetCache 由显式装配注入 Redis 快照与 state store。
func (s *Service) SetCache(cache *Cache) { s.cache = cache }

// SetGenerations 由显式装配注入 generation 事实来源与 Redis state store。
func (s *Service) SetGenerations(generations *cachegeneration.Repository, states *cachegeneration.Store) {
	s.generations = generations
	s.states = states
}

func (s *Service) SetLogger(logger *slog.Logger) {
	if logger != nil {
		s.logger = logger
	}
}

func (s *Service) ValidateDependencies() error {
	if s == nil || s.repository == nil || s.generations == nil || s.states == nil || s.cache == nil ||
		s.cache.client == nil || s.cache.store == nil || s.keys == nil || s.tester == nil {
		return fmt.Errorf("COS config cache generation dependencies are not configured")
	}
	if validator, ok := s.repository.(interface{ ValidateDependencies() error }); ok {
		return validator.ValidateDependencies()
	}
	return nil
}

// Runtime 读取 (storage.cosconfig/<id>, runtime) 快照：
// ready 命中零 PostgreSQL；miss/损坏由 cacheFill 单 leader 有界回源；Redis 故障与预算耗尽 fail closed。
func (s *Service) Runtime(ctx context.Context, id int64) (RuntimeConfig, error) {
	if id < 1 {
		return RuntimeConfig{}, invalid(fmt.Errorf("id is invalid"))
	}
	return readRuntimeCache(ctx, s, id)
}

func (s *Service) List(ctx context.Context, query ListQuery) (pagination.Result[SafeValue], error) {
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 {
		return pagination.Result[SafeValue]{}, invalid(fmt.Errorf("invalid pagination"))
	}
	query.Keyword = strings.TrimSpace(query.Keyword)
	total, err := s.repository.Count(ctx, query)
	if err != nil {
		return pagination.Result[SafeValue]{}, dependency(err)
	}
	rows, err := s.repository.List(ctx, query)
	if err != nil {
		return pagination.Result[SafeValue]{}, dependency(err)
	}
	values := make([]SafeValue, 0, len(rows))
	for _, row := range rows {
		values = append(values, safeValue(row))
	}
	return pagination.Result[SafeValue]{List: values, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *Service) Get(ctx context.Context, id int64) (SafeValue, error) {
	if id < 1 {
		return SafeValue{}, invalid(fmt.Errorf("id is invalid"))
	}
	row, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return SafeValue{}, mapReadError(err)
	}
	return safeValue(row), nil
}

// Create：同一事务写入逻辑配置 + version 1 + generation 1/outbox 1，随后同步发布 ready 1；
// 发布失败只记录安全日志，由 missing-state repair 与 outbox relay 恢复。
func (s *Service) Create(ctx context.Context, input CreateInput) (int64, error) {
	if err := validateCreate(input); err != nil {
		return 0, invalid(err)
	}
	key, err := s.storageEncryptionKey()
	if err != nil {
		return 0, err
	}
	secretIDCiphertext, err := encryptCredential(key, strings.TrimSpace(input.SecretID))
	if err != nil {
		return 0, dependency(err)
	}
	secretKeyCiphertext, err := encryptCredential(key, strings.TrimSpace(input.SecretKey))
	if err != nil {
		return 0, dependency(err)
	}
	now := s.now().UTC()
	model := &Model{
		Name:                strings.TrimSpace(input.Name),
		AppID:               strings.TrimSpace(input.AppID),
		SecretIDCiphertext:  secretIDCiphertext,
		SecretKeyCiphertext: secretKeyCiphertext,
		IsEnabled:           input.IsEnabled,
		Remark:              strings.TrimSpace(input.Remark),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	version := Version{
		Bucket:       strings.TrimSpace(input.Bucket),
		Region:       strings.TrimSpace(input.Region),
		Endpoint:     normalizedPointer(input.Endpoint),
		BucketDomain: normalizedPointer(input.BucketDomain),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	result, err := s.repository.Create(ctx, model, version)
	if err != nil {
		if errors.Is(err, ErrNameConflict) {
			return 0, conflict(err)
		}
		return 0, dependency(err)
	}
	if s.states != nil {
		scope, scopeErr := cachegeneration.NewScope(cosConfigGenerationNamespace, fmt.Sprintf("%d", model.ID))
		if scopeErr != nil {
			s.logGenerationFailure("publish new COS config generation failed", cachegeneration.Scope{}, 0, 0, scopeErr)
			return model.ID, nil
		}
		if _, publishErr := s.states.Reconcile(ctx, scope, result.Generation); publishErr != nil {
			s.logGenerationFailure("publish new COS config generation failed", scope, result.Generation, result.OutboxID, publishErr)
		} else if s.generations != nil {
			if _, markErr := s.generations.MarkPublishedIfUnclaimed(ctx, result.OutboxID, s.now().UTC()); markErr != nil {
				s.logGenerationFailure("mark new COS config outbox published failed", scope, result.Generation, result.OutboxID, markErr)
			}
		}
	}
	return model.ID, nil
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) error {
	if id < 1 {
		return invalid(fmt.Errorf("id is invalid"))
	}
	if err := validateUpdate(input); err != nil {
		return invalid(err)
	}
	current, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return mapReadError(err)
	}
	values := UpdateValues{
		Name:         strings.TrimSpace(input.Name),
		Bucket:       strings.TrimSpace(input.Bucket),
		Region:       strings.TrimSpace(input.Region),
		Endpoint:     normalizedPointer(input.Endpoint),
		BucketDomain: normalizedPointer(input.BucketDomain),
		Remark:       strings.TrimSpace(input.Remark),
	}
	// Secret 轮换先按明文比较：只有真实变化才重新加密，避免相同值重复提交产生假变化。
	if input.SecretID.Present || input.SecretKey.Present {
		key, err := s.storageEncryptionKey()
		if err != nil {
			return err
		}
		if input.SecretID.Present {
			existing, err := decryptCredential(key, current.SecretIDCiphertext)
			if err != nil {
				return dependency(err)
			}
			if strings.TrimSpace(input.SecretID.Value) != existing {
				ciphertext, err := encryptCredential(key, strings.TrimSpace(input.SecretID.Value))
				if err != nil {
					return dependency(err)
				}
				values.SecretIDCiphertext = &ciphertext
			}
		}
		if input.SecretKey.Present {
			existing, err := decryptCredential(key, current.SecretKeyCiphertext)
			if err != nil {
				return dependency(err)
			}
			if strings.TrimSpace(input.SecretKey.Value) != existing {
				ciphertext, err := encryptCredential(key, strings.TrimSpace(input.SecretKey.Value))
				if err != nil {
					return dependency(err)
				}
				values.SecretKeyCiphertext = &ciphertext
			}
		}
	}
	return s.mutate(ctx, id, func(mutationCtx context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.Update(mutationCtx, id, values, expected, s.now().UTC())
	})
}

func (s *Service) UpdateStatus(ctx context.Context, id int64, value yesno.Value) error {
	if id < 1 {
		return invalid(fmt.Errorf("id is invalid"))
	}
	if !yesno.IsValid(value) {
		return invalid(fmt.Errorf("isEnabled is invalid"))
	}
	if _, err := s.repository.FindByID(ctx, id); err != nil {
		return mapReadError(err)
	}
	return s.mutate(ctx, id, func(mutationCtx context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.UpdateStatus(mutationCtx, id, value, expected, s.now().UTC())
	})
}

func (s *Service) TestConnection(ctx context.Context, id int64) error {
	if id < 1 {
		return invalid(fmt.Errorf("id is invalid"))
	}
	row, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return mapReadError(err)
	}
	if row.IsEnabled != yesno.Yes {
		return conflict(fmt.Errorf("COS config is disabled"))
	}
	if s.tester == nil {
		return dependency(fmt.Errorf("COS connection tester unavailable"))
	}
	key, err := s.storageEncryptionKey()
	if err != nil {
		return err
	}
	secretID, err := decryptCredential(key, row.SecretIDCiphertext)
	if err != nil {
		return dependency(err)
	}
	secretKey, err := decryptCredential(key, row.SecretKeyCiphertext)
	if err != nil {
		return dependency(err)
	}
	endpoint := ""
	if row.Endpoint != nil {
		endpoint = *row.Endpoint
	}
	if err := s.tester.TestConnection(ctx, cos.Credentials{AppID: row.AppID, SecretID: secretID, SecretKey: secretKey, Bucket: row.Bucket, Region: row.Region, Endpoint: endpoint}); err != nil {
		return dependency(err)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if id < 1 {
		return invalid(fmt.Errorf("id is invalid"))
	}
	if _, err := s.repository.FindByID(ctx, id); err != nil {
		return mapReadError(err)
	}
	references, err := s.repository.CountRuleReferences(ctx, id)
	if err != nil {
		return dependency(err)
	}
	if references > 0 {
		return conflict(fmt.Errorf("COS config is referenced by upload rules"))
	}
	return s.mutate(ctx, id, func(mutationCtx context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.MarkDeleted(mutationCtx, id, expected, s.now().UTC())
	})
}

// mutate：在 500ms 预算内重复完整 mutation 尝试；只对内部并发状态重试，Redis I/O 错误直接失败闭合。
func (s *Service) mutate(ctx context.Context, id int64, apply func(context.Context, int64) (cachegeneration.MutationResult, error)) error {
	if s.generations == nil || s.states == nil {
		return dependency(fmt.Errorf("COS config mutation requires cache generation dependencies"))
	}
	deadline := s.now().Add(s.writeBudget)
	for {
		if err := ctx.Err(); err != nil {
			return dependency(err)
		}
		err := s.mutateOnce(ctx, id, apply)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, cachegeneration.ErrUpdating), errors.Is(err, cachegeneration.ErrGenerationChanged):
			if !s.now().Before(deadline) {
				return dependency(fmt.Errorf("COS config mutation budget exhausted: %w", err))
			}
			if waitErr := s.wait(ctx, s.waitStep); waitErr != nil {
				return dependency(waitErr)
			}
		default:
			return s.mapMutationError(err)
		}
	}
}

// mutateOnce 固定顺序：读取 current generation -> Acquire 精确代际 lease -> 续租 ->
// 业务事务（含 AdvanceTx/outbox）-> 停续租 -> no-op rollback / changed commit -> 同步标记 outbox。
func (s *Service) mutateOnce(ctx context.Context, id int64, apply func(context.Context, int64) (cachegeneration.MutationResult, error)) error {
	scope, err := cachegeneration.NewScope(cosConfigGenerationNamespace, fmt.Sprintf("%d", id))
	if err != nil {
		return err
	}
	base, err := s.generations.Current(ctx, scope)
	if err != nil {
		return err
	}
	lease, err := s.acquireLease(ctx, scope, base)
	if err != nil {
		return err
	}
	mutationCtx, stopRenewal := s.startRenewal(ctx, lease)
	result, applyErr := apply(mutationCtx, base)
	stopRenewal()

	if applyErr != nil {
		return s.finishFailedMutation(ctx, scope, lease, base, applyErr)
	}
	if !result.Changed {
		if err := lease.Rollback(ctx); err != nil {
			return err
		}
		return nil
	}
	if err := lease.Commit(ctx, result.Generation); err != nil {
		s.logGenerationFailure("publish COS config generation failed", scope, result.Generation, result.OutboxID, err)
		return nil
	}
	if _, err := s.generations.MarkPublishedIfUnclaimed(ctx, result.OutboxID, s.now().UTC()); err != nil {
		s.logGenerationFailure("mark COS config outbox published failed", scope, result.Generation, result.OutboxID, err)
	}
	return nil
}

// acquireLease 在 state missing/corrupt 时先按 PostgreSQL 权威代际 Reconcile 一次再重试。
func (s *Service) acquireLease(ctx context.Context, scope cachegeneration.Scope, base int64) (*cachegeneration.Lease, error) {
	lease, err := s.states.Acquire(ctx, scope, base)
	if err == nil {
		return lease, nil
	}
	if !errors.Is(err, cachegeneration.ErrStateMissing) && !errors.Is(err, cachegeneration.ErrStateCorrupt) {
		return nil, err
	}
	if _, reconcileErr := s.states.Reconcile(ctx, scope, base); reconcileErr != nil {
		return nil, errors.Join(err, reconcileErr)
	}
	return s.states.Acquire(ctx, scope, base)
}

func (s *Service) startRenewal(parent context.Context, lease *cachegeneration.Lease) (context.Context, func()) {
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

// finishFailedMutation 判定业务事务的真实结果：无法确认时保留 invalidating，绝不恢复 stale ready。
func (s *Service) finishFailedMutation(ctx context.Context, scope cachegeneration.Scope, lease *cachegeneration.Lease, base int64, applyErr error) error {
	current, readErr := s.generations.Current(ctx, scope)
	if readErr != nil {
		return errors.Join(applyErr, readErr)
	}
	if current > base {
		if err := lease.Commit(ctx, current); err != nil {
			s.logGenerationFailure("publish COS config generation after uncertain commit failed", scope, current, 0, err)
		}
		return applyErr
	}
	if err := lease.Rollback(ctx); err != nil {
		return errors.Join(applyErr, err)
	}
	return applyErr
}

// readRuntimeCache 是热读协议：只读取捕获到的 ready generation 的不可变快照。
func readRuntimeCache(ctx context.Context, service *Service, id int64) (RuntimeConfig, error) {
	if service == nil || service.cache == nil || service.generations == nil || service.states == nil {
		return RuntimeConfig{}, dependency(fmt.Errorf("COS config read requires cache generation dependencies"))
	}
	deadline := service.now().Add(service.readBudget)
	for service.now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return RuntimeConfig{}, dependency(err)
		}
		state, found, err := service.cache.ReadState(ctx, id)
		if err != nil {
			if errors.Is(err, cachegeneration.ErrStateMissing) || errors.Is(err, cachegeneration.ErrStateCorrupt) {
				if repairErr := service.recoverRuntimeState(ctx, id); repairErr != nil {
					if errors.Is(repairErr, ErrGenerationAdvanced) {
						continue
					}
					return RuntimeConfig{}, dependency(errors.Join(err, repairErr))
				}
				continue
			}
			// Redis 请求本身失败：不访问 PostgreSQL。
			return RuntimeConfig{}, dependency(err)
		}
		if !found {
			if repairErr := service.recoverRuntimeState(ctx, id); repairErr != nil {
				if errors.Is(repairErr, ErrGenerationAdvanced) {
					continue
				}
				return RuntimeConfig{}, dependency(errors.Join(cachegeneration.ErrStateMissing, repairErr))
			}
			continue
		}
		if state.State == cachegeneration.StateInvalidating {
			if service.now().Add(service.waitStep).After(deadline) {
				return RuntimeConfig{}, dependency(fmt.Errorf("COS config cache state is invalidating"))
			}
			if waitErr := service.wait(ctx, service.waitStep); waitErr != nil {
				return RuntimeConfig{}, dependency(waitErr)
			}
			continue
		}
		if state.State != cachegeneration.StateReady {
			return RuntimeConfig{}, dependency(fmt.Errorf("COS config cache state is invalid"))
		}
		config, ok, err := service.cache.GetRuntime(ctx, id, state.Generation)
		if err != nil && !errors.Is(err, ErrSnapshotCorrupt) {
			return RuntimeConfig{}, dependency(err)
		}
		if err == nil && ok {
			return config, nil
		}
		if fillErr := service.fillRuntime(ctx, id, state.Generation); fillErr != nil {
			if errors.Is(fillErr, ErrGenerationAdvanced) {
				continue
			}
			return RuntimeConfig{}, dependency(fillErr)
		}
	}
	return RuntimeConfig{}, dependency(fmt.Errorf("COS config cache read budget exhausted"))
}

func (s *Service) fillRuntime(ctx context.Context, id, generation int64) error {
	lease, err := s.cache.TryFill(ctx, id, generation)
	if err != nil {
		return err
	}
	if lease == nil {
		return s.wait(ctx, s.waitStep)
	}
	defer func() { _ = lease.Release(ctx) }()
	workCtx, cancel := lease.WorkContext(ctx)
	defer cancel()

	if _, ok, err := s.cache.GetRuntime(workCtx, id, generation); err == nil && ok {
		return nil
	}
	config, err := s.repository.RuntimeFacts(workCtx, id)
	if err != nil {
		return err
	}
	if err := s.confirmReadyGeneration(workCtx, id, generation); err != nil {
		return err
	}
	return s.cache.PutRuntime(workCtx, id, generation, config)
}

// recoverRuntimeState 在 state missing/corrupt 时由单 leader 读取 PostgreSQL 权威代际并修复 state 与快照。
func (s *Service) recoverRuntimeState(ctx context.Context, id int64) error {
	lease, err := s.cache.TryRepair(ctx, id)
	if err != nil {
		return err
	}
	if lease == nil {
		return s.wait(ctx, s.waitStep)
	}
	defer func() { _ = lease.Release(ctx) }()
	workCtx, cancel := lease.WorkContext(ctx)
	defer cancel()

	scope, err := runtimeScope(id)
	if err != nil {
		return err
	}
	generation, err := s.generations.Current(workCtx, scope)
	if err != nil {
		return err
	}
	if _, err := s.states.Reconcile(workCtx, scope, generation); err != nil {
		return err
	}
	config, err := s.repository.RuntimeFacts(workCtx, id)
	if err != nil {
		return err
	}
	if err := s.confirmReadyGeneration(workCtx, id, generation); err != nil {
		return err
	}
	return s.cache.PutRuntime(workCtx, id, generation, config)
}

func (s *Service) confirmReadyGeneration(ctx context.Context, id, generation int64) error {
	state, found, err := s.cache.ReadState(ctx, id)
	if err != nil {
		return err
	}
	if !found || state.State != cachegeneration.StateReady || state.Generation != generation {
		return fmt.Errorf("%w: expected generation %d", ErrGenerationAdvanced, generation)
	}
	return nil
}

// mapMutationError 不把内部 generation 竞争或依赖故障伪装成 409；业务冲突仍保持原语义。
func (s *Service) mapMutationError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return notFound(err)
	case errors.Is(err, ErrNameConflict):
		return conflict(err)
	default:
		return dependency(err)
	}
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

func (s *Service) storageEncryptionKey() ([]byte, error) {
	if s.keys == nil {
		return nil, dependency(fmt.Errorf("storage encryption key unavailable"))
	}
	return s.keys.StorageEncryptionKey(), nil
}

func (s *Service) logGenerationFailure(message string, scope cachegeneration.Scope, generation, outboxID int64, err error) {
	s.logger.Error(message,
		"namespace", scope.Namespace,
		"scopeKey", scope.ScopeKey,
		"generation", generation,
		"outboxId", outboxID,
		"errorClass", cachegeneration.ErrorClass(err),
	)
}

func mapReadError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return notFound(err)
	}
	return dependency(err)
}

func normalizedPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	trimmed = strings.TrimSuffix(trimmed, "/")
	return &trimmed
}
