package setting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/cacheFill"
	"admin/server/internal/shared/cacheGeneration"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

const (
	// settingReadBudget 是热读失败闭合的上限：followers 最多等待约 500ms。
	settingReadBudget  = 500 * time.Millisecond
	settingFillWait    = 20 * time.Millisecond
	settingWriteBudget = 500 * time.Millisecond
	settingWriteWait   = 20 * time.Millisecond
)

// ErrGenerationAdvanced 表示回源期间 generation 已变化：必须丢弃结果并重试完整读流程。
var ErrGenerationAdvanced = errors.New("setting cache generation advanced during fill")

type repository interface {
	List(context.Context, ListQuery) ([]Record, int64, error)
	Find(context.Context, string) (Record, error)
	Create(context.Context, *Record, int64) (cachegeneration.MutationResult, error)
	Update(context.Context, string, Record, int64) (cachegeneration.MutationResult, error)
	UpdateStatus(context.Context, string, yesno.Value, int64, time.Time) (cachegeneration.MutationResult, error)
	Delete(context.Context, string, int64, time.Time) (cachegeneration.MutationResult, error)
	FindBrand(context.Context) (BrandSettings, error)
	UpdateBrand(context.Context, BrandSettings, int64, time.Time) (cachegeneration.MutationResult, error)
}

type settingCache interface {
	ReadState(context.Context) (cachegeneration.State, bool, error)
	GetRecord(context.Context, int64, string) (Record, bool, error)
	PutRecord(context.Context, int64, Record) error
	GetBrand(context.Context, int64) (BrandSettings, bool, error)
	PutBrand(context.Context, int64, BrandSettings) error
	TryFill(context.Context, int64, string) (*cachefill.Lease, error)
	TryRepair(context.Context, string) (*cachefill.Lease, error)
}

type Service struct {
	repository    repository
	cache         settingCache
	generations   *cachegeneration.Repository
	states        *cachegeneration.Store
	logger        *slog.Logger
	scope         cachegeneration.Scope
	renewInterval time.Duration
	readBudget    time.Duration
	writeBudget   time.Duration
	waitStep      time.Duration
	now           func() time.Time
	wait          func(context.Context, time.Duration) error
}

func NewService(repository repository) *Service {
	return &Service{
		repository:    repository,
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		scope:         settingGenerationScope,
		renewInterval: cachegeneration.MutationRenewInterval,
		readBudget:    settingReadBudget,
		writeBudget:   settingWriteBudget,
		waitStep:      settingWriteWait,
		now:           time.Now,
		wait:          waitForSettingRetry,
	}
}

func (s *Service) SetCache(cache settingCache) { s.cache = cache }

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
	if s == nil || s.repository == nil || s.cache == nil || s.generations == nil || s.states == nil {
		return fmt.Errorf("setting cache generation dependencies are not configured")
	}
	if cache, ok := s.cache.(interface{ ValidateDependencies() error }); ok {
		if err := cache.ValidateDependencies(); err != nil {
			return err
		}
	}
	return s.scope.Validate()
}

// Brand 读取同一 generation 下的单 brand 快照：一次 PostgreSQL 查询装入三字段。
func (s *Service) Brand(ctx context.Context) (BrandSettings, error) {
	return readSettingCache(ctx, s,
		func(ctx context.Context, generation int64) (BrandSettings, bool, error) {
			return s.cache.GetBrand(ctx, generation)
		},
		s.recoverBrandState,
		s.fillBrand,
	)
}

func (s *Service) UpdateBrand(ctx context.Context, brand BrandSettings) error {
	brand.TitleZhCN = strings.TrimSpace(brand.TitleZhCN)
	brand.TitleEnUS = strings.TrimSpace(brand.TitleEnUS)
	brand.DefaultAvatar = strings.TrimSpace(brand.DefaultAvatar)
	if utf8.RuneCountInString(brand.TitleZhCN) == 0 || utf8.RuneCountInString(brand.TitleZhCN) > 128 || utf8.RuneCountInString(brand.TitleEnUS) == 0 || utf8.RuneCountInString(brand.TitleEnUS) > 128 {
		return apperror.InvalidRequest(fmt.Errorf("brand title is invalid"))
	}
	if brand.DefaultAvatar != "" && (!strings.HasPrefix(brand.DefaultAvatar, "avatar/") || strings.Contains(brand.DefaultAvatar, "..") || strings.ContainsAny(brand.DefaultAvatar, "\\\r\n\t")) {
		return apperror.InvalidRequest(fmt.Errorf("brand avatar is invalid"))
	}
	return s.mutate(ctx, func(mutationCtx context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.UpdateBrand(mutationCtx, brand, expected, s.now().UTC())
	})
}

// Find 读取运行时设置：ready generation 快照命中零 PostgreSQL；
// miss/损坏由 cacheFill 单 leader 有界回源；Redis 故障与等待超时 fail closed。
func (s *Service) Find(ctx context.Context, key string) (Record, error) {
	key = strings.TrimSpace(key)
	row, err := readSettingCache(ctx, s,
		func(ctx context.Context, generation int64) (Record, bool, error) {
			return s.cache.GetRecord(ctx, generation, key)
		},
		func(ctx context.Context) error { return s.recoverRecordState(ctx, key) },
		func(ctx context.Context, generation int64) error { return s.fillRecord(ctx, generation, key) },
	)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Record{}, ErrNotFound
		}
		return Record{}, err
	}
	return row, nil
}

func (s *Service) FindByKey(ctx context.Context, key string) (sharedsetting.Record, error) {
	row, err := s.Find(ctx, key)
	if err != nil {
		return sharedsetting.Record{}, err
	}
	return sharedsetting.Record{Key: row.Key, Value: row.Value, ValueType: row.ValueType, Description: row.Description, IsEnabled: row.IsEnabled, IsBuiltin: row.IsBuiltin}, nil
}

func (s *Service) List(ctx context.Context, query ListQuery) (ListResult, error) {
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 {
		return ListResult{}, apperror.InvalidRequest(fmt.Errorf("pagination is invalid"))
	}
	rows, total, err := s.repository.List(ctx, query)
	if err != nil {
		return ListResult{}, apperror.DependencyUnavailable(err)
	}
	return ListResult{Items: rows, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (int64, error) {
	input.Key, input.Value, input.Description = strings.TrimSpace(input.Key), strings.TrimSpace(input.Value), strings.TrimSpace(input.Description)
	if err := validateInput(input.Key, input.Value, input.ValueType, input.Description); err != nil {
		return 0, apperror.InvalidRequest(err)
	}
	if row, err := s.repository.Find(ctx, input.Key); err == nil && row.Key != "" {
		return 0, apperror.Conflict("error.conflict", nil, ErrConflict)
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return 0, apperror.DependencyUnavailable(err)
	}
	now := s.now().UTC()
	row := Record{Key: input.Key, Value: input.Value, ValueType: input.ValueType, Description: input.Description, IsEnabled: yesno.Yes, IsBuiltin: yesno.No, CreatedAt: now, UpdatedAt: now}
	if err := s.mutate(ctx, func(mutationCtx context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.Create(mutationCtx, &row, expected)
	}); err != nil {
		return 0, err
	}
	return row.ID, nil
}

func (s *Service) Update(ctx context.Context, key string, input UpdateInput) error {
	key = strings.TrimSpace(key)
	input.Value, input.Description = strings.TrimSpace(input.Value), strings.TrimSpace(input.Description)
	if err := validateInput(key, input.Value, input.ValueType, input.Description); err != nil {
		return apperror.InvalidRequest(err)
	}
	current, err := s.repository.Find(ctx, key)
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound(err)
	}
	if err != nil {
		return apperror.DependencyUnavailable(err)
	}
	current.Value, current.ValueType, current.Description, current.UpdatedAt = input.Value, input.ValueType, input.Description, s.now().UTC()
	return s.mutate(ctx, func(mutationCtx context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.Update(mutationCtx, key, current, expected)
	})
}

func (s *Service) UpdateStatus(ctx context.Context, key string, status yesno.Value) error {
	if !yesno.IsValid(status) {
		return apperror.InvalidRequest(fmt.Errorf("isEnabled must be 0 or 1"))
	}
	key = strings.TrimSpace(key)
	if isRequiredSetting(key) && status == yesno.No {
		return apperror.InvalidRequest(fmt.Errorf("required setting cannot be disabled"))
	}
	if _, err := s.repository.Find(ctx, key); errors.Is(err, ErrNotFound) {
		return apperror.NotFound(err)
	} else if err != nil {
		return apperror.DependencyUnavailable(err)
	}
	return s.mutate(ctx, func(mutationCtx context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.UpdateStatus(mutationCtx, key, status, expected, s.now().UTC())
	})
}

func (s *Service) Delete(ctx context.Context, key string) error {
	key = strings.TrimSpace(key)
	row, err := s.repository.Find(ctx, key)
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound(err)
	}
	if err != nil {
		return apperror.DependencyUnavailable(err)
	}
	if row.IsBuiltin == yesno.Yes {
		return apperror.Conflict("error.conflict", nil, fmt.Errorf("builtin setting cannot be deleted"))
	}
	return s.mutate(ctx, func(mutationCtx context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.Delete(mutationCtx, key, expected, s.now().UTC())
	})
}

// mutate 在 500ms 预算内重复完整 mutation 尝试；只重试内部代际竞争，
// Redis I/O、非法 state 和业务错误直接失败闭合。
func (s *Service) mutate(ctx context.Context, apply func(context.Context, int64) (cachegeneration.MutationResult, error)) error {
	if s.generations == nil || s.states == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("setting mutation requires cache generation dependencies"))
	}
	deadline := s.now().Add(s.writeBudget)
	for {
		if err := ctx.Err(); err != nil {
			return apperror.DependencyUnavailable(err)
		}
		err := s.mutateOnce(ctx, apply)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, cachegeneration.ErrUpdating), errors.Is(err, cachegeneration.ErrGenerationChanged):
			if !s.now().Before(deadline) {
				return apperror.DependencyUnavailable(fmt.Errorf("setting mutation budget exhausted: %w", err))
			}
			if waitErr := s.wait(ctx, s.waitStep); waitErr != nil {
				return apperror.DependencyUnavailable(waitErr)
			}
		default:
			return mapMutationError(err)
		}
	}
}

// mutateOnce 固定单次顺序：读取 current generation -> Acquire lease ->
// 启动带 cancel 的续租 -> 业务事务 -> 停续租 -> commit/rollback Redis state ->
// 同步标记 outbox。PostgreSQL 已提交后 Redis 发布失败不再是业务失败。
func (s *Service) mutateOnce(ctx context.Context, apply func(context.Context, int64) (cachegeneration.MutationResult, error)) error {
	scope := s.scope
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
		// no-op：不推进 generation，恢复原 ready。
		if err := lease.Rollback(ctx); err != nil {
			return err
		}
		return nil
	}
	if err := lease.Commit(ctx, result.Generation); err != nil {
		s.logGenerationFailure("publish setting generation failed", scope, result.Generation, result.OutboxID, err)
		return nil
	}
	if _, err := s.generations.MarkPublishedIfUnclaimed(ctx, result.OutboxID, s.now().UTC()); err != nil {
		s.logGenerationFailure("mark setting outbox published failed", scope, result.Generation, result.OutboxID, err)
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

// startRenewal 周期续租；续租失败立即取消 mutation context，使数据库事务回滚。
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

// finishFailedMutation 判定业务事务的真实结果：
// 无法确认时保留 invalidating，绝不恢复 stale ready。
func (s *Service) finishFailedMutation(ctx context.Context, scope cachegeneration.Scope, lease *cachegeneration.Lease, base int64, applyErr error) error {
	current, readErr := s.generations.Current(ctx, scope)
	if readErr != nil {
		return errors.Join(applyErr, readErr)
	}
	if current > base {
		if err := lease.Commit(ctx, current); err != nil {
			s.logGenerationFailure("publish setting generation after uncertain commit failed", scope, current, 0, err)
		}
		if errors.Is(applyErr, errMutationRolledBack) {
			return applyErr
		}
		return nil
	}
	if err := lease.Rollback(ctx); err != nil {
		return errors.Join(applyErr, err)
	}
	return applyErr
}

// readSettingCache 是共享热读协议：只从捕获到的 ready generation 读取不可变快照；
// 回源必须经过 cacheFill 单 leader 准入，且发布前重新确认同一 generation。
func readSettingCache[T any](
	ctx context.Context,
	service *Service,
	get func(context.Context, int64) (T, bool, error),
	recoverState func(context.Context) error,
	fill func(context.Context, int64) error,
) (T, error) {
	var zero T
	if service == nil || service.cache == nil || service.generations == nil || service.states == nil {
		return zero, apperror.DependencyUnavailable(fmt.Errorf("setting read requires cache generation dependencies"))
	}
	deadline := time.Now().Add(service.readBudget)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return zero, apperror.DependencyUnavailable(err)
		}
		state, found, err := service.cache.ReadState(ctx)
		if err != nil {
			if errors.Is(err, cachegeneration.ErrStateMissing) || errors.Is(err, cachegeneration.ErrStateCorrupt) {
				if repairErr := recoverState(ctx); repairErr != nil {
					if errors.Is(repairErr, ErrGenerationAdvanced) {
						continue
					}
					return zero, apperror.DependencyUnavailable(errors.Join(err, repairErr))
				}
				continue
			}
			// Redis 请求本身失败：不访问 PostgreSQL。
			return zero, apperror.DependencyUnavailable(err)
		}
		if !found {
			if repairErr := recoverState(ctx); repairErr != nil {
				if errors.Is(repairErr, ErrGenerationAdvanced) {
					continue
				}
				return zero, apperror.DependencyUnavailable(errors.Join(cachegeneration.ErrStateMissing, repairErr))
			}
			continue
		}
		if state.State == cachegeneration.StateInvalidating {
			// mutation 进行中：等待，不读 PostgreSQL；预算耗尽 fail closed。
			if time.Until(deadline) < settingFillWait {
				return zero, apperror.DependencyUnavailable(fmt.Errorf("setting cache state is invalidating"))
			}
			if waitErr := cachefill.Wait(ctx); waitErr != nil {
				return zero, apperror.DependencyUnavailable(waitErr)
			}
			continue
		}
		if state.State != cachegeneration.StateReady {
			return zero, apperror.DependencyUnavailable(fmt.Errorf("setting cache state is invalid"))
		}
		value, ok, err := get(ctx, state.Generation)
		if err != nil && !errors.Is(err, ErrSnapshotCorrupt) {
			return zero, apperror.DependencyUnavailable(err)
		}
		if err == nil && ok {
			return value, nil
		}
		if fillErr := fill(ctx, state.Generation); fillErr != nil {
			if errors.Is(fillErr, ErrGenerationAdvanced) {
				// 丢弃回源结果，重试完整读流程。
				continue
			}
			return zero, apperror.DependencyUnavailable(fillErr)
		}
	}
	return zero, apperror.DependencyUnavailable(fmt.Errorf("setting cache read budget exhausted"))
}

func (s *Service) fillRecord(ctx context.Context, generation int64, key string) error {
	lease, err := s.cache.TryFill(ctx, generation, recordVariant(key))
	if err != nil {
		return err
	}
	if lease == nil {
		// follower：有界等待，由外层重试完整读流程。
		return cachefill.Wait(ctx)
	}
	defer func() { _ = lease.Release(ctx) }()
	workCtx, cancel := lease.WorkContext(ctx)
	defer cancel()

	// 拿到 leader 准入后复查快照，避免前任 leader 已写回时重复回源。
	if _, ok, err := s.cache.GetRecord(workCtx, generation, key); err == nil && ok {
		return nil
	}
	row, err := s.repository.Find(workCtx, key)
	if err != nil {
		return err
	}
	if err := s.confirmReadyGeneration(workCtx, generation); err != nil {
		return err
	}
	return s.cache.PutRecord(workCtx, generation, row)
}

func (s *Service) fillBrand(ctx context.Context, generation int64) error {
	lease, err := s.cache.TryFill(ctx, generation, brandSnapshotVariant)
	if err != nil {
		return err
	}
	if lease == nil {
		return cachefill.Wait(ctx)
	}
	defer func() { _ = lease.Release(ctx) }()
	workCtx, cancel := lease.WorkContext(ctx)
	defer cancel()

	if _, ok, err := s.cache.GetBrand(workCtx, generation); err == nil && ok {
		return nil
	}
	brand, err := s.repository.FindBrand(workCtx)
	if err != nil {
		return err
	}
	if err := s.confirmReadyGeneration(workCtx, generation); err != nil {
		return err
	}
	return s.cache.PutBrand(workCtx, generation, brand)
}

// recoverRecordState 在 state missing/corrupt 时由单 leader 读取 PostgreSQL
// 权威代际，同时修复 state 与快照；followers 只等待。
func (s *Service) recoverRecordState(ctx context.Context, key string) error {
	lease, err := s.cache.TryRepair(ctx, recordVariant(key))
	if err != nil {
		return err
	}
	if lease == nil {
		return cachefill.Wait(ctx)
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
	row, err := s.repository.Find(workCtx, key)
	if err != nil {
		return err
	}
	if err := s.confirmReadyGeneration(workCtx, generation); err != nil {
		return err
	}
	return s.cache.PutRecord(workCtx, generation, row)
}

func (s *Service) recoverBrandState(ctx context.Context) error {
	lease, err := s.cache.TryRepair(ctx, brandSnapshotVariant)
	if err != nil {
		return err
	}
	if lease == nil {
		return cachefill.Wait(ctx)
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
	brand, err := s.repository.FindBrand(workCtx)
	if err != nil {
		return err
	}
	if err := s.confirmReadyGeneration(workCtx, generation); err != nil {
		return err
	}
	return s.cache.PutBrand(workCtx, generation, brand)
}

// confirmReadyGeneration 确保回源结果只写入它开始时捕获的代际。
func (s *Service) confirmReadyGeneration(ctx context.Context, generation int64) error {
	state, found, err := s.cache.ReadState(ctx)
	if err != nil {
		return err
	}
	if !found || state.State != cachegeneration.StateReady || state.Generation != generation {
		return fmt.Errorf("%w: expected generation %d", ErrGenerationAdvanced, generation)
	}
	return nil
}

func mapMutationError(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return apperror.NotFound(err)
	case errors.Is(err, ErrConflict):
		return apperror.Conflict("error.conflict", nil, err)
	default:
		return apperror.DependencyUnavailable(err)
	}
}

func waitForSettingRetry(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
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

var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)

func validateInput(key, value string, valueType int, description string) error {
	if !keyPattern.MatchString(key) || utf8.RuneCountInString(key) > 128 || utf8.RuneCountInString(description) > 512 || !validSettingValue(value, valueType) {
		return fmt.Errorf("setting input is invalid")
	}
	switch key {
	case sharedsetting.MessageNotificationRetentionDaysKey:
		if valueType != ValueTypeNumber || !integerInRange(value, 30, 3650) {
			return fmt.Errorf("notification retention days must be an integer from 30 to 3650")
		}
	case sharedsetting.RealtimeEventRetentionDaysKey:
		if valueType != ValueTypeNumber || !integerInRange(value, 1, 30) {
			return fmt.Errorf("realtime event retention days must be an integer from 1 to 30")
		}
	case sharedsetting.SchedulerHistoryRetentionDaysKey:
		if valueType != ValueTypeNumber || !integerInRange(value, 7, 3650) {
			return fmt.Errorf("scheduler history retention days must be an integer from 7 to 3650")
		}
	}
	return nil
}

func isRequiredSetting(key string) bool {
	return key == sharedsetting.MessageNotificationRetentionDaysKey || key == sharedsetting.RealtimeEventRetentionDaysKey || key == sharedsetting.SchedulerHistoryRetentionDaysKey
}

func integerInRange(value string, minimum, maximum int) bool {
	parsed, err := strconv.Atoi(value)
	return err == nil && parsed >= minimum && parsed <= maximum
}

func validSettingValue(value string, valueType int) bool {
	switch valueType {
	case ValueTypeString:
		return utf8.RuneCountInString(value) <= 4096
	case ValueTypeNumber:
		_, err := strconv.Atoi(value)
		return err == nil
	case ValueTypeBool:
		return value == "0" || value == "1" || value == "true" || value == "false"
	case ValueTypeJSON:
		return json.Valid([]byte(value))
	default:
		return false
	}
}
