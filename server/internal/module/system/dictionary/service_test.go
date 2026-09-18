package dictionary

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type fakeRepository struct {
	dictionaries map[string]Dictionary
	items        map[int64][]Item
	mutationErr  error
	createdItem  Item
}

type countingOptionsRepository struct {
	repository
	mutex sync.Mutex
	calls int
}

func (r *countingOptionsRepository) Options(ctx context.Context, codes []string) ([]optionFact, error) {
	r.mutex.Lock()
	r.calls++
	r.mutex.Unlock()
	return r.repository.Options(ctx, codes)
}

func (r *countingOptionsRepository) count() int {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.calls
}

type countingUpdateRepository struct {
	repository
	mutex sync.Mutex
	calls int
}

func (r *countingUpdateRepository) Update(ctx context.Context, id int64, input UpdateInput, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	r.mutex.Lock()
	r.calls++
	r.mutex.Unlock()
	return r.repository.Update(ctx, id, input, expected, now)
}

func (r *countingUpdateRepository) count() int {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.calls
}

func (f *fakeRepository) List(context.Context, ListQuery) ([]ListItem, int64, error) {
	return []ListItem{}, 0, nil
}
func (f *fakeRepository) Find(_ context.Context, id int64) (Dictionary, error) {
	for _, v := range f.dictionaries {
		if v.ID == id {
			return v, nil
		}
	}
	return Dictionary{}, gorm.ErrRecordNotFound
}
func (f *fakeRepository) Items(_ context.Context, id int64, enabled bool) ([]Item, error) {
	result := []Item{}
	for _, v := range f.items[id] {
		if !enabled || v.IsEnabled == yesno.Yes {
			result = append(result, v)
		}
	}
	return result, nil
}
func (f *fakeRepository) Options(_ context.Context, codes []string) ([]optionFact, error) {
	result := make([]optionFact, 0)
	for _, code := range codes {
		dictionary, ok := f.dictionaries[code]
		if !ok {
			continue
		}
		if len(f.items[dictionary.ID]) == 0 {
			result = append(result, optionFact{Code: code, DictionaryEnabled: dictionary.IsEnabled})
			continue
		}
		for _, item := range f.items[dictionary.ID] {
			if item.IsEnabled != yesno.Yes {
				continue
			}
			itemID, value, labelZH, labelEN := item.ID, item.Value, item.LabelZH, item.LabelEN
			result = append(result, optionFact{Code: code, DictionaryEnabled: dictionary.IsEnabled, ItemID: &itemID, Value: &value, LabelZH: &labelZH, LabelEN: &labelEN})
		}
	}
	return result, nil
}
func (f *fakeRepository) Create(_ context.Context, value *Dictionary, _ int64, _ time.Time) (cachegeneration.MutationResult, error) {
	if f.mutationErr != nil {
		return cachegeneration.MutationResult{}, f.mutationErr
	}
	value.ID = 76
	return cachegeneration.MutationResult{Changed: true, Generation: 2, OutboxID: 1}, nil
}
func (f *fakeRepository) Update(context.Context, int64, UpdateInput, int64, time.Time) (cachegeneration.MutationResult, error) {
	return cachegeneration.MutationResult{}, f.mutationErr
}
func (f *fakeRepository) UpdateStatus(context.Context, int64, yesno.Value, int64, time.Time) (cachegeneration.MutationResult, error) {
	return cachegeneration.MutationResult{}, f.mutationErr
}
func (f *fakeRepository) Delete(context.Context, int64, int64, time.Time) (cachegeneration.MutationResult, error) {
	return cachegeneration.MutationResult{}, f.mutationErr
}
func (f *fakeRepository) CreateItem(_ context.Context, value *Item, _ int64, _ time.Time) (cachegeneration.MutationResult, error) {
	f.createdItem = *value
	value.ID = 77
	if f.mutationErr != nil {
		return cachegeneration.MutationResult{}, f.mutationErr
	}
	return cachegeneration.MutationResult{Changed: true, Generation: 2, OutboxID: 1}, nil
}
func (f *fakeRepository) UpdateItem(context.Context, int64, int64, UpdateItemInput, int64, time.Time) (cachegeneration.MutationResult, error) {
	return cachegeneration.MutationResult{}, f.mutationErr
}
func (f *fakeRepository) UpdateItemStatus(context.Context, int64, int64, yesno.Value, int64, time.Time) (cachegeneration.MutationResult, error) {
	return cachegeneration.MutationResult{}, f.mutationErr
}
func (f *fakeRepository) DeleteItem(context.Context, int64, int64, int64, time.Time) (cachegeneration.MutationResult, error) {
	return cachegeneration.MutationResult{}, f.mutationErr
}

func TestOptionsUsesRequestedLanguageAndChineseFallback(t *testing.T) {
	repo := &fakeRepository{dictionaries: map[string]Dictionary{"user.gender": {ID: 1, Code: "user.gender", IsEnabled: yesno.Yes}}, items: map[int64][]Item{1: {{Value: "male", LabelZH: "男", LabelEN: "Male", IsEnabled: yesno.Yes}, {Value: "unknown", LabelZH: "未知", LabelEN: "", IsEnabled: yesno.Yes}}}}
	result, err := NewService(repo).loadOptions(context.Background(), []string{"user.gender"}, "en-US")
	if err != nil {
		t.Fatal(err)
	}
	if result["user.gender"][0].Label != "Male" || result["user.gender"][1].Label != "未知" {
		t.Fatalf("options=%+v", result)
	}
}

func TestOptionsRejectsDisabledAndDuplicateCodes(t *testing.T) {
	repo := &fakeRepository{dictionaries: map[string]Dictionary{"user.gender": {ID: 1, Code: "user.gender", IsEnabled: yesno.No}}, items: map[int64][]Item{}}
	service := NewService(repo)
	if _, err := service.loadOptions(context.Background(), []string{"user.gender"}, "zh-CN"); !errors.Is(err, ErrConflict) {
		t.Fatal("disabled dictionary was accepted")
	}
	repo.dictionaries["user.gender"] = Dictionary{ID: 1, Code: "user.gender", IsEnabled: yesno.Yes}
	if _, err := service.Options(context.Background(), []string{"user.gender", "user.gender"}, "zh-CN"); err == nil {
		t.Fatal("duplicate codes were accepted")
	}
}

func TestCreateMapsUniqueViolationToConflict(t *testing.T) {
	err := mapDictionaryError(ErrConflict)
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeConflict {
		t.Fatalf("unique violation error = %v, want conflict", err)
	}
}

func TestDictionaryItemLabelsAreTrimmedAndValidated(t *testing.T) {
	repo := &fakeRepository{dictionaries: map[string]Dictionary{"user.gender": {ID: 1, IsEnabled: yesno.Yes}}, items: map[int64][]Item{}}
	service := NewService(repo)
	if _, err := service.CreateItem(context.Background(), 1, CreateItemInput{Value: "x", LabelZH: "   ", LabelEN: "Label"}); appCode(err) != apperror.CodeInvalidRequest {
		t.Fatalf("blank label error = %v, want invalid request", err)
	}
	if _, err := service.CreateItem(context.Background(), 1, CreateItemInput{Value: "x", LabelZH: string(make([]rune, 257)), LabelEN: "Label"}); appCode(err) != apperror.CodeInvalidRequest {
		t.Fatalf("long label error = %v, want invalid request", err)
	}
}

func TestServiceMutationAdvancesGenerationAndPublishesOutbox(t *testing.T) {
	h := openDictionaryServiceHarness(t)
	dictionary := h.seedDictionary(t, Dictionary{Code: "user.gender", NameZH: "性别", NameEN: "Gender", IsEnabled: yesno.Yes})
	service := h.service(h.repository)

	id, err := service.CreateItem(h.ctx, dictionary.ID, CreateItemInput{Value: " male ", LabelZH: " 男 ", LabelEN: " Male ", Sort: 1})
	if err != nil {
		t.Fatalf("CreateItem() error = %v", err)
	}
	item, err := h.repository.FindItem(h.ctx, dictionary.ID, id)
	if err != nil {
		t.Fatal(err)
	}
	if item.Value != "male" || item.LabelZH != "男" || item.LabelEN != "Male" {
		t.Fatalf("created item = %+v", item)
	}
	h.assertGeneration(t, 2, []int64{2})
	var publishedAt *time.Time
	if err := h.db.WithContext(h.ctx).Raw(
		`SELECT published_at FROM system_config_cache_outbox WHERE namespace = ? AND scope_key = ? AND generation = 2`,
		h.scope.Namespace, h.scope.ScopeKey,
	).Scan(&publishedAt).Error; err != nil {
		t.Fatal(err)
	}
	if publishedAt == nil {
		t.Fatal("outbox was not marked published")
	}
}

func TestServiceOptionsHotHitAndCrossInstanceColdFillUseBoundedPostgres(t *testing.T) {
	h := openDictionaryServiceHarness(t)
	dictionary := h.seedDictionary(t, Dictionary{Code: "user.gender", NameZH: "性别", NameEN: "Gender", IsEnabled: yesno.Yes})
	h.seedItem(t, Item{DictionaryID: dictionary.ID, Value: "male", LabelZH: "男", LabelEN: "Male", IsEnabled: yesno.Yes})
	counting := &countingOptionsRepository{repository: h.repository}
	first := h.service(counting)
	secondClient, secondCache, secondStore := h.secondCache(t)
	defer func() { _ = secondClient.Close() }()
	second := NewService(counting)
	second.scope = h.scope
	second.cache = secondCache
	second.SetGenerations(h.generations, secondStore)
	second.renewInterval = time.Hour

	start := make(chan struct{})
	results := make(chan OptionResult, 2)
	failures := make(chan error, 2)
	var waitGroup sync.WaitGroup
	for _, service := range []*Service{first, second} {
		waitGroup.Add(1)
		go func(service *Service) {
			defer waitGroup.Done()
			<-start
			result, err := service.Options(h.ctx, []string{"user.gender"}, "en-US")
			if err != nil {
				failures <- err
				return
			}
			results <- result
		}(service)
	}
	close(start)
	waitGroup.Wait()
	close(failures)
	for err := range failures {
		t.Fatalf("cold Options() error = %v", err)
	}
	close(results)
	for result := range results {
		if len(result["user.gender"]) != 1 || result["user.gender"][0].Label != "Male" {
			t.Fatalf("cold options = %+v", result)
		}
	}
	if calls := counting.count(); calls != 1 {
		t.Fatalf("cold options PostgreSQL calls = %d want 1", calls)
	}
	if _, err := first.Options(h.ctx, []string{"user.gender"}, "en-US"); err != nil {
		t.Fatalf("hot Options() error = %v", err)
	}
	if calls := counting.count(); calls != 1 {
		t.Fatalf("hot options read PostgreSQL: calls=%d", calls)
	}
}

func TestServiceOptionsRepairsMissingAndCorruptStateFromAuthority(t *testing.T) {
	for _, test := range []struct {
		name string
		raw  *string
	}{
		{name: "missing"},
		{name: "corrupt", raw: stringPointer("not-json")},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := openDictionaryServiceHarness(t)
			dictionary := h.seedDictionary(t, Dictionary{Code: "user.gender", NameZH: "性别", NameEN: "Gender", IsEnabled: yesno.Yes})
			h.seedItem(t, Item{DictionaryID: dictionary.ID, Value: "male", LabelZH: "男", LabelEN: "Male", IsEnabled: yesno.Yes})
			if test.raw == nil {
				if err := h.client.Delete(h.ctx, cachegeneration.StateKey(h.scope)); err != nil {
					t.Fatal(err)
				}
			} else if err := h.client.SetString(h.ctx, cachegeneration.StateKey(h.scope), *test.raw, 0); err != nil {
				t.Fatal(err)
			}
			counting := &countingOptionsRepository{repository: h.repository}
			result, err := h.service(counting).Options(h.ctx, []string{"user.gender"}, "zh-CN")
			if err != nil || result["user.gender"][0].Label != "男" {
				t.Fatalf("Options() = %+v err=%v", result, err)
			}
			if calls := counting.count(); calls != 1 {
				t.Fatalf("repair PostgreSQL calls = %d want 1", calls)
			}
			state, found, err := h.store.Read(h.ctx, h.scope)
			if err != nil || !found || state.State != cachegeneration.StateReady || state.Generation != 1 {
				t.Fatalf("repaired state = %+v found=%v err=%v", state, found, err)
			}
		})
	}
}

func TestServiceOptionsInvalidatingBudgetAndRedisFailureSkipPostgres(t *testing.T) {
	t.Run("invalidating budget", func(t *testing.T) {
		h := openDictionaryServiceHarness(t)
		counting := &countingOptionsRepository{repository: h.repository}
		lease, err := h.store.Acquire(h.ctx, h.scope, 1)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = lease.Rollback(context.Background()) })
		service := h.service(counting)
		current := time.Date(2026, time.September, 17, 0, 0, 0, 0, time.UTC)
		waits := 0
		service.now = func() time.Time { return current }
		service.wait = func(context.Context, time.Duration) error {
			waits++
			current = current.Add(dictionaryWaitStep)
			return nil
		}
		_, err = service.Options(h.ctx, []string{"user.gender"}, "zh-CN")
		if appCode(err) != apperror.CodeDependencyUnavailable {
			t.Fatalf("Options() error = %v want dependency unavailable", err)
		}
		if waits > 25 || counting.count() != 0 {
			t.Fatalf("waits=%d PostgreSQL calls=%d", waits, counting.count())
		}
	})

	t.Run("redis failure", func(t *testing.T) {
		h := openDictionaryServiceHarness(t)
		counting := &countingOptionsRepository{repository: h.repository}
		service := h.service(counting)
		if err := h.client.Close(); err != nil {
			t.Fatal(err)
		}
		_, err := service.Options(h.ctx, []string{"user.gender"}, "zh-CN")
		if appCode(err) != apperror.CodeDependencyUnavailable || counting.count() != 0 {
			t.Fatalf("Options() error=%v PostgreSQL calls=%d", err, counting.count())
		}
	})
}

func TestServiceMutationBudgetExhaustionDoesNotEnterRepository(t *testing.T) {
	h := openDictionaryServiceHarness(t)
	dictionary := h.seedDictionary(t, Dictionary{Code: "user.gender", NameZH: "性别", NameEN: "Gender", IsEnabled: yesno.Yes})
	counting := &countingUpdateRepository{repository: h.repository}
	lease, err := h.store.Acquire(h.ctx, h.scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lease.Rollback(context.Background()) })
	service := h.service(counting)
	current := time.Date(2026, time.September, 17, 0, 0, 0, 0, time.UTC)
	waits := 0
	service.now = func() time.Time { return current }
	service.wait = func(context.Context, time.Duration) error {
		waits++
		current = current.Add(dictionaryWaitStep)
		return nil
	}
	err = service.Update(h.ctx, dictionary.ID, UpdateInput{NameZH: "用户性别", NameEN: "User gender"})
	if appCode(err) != apperror.CodeDependencyUnavailable || waits > 25 || counting.count() != 0 {
		t.Fatalf("Update() error=%v waits=%d repository calls=%d", err, waits, counting.count())
	}
}

type dictionaryServiceHarness struct {
	dictionaryRepositoryHarness
	client *projectredis.Client
	store  *cachegeneration.Store
	cache  *optionsCache
}

func openDictionaryServiceHarness(t *testing.T) dictionaryServiceHarness {
	t.Helper()
	base := openDictionaryRepositoryHarness(t)
	settings := loadDictionaryTestSettings(t)
	client, err := projectredis.Open(base.ctx, settings.RedisURL)
	if err != nil {
		t.Fatal(err)
	}
	store := cachegeneration.NewStore(client)
	if _, err := store.Reconcile(base.ctx, base.scope, 1); err != nil {
		t.Fatal(err)
	}
	cache := NewOptionsCache(client)
	cache.scope = base.scope
	cache.SetStateStore(store)
	t.Cleanup(func() {
		_ = client.Delete(context.Background(), cachegeneration.StateKey(base.scope))
		_ = client.Close()
	})
	return dictionaryServiceHarness{dictionaryRepositoryHarness: base, client: client, store: store, cache: cache}
}

func (h dictionaryServiceHarness) service(repository repository) *Service {
	service := NewService(repository)
	service.scope = h.scope
	service.renewInterval = time.Hour
	service.SetGenerations(h.generations, h.store)
	service.SetCache(h.cache)
	return service
}

func (h dictionaryServiceHarness) secondCache(t *testing.T) (*projectredis.Client, *optionsCache, *cachegeneration.Store) {
	t.Helper()
	settings := loadDictionaryTestSettings(t)
	client, err := projectredis.Open(h.ctx, settings.RedisURL)
	if err != nil {
		t.Fatal(err)
	}
	store := cachegeneration.NewStore(client)
	cache := NewOptionsCache(client)
	cache.scope = h.scope
	cache.SetStateStore(store)
	return client, cache, store
}

func stringPointer(value string) *string { return &value }

func appCode(err error) int {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return 0
}

func TestServiceValidateDependenciesRejectsIncompleteStartupWiring(t *testing.T) {
	if err := NewService(nil).ValidateDependencies(); err == nil {
		t.Fatal("ValidateDependencies accepted missing dictionary dependencies")
	}
}
