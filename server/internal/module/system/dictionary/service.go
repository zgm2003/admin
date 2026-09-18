package dictionary

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/yesno"

	"gorm.io/gorm"
)

const (
	dictionaryReadBudget  = 500 * time.Millisecond
	dictionaryWriteBudget = 500 * time.Millisecond
	dictionaryWaitStep    = 20 * time.Millisecond
)

var (
	codePattern                     = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$`)
	ErrDictionaryGenerationAdvanced = errors.New("dictionary cache generation advanced during fill")
)

type repository interface {
	List(context.Context, ListQuery) ([]ListItem, int64, error)
	Find(context.Context, int64) (Dictionary, error)
	Items(context.Context, int64, bool) ([]Item, error)
	Options(context.Context, []string) ([]optionFact, error)
	Create(context.Context, *Dictionary, int64, time.Time) (cachegeneration.MutationResult, error)
	Update(context.Context, int64, UpdateInput, int64, time.Time) (cachegeneration.MutationResult, error)
	UpdateStatus(context.Context, int64, yesno.Value, int64, time.Time) (cachegeneration.MutationResult, error)
	Delete(context.Context, int64, int64, time.Time) (cachegeneration.MutationResult, error)
	CreateItem(context.Context, *Item, int64, time.Time) (cachegeneration.MutationResult, error)
	UpdateItem(context.Context, int64, int64, UpdateItemInput, int64, time.Time) (cachegeneration.MutationResult, error)
	UpdateItemStatus(context.Context, int64, int64, yesno.Value, int64, time.Time) (cachegeneration.MutationResult, error)
	DeleteItem(context.Context, int64, int64, int64, time.Time) (cachegeneration.MutationResult, error)
}

type Service struct {
	repository  repository
	cache       *optionsCache
	generations *cachegeneration.Repository
	states      *cachegeneration.Store
	logger      *slog.Logger
	scope       cachegeneration.Scope

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
		scope:         dictionaryGenerationScope,
		renewInterval: cachegeneration.MutationRenewInterval,
		readBudget:    dictionaryReadBudget,
		writeBudget:   dictionaryWriteBudget,
		waitStep:      dictionaryWaitStep,
		now:           time.Now,
		wait:          waitForDictionary,
	}
}

func (s *Service) SetCache(cache *optionsCache) { s.cache = cache }

func (s *Service) SetGenerations(repository *cachegeneration.Repository, store *cachegeneration.Store) {
	s.generations = repository
	s.states = store
}

func (s *Service) SetLogger(logger *slog.Logger) {
	if logger != nil {
		s.logger = logger
	}
}

func (s *Service) ValidateDependencies() error {
	if s == nil || s.repository == nil || s.cache == nil || s.cache.redis == nil || s.cache.store == nil ||
		s.generations == nil || s.states == nil {
		return fmt.Errorf("dictionary cache generation dependencies are not configured")
	}
	return s.scope.Validate()
}

func (s *Service) List(ctx context.Context, query ListQuery) (ListResult, error) {
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 {
		return ListResult{}, apperror.InvalidRequest(fmt.Errorf("pagination is invalid"))
	}
	items, total, err := s.repository.List(ctx, query)
	if err != nil {
		return ListResult{}, apperror.DependencyUnavailable(err)
	}
	return ListResult{List: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *Service) Get(ctx context.Context, id int64) (Detail, error) {
	value, err := s.repository.Find(ctx, id)
	if err != nil {
		return Detail{}, mapDictionaryError(err)
	}
	items, err := s.repository.Items(ctx, id, false)
	if err != nil {
		return Detail{}, apperror.DependencyUnavailable(err)
	}
	return Detail{Dictionary: value, Items: items}, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (int64, error) {
	input.Code = strings.TrimSpace(input.Code)
	input.NameZH = strings.TrimSpace(input.NameZH)
	input.NameEN = strings.TrimSpace(input.NameEN)
	input.Description = strings.TrimSpace(input.Description)
	if !codePattern.MatchString(input.Code) || utf8.RuneCountInString(input.Code) > 128 || !validText(input.NameZH, 128) || !validText(input.NameEN, 128) || utf8.RuneCountInString(input.Description) > 512 {
		return 0, apperror.InvalidRequest(fmt.Errorf("dictionary input is invalid"))
	}
	now := s.now().UTC()
	value := Dictionary{Code: input.Code, NameZH: input.NameZH, NameEN: input.NameEN, Description: input.Description, IsEnabled: yesno.Yes, IsBuiltin: yesno.No, CreatedAt: now, UpdatedAt: now}
	if err := s.mutate(ctx, func(mutationCtx context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.Create(mutationCtx, &value, expected, now)
	}); err != nil {
		return 0, err
	}
	return value.ID, nil
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) error {
	if id < 1 {
		return apperror.InvalidRequest(fmt.Errorf("dictionary id is invalid"))
	}
	input.NameZH = strings.TrimSpace(input.NameZH)
	input.NameEN = strings.TrimSpace(input.NameEN)
	input.Description = strings.TrimSpace(input.Description)
	if !validText(input.NameZH, 128) || !validText(input.NameEN, 128) || utf8.RuneCountInString(input.Description) > 512 {
		return apperror.InvalidRequest(fmt.Errorf("dictionary names are required"))
	}
	now := s.now().UTC()
	return s.mutate(ctx, func(mutationCtx context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.Update(mutationCtx, id, input, expected, now)
	})
}

func (s *Service) UpdateStatus(ctx context.Context, id int64, status yesno.Value) error {
	if id < 1 || !yesno.IsValid(status) {
		return apperror.InvalidRequest(fmt.Errorf("dictionary status input is invalid"))
	}
	now := s.now().UTC()
	return s.mutate(ctx, func(mutationCtx context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.UpdateStatus(mutationCtx, id, status, expected, now)
	})
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if id < 1 {
		return apperror.InvalidRequest(fmt.Errorf("dictionary id is invalid"))
	}
	now := s.now().UTC()
	return s.mutate(ctx, func(mutationCtx context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.Delete(mutationCtx, id, expected, now)
	})
}

func (s *Service) Options(ctx context.Context, codes []string, language string) (OptionResult, error) {
	normalized, err := normalizeRequestedCodes(codes)
	if err != nil {
		return nil, apperror.InvalidRequest(err)
	}
	if strings.TrimSpace(language) == "" {
		return nil, apperror.InvalidRequest(fmt.Errorf("language is invalid"))
	}
	result, err := s.readOptions(ctx, normalized, language)
	if err != nil {
		return nil, mapDictionaryError(err)
	}
	return result, nil
}

func (s *Service) CreateItem(ctx context.Context, dictionaryID int64, input CreateItemInput) (int64, error) {
	if dictionaryID < 1 {
		return 0, apperror.InvalidRequest(fmt.Errorf("dictionary id is invalid"))
	}
	input.Value = strings.TrimSpace(input.Value)
	input.LabelZH = strings.TrimSpace(input.LabelZH)
	input.LabelEN = strings.TrimSpace(input.LabelEN)
	if !validText(input.Value, 128) || !validText(input.LabelZH, 256) || !validText(input.LabelEN, 256) || input.Sort < 0 {
		return 0, apperror.InvalidRequest(fmt.Errorf("dictionary item input is invalid"))
	}
	now := s.now().UTC()
	value := Item{DictionaryID: dictionaryID, Value: input.Value, LabelZH: input.LabelZH, LabelEN: input.LabelEN, Sort: input.Sort, IsEnabled: yesno.Yes, IsBuiltin: yesno.No, CreatedAt: now, UpdatedAt: now}
	if err := s.mutate(ctx, func(mutationCtx context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.CreateItem(mutationCtx, &value, expected, now)
	}); err != nil {
		return 0, err
	}
	return value.ID, nil
}

func (s *Service) UpdateItem(ctx context.Context, dictionaryID, itemID int64, input UpdateItemInput) error {
	if dictionaryID < 1 || itemID < 1 {
		return apperror.InvalidRequest(fmt.Errorf("dictionary item coordinates are invalid"))
	}
	input.LabelZH = strings.TrimSpace(input.LabelZH)
	input.LabelEN = strings.TrimSpace(input.LabelEN)
	if !validText(input.LabelZH, 256) || !validText(input.LabelEN, 256) || input.Sort < 0 {
		return apperror.InvalidRequest(fmt.Errorf("dictionary item input is invalid"))
	}
	now := s.now().UTC()
	return s.mutate(ctx, func(mutationCtx context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.UpdateItem(mutationCtx, dictionaryID, itemID, input, expected, now)
	})
}

func (s *Service) UpdateItemStatus(ctx context.Context, dictionaryID, itemID int64, status yesno.Value) error {
	if dictionaryID < 1 || itemID < 1 || !yesno.IsValid(status) {
		return apperror.InvalidRequest(fmt.Errorf("dictionary item status input is invalid"))
	}
	now := s.now().UTC()
	return s.mutate(ctx, func(mutationCtx context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.UpdateItemStatus(mutationCtx, dictionaryID, itemID, status, expected, now)
	})
}

func (s *Service) DeleteItem(ctx context.Context, dictionaryID, itemID int64) error {
	if dictionaryID < 1 || itemID < 1 {
		return apperror.InvalidRequest(fmt.Errorf("dictionary item coordinates are invalid"))
	}
	now := s.now().UTC()
	return s.mutate(ctx, func(mutationCtx context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.DeleteItem(mutationCtx, dictionaryID, itemID, expected, now)
	})
}

func (s *Service) mutate(ctx context.Context, apply func(context.Context, int64) (cachegeneration.MutationResult, error)) error {
	if s.generations == nil || s.states == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("dictionary mutation generation dependencies are not configured"))
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
				return apperror.DependencyUnavailable(fmt.Errorf("dictionary mutation budget exhausted: %w", err))
			}
			if waitErr := s.wait(ctx, s.waitStep); waitErr != nil {
				return apperror.DependencyUnavailable(waitErr)
			}
		default:
			return mapDictionaryError(err)
		}
	}
}

func (s *Service) mutateOnce(ctx context.Context, apply func(context.Context, int64) (cachegeneration.MutationResult, error)) error {
	base, err := s.generations.Current(ctx, s.scope)
	if err != nil {
		return err
	}
	lease, err := s.acquireLease(ctx, base)
	if err != nil {
		return err
	}
	mutationCtx, stopRenewal := s.startRenewal(ctx, lease)
	result, applyErr := apply(mutationCtx, base)
	stopRenewal()
	if applyErr != nil {
		return s.finishFailedMutation(ctx, lease, base, applyErr)
	}
	if !result.Changed {
		return lease.Rollback(ctx)
	}
	if err := lease.Commit(ctx, result.Generation); err != nil {
		s.logGenerationFailure("publish dictionary generation failed", result.Generation, result.OutboxID, err)
		return nil
	}
	if _, err := s.generations.MarkPublishedIfUnclaimed(ctx, result.OutboxID, s.now().UTC()); err != nil {
		s.logGenerationFailure("mark dictionary outbox published failed", result.Generation, result.OutboxID, err)
	}
	return nil
}

func (s *Service) acquireLease(ctx context.Context, base int64) (*cachegeneration.Lease, error) {
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

func (s *Service) finishFailedMutation(ctx context.Context, lease *cachegeneration.Lease, base int64, applyErr error) error {
	current, readErr := s.generations.Current(ctx, s.scope)
	if readErr != nil {
		return errors.Join(applyErr, readErr)
	}
	if current > base {
		if err := lease.Commit(ctx, current); err != nil {
			s.logGenerationFailure("publish dictionary generation after uncertain commit failed", current, 0, err)
		}
		if errors.Is(applyErr, errDictionaryMutationRolledBack) {
			return applyErr
		}
		return nil
	}
	if err := lease.Rollback(ctx); err != nil {
		return errors.Join(applyErr, err)
	}
	return applyErr
}

func (s *Service) readOptions(ctx context.Context, codes []string, language string) (OptionResult, error) {
	if s.cache == nil || s.generations == nil || s.states == nil {
		return nil, fmt.Errorf("dictionary cache generation dependencies are not configured")
	}
	variant, err := optionsVariant(codes, language)
	if err != nil {
		return nil, err
	}
	deadline := s.now().Add(s.readBudget)
	for s.now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		state, found, err := s.cache.ReadState(ctx)
		if err != nil {
			if errors.Is(err, cachegeneration.ErrStateCorrupt) {
				if repairErr := s.recoverOptionsState(ctx, codes, language, variant); repairErr != nil {
					if errors.Is(repairErr, ErrDictionaryGenerationAdvanced) {
						continue
					}
					return nil, errors.Join(err, repairErr)
				}
				continue
			}
			return nil, err
		}
		if !found {
			if repairErr := s.recoverOptionsState(ctx, codes, language, variant); repairErr != nil {
				if errors.Is(repairErr, ErrDictionaryGenerationAdvanced) {
					continue
				}
				return nil, errors.Join(cachegeneration.ErrStateMissing, repairErr)
			}
			continue
		}
		if state.State == cachegeneration.StateInvalidating {
			if s.now().Add(s.waitStep).After(deadline) {
				return nil, fmt.Errorf("dictionary cache state is invalidating")
			}
			if err := s.wait(ctx, s.waitStep); err != nil {
				return nil, err
			}
			continue
		}
		if state.State != cachegeneration.StateReady {
			return nil, fmt.Errorf("dictionary cache state is invalid")
		}
		cached, ok, err := s.cache.Get(ctx, state.Generation, codes, language)
		if err != nil && !errors.Is(err, ErrOptionsSnapshotCorrupt) {
			return nil, err
		}
		if err == nil && ok {
			return cached, nil
		}
		if err := s.fillOptions(ctx, state.Generation, codes, language, variant); err != nil {
			if errors.Is(err, ErrDictionaryGenerationAdvanced) {
				continue
			}
			return nil, err
		}
	}
	return nil, fmt.Errorf("dictionary options cache read budget exhausted")
}

func (s *Service) fillOptions(ctx context.Context, generation int64, codes []string, language, variant string) error {
	lease, err := s.cache.TryFill(ctx, generation, variant)
	if err != nil {
		return err
	}
	if lease == nil {
		return s.wait(ctx, s.waitStep)
	}
	defer func() { _ = lease.Release(ctx) }()
	workCtx, cancel := lease.WorkContext(ctx)
	defer cancel()
	if _, found, err := s.cache.Get(workCtx, generation, codes, language); err == nil && found {
		return nil
	}
	options, err := s.loadOptions(workCtx, codes, language)
	if err != nil {
		return err
	}
	if err := s.confirmReadyGeneration(workCtx, generation); err != nil {
		return err
	}
	return s.cache.Put(workCtx, generation, codes, language, options)
}

func (s *Service) recoverOptionsState(ctx context.Context, codes []string, language, variant string) error {
	lease, err := s.cache.TryRepair(ctx, variant)
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
	options, err := s.loadOptions(workCtx, codes, language)
	if err != nil {
		return err
	}
	if err := s.confirmReadyGeneration(workCtx, generation); err != nil {
		return err
	}
	return s.cache.Put(workCtx, generation, codes, language, options)
}

func (s *Service) confirmReadyGeneration(ctx context.Context, generation int64) error {
	state, found, err := s.cache.ReadState(ctx)
	if err != nil {
		return err
	}
	if !found || state.State != cachegeneration.StateReady || state.Generation != generation {
		return fmt.Errorf("%w: expected generation %d", ErrDictionaryGenerationAdvanced, generation)
	}
	return nil
}

func (s *Service) loadOptions(ctx context.Context, codes []string, language string) (OptionResult, error) {
	facts, err := s.repository.Options(ctx, codes)
	if err != nil {
		return nil, err
	}
	result := make(OptionResult, len(codes))
	status := make(map[string]yesno.Value, len(codes))
	for _, fact := range facts {
		status[fact.Code] = fact.DictionaryEnabled
		if fact.ItemID == nil {
			if _, exists := result[fact.Code]; !exists {
				result[fact.Code] = []OptionsItem{}
			}
			continue
		}
		if fact.Value == nil || fact.LabelZH == nil || fact.LabelEN == nil {
			return nil, fmt.Errorf("dictionary option fact is incomplete")
		}
		label := *fact.LabelZH
		if strings.HasPrefix(strings.ToLower(language), "en") {
			label = *fact.LabelEN
		}
		if label == "" {
			label = *fact.LabelZH
		}
		if label == "" || *fact.Value == "" {
			return nil, fmt.Errorf("dictionary item label or value is missing")
		}
		result[fact.Code] = append(result[fact.Code], OptionsItem{Label: label, Value: *fact.Value})
	}
	for _, code := range codes {
		enabled, found := status[code]
		if !found {
			return nil, fmt.Errorf("%w: dictionary %s", gorm.ErrRecordNotFound, code)
		}
		if enabled != yesno.Yes {
			return nil, fmt.Errorf("%w: dictionary %s is disabled", ErrConflict, code)
		}
		if _, found := result[code]; !found {
			result[code] = []OptionsItem{}
		}
	}
	return result, nil
}

func normalizeRequestedCodes(codes []string) ([]string, error) {
	if len(codes) == 0 || len(codes) > 50 {
		return nil, fmt.Errorf("codes are invalid")
	}
	seen := make(map[string]struct{}, len(codes))
	result := make([]string, 0, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if !codePattern.MatchString(code) || utf8.RuneCountInString(code) > 128 {
			return nil, fmt.Errorf("dictionary code is invalid")
		}
		if _, exists := seen[code]; exists {
			return nil, fmt.Errorf("codes contain duplicate")
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	return result, nil
}

func validText(value string, maxRunes int) bool {
	return value != "" && utf8.RuneCountInString(value) <= maxRunes
}

func mapDictionaryError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return apperror.NotFound(err)
	case errors.Is(err, ErrConflict):
		return apperror.Conflict("error.conflict", nil, err)
	default:
		return apperror.DependencyUnavailable(err)
	}
}

func waitForDictionary(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *Service) logGenerationFailure(message string, generation, outboxID int64, err error) {
	s.logger.Error(message,
		"namespace", s.scope.Namespace,
		"scopeKey", s.scope.ScopeKey,
		"generation", generation,
		"outboxId", outboxID,
		"errorClass", cachegeneration.ErrorClass(err),
	)
}
