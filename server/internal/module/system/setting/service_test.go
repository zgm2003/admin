package setting

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/cacheGeneration"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"

	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const generationTestKey = "auth.captcha.ttl_minutes"

type fakeRepository struct {
	mutex          sync.Mutex
	rows           map[string]Record
	list           []Record
	create         Record
	update         Record
	findErr        error
	writeErr       error
	brand          BrandSettings
	mutation       cachegeneration.MutationResult
	hasMutation    bool
	mutationErr    error
	onMutation     func(context.Context) error
	onFind         func(context.Context, string) (Record, error)
	findDelay      time.Duration
	findCallCount  int
	brandCallCount int
	mutationCalls  int
}

func (f *fakeRepository) findCalls() int {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.findCallCount
}

func (f *fakeRepository) brandCalls() int {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.brandCallCount
}

func (f *fakeRepository) mutations() int {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.mutationCalls
}

func TestRetentionSettingValidationRejectsBeforeRepositoryAccess(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		value     string
		valueType int
	}{
		{"notification below minimum", sharedsetting.MessageNotificationRetentionDaysKey, "29", ValueTypeNumber},
		{"notification above maximum", sharedsetting.MessageNotificationRetentionDaysKey, "3651", ValueTypeNumber},
		{"realtime below minimum", sharedsetting.RealtimeEventRetentionDaysKey, "0", ValueTypeNumber},
		{"realtime above maximum", sharedsetting.RealtimeEventRetentionDaysKey, "31", ValueTypeNumber},
		{"scheduler below minimum", sharedsetting.SchedulerHistoryRetentionDaysKey, "6", ValueTypeNumber},
		{"scheduler above maximum", sharedsetting.SchedulerHistoryRetentionDaysKey, "3651", ValueTypeNumber},
		{"wrong type", sharedsetting.RealtimeEventRetentionDaysKey, "7", ValueTypeString},
		{"floating point", sharedsetting.RealtimeEventRetentionDaysKey, "1.5", ValueTypeNumber},
		{"overflow", sharedsetting.MessageNotificationRetentionDaysKey, "999999999999999999999999", ValueTypeNumber},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, operation := range []string{"create", "update"} {
				t.Run(operation, func(t *testing.T) {
					repository := &fakeRepository{rows: map[string]Record{test.key: {Key: test.key}}}
					service := NewService(repository)
					var err error
					if operation == "create" {
						_, err = service.Create(context.Background(), CreateInput{Key: test.key, Value: test.value, ValueType: test.valueType})
					} else {
						err = service.Update(context.Background(), test.key, UpdateInput{Value: test.value, ValueType: test.valueType})
					}
					if err == nil {
						t.Fatal("expected invalid request")
					}
					if repository.findCalls() != 0 || repository.mutations() != 0 {
						t.Fatalf("invalid input touched repository: finds=%d mutations=%d", repository.findCalls(), repository.mutations())
					}
				})
			}
		})
	}
}

func TestRetentionSettingValidationAcceptsBoundaries(t *testing.T) {
	for _, test := range []struct{ key, value string }{
		{sharedsetting.MessageNotificationRetentionDaysKey, "30"},
		{sharedsetting.MessageNotificationRetentionDaysKey, "3650"},
		{sharedsetting.RealtimeEventRetentionDaysKey, "1"},
		{sharedsetting.RealtimeEventRetentionDaysKey, "30"},
	} {
		if err := validateInput(test.key, test.value, ValueTypeNumber, ""); err != nil {
			t.Fatalf("validateInput(%q,%q)=%v", test.key, test.value, err)
		}
	}
}

func TestRequiredSettingCannotBeDisabledBeforeRepositoryAccess(t *testing.T) {
	for _, key := range []string{sharedsetting.MessageNotificationRetentionDaysKey, sharedsetting.RealtimeEventRetentionDaysKey} {
		repository := &fakeRepository{rows: map[string]Record{key: {Key: key}}}
		if err := NewService(repository).UpdateStatus(context.Background(), key, yesno.No); err == nil {
			t.Fatalf("UpdateStatus(%q, No) error=nil", key)
		}
		if repository.findCalls() != 0 || repository.mutations() != 0 {
			t.Fatalf("disable %q touched repository", key)
		}
	}
}

func (f *fakeRepository) runMutation(ctx context.Context) (cachegeneration.MutationResult, error) {
	f.mutex.Lock()
	f.mutationCalls++
	f.mutex.Unlock()
	if f.onMutation != nil {
		if err := f.onMutation(ctx); err != nil {
			return cachegeneration.MutationResult{}, err
		}
	}
	if f.mutationErr != nil {
		return cachegeneration.MutationResult{}, f.mutationErr
	}
	if f.writeErr != nil {
		return cachegeneration.MutationResult{}, f.writeErr
	}
	if f.hasMutation {
		return f.mutation, nil
	}
	return cachegeneration.MutationResult{Generation: 2, OutboxID: 1, Changed: true}, nil
}

func (f *fakeRepository) List(context.Context, ListQuery) ([]Record, int64, error) {
	return f.list, int64(len(f.list)), nil
}
func (f *fakeRepository) Find(ctx context.Context, key string) (Record, error) {
	f.mutex.Lock()
	f.findCallCount++
	delay := f.findDelay
	onFind := f.onFind
	findErr := f.findErr
	row, ok := f.rows[key]
	f.mutex.Unlock()

	if delay > 0 {
		select {
		case <-ctx.Done():
			return Record{}, ctx.Err()
		case <-time.After(delay):
		}
	}
	if onFind != nil {
		return onFind(ctx, key)
	}
	if findErr != nil {
		return Record{}, findErr
	}
	if !ok {
		return Record{}, ErrNotFound
	}
	return row, nil
}
func (f *fakeRepository) Create(ctx context.Context, row *Record, _ int64) (cachegeneration.MutationResult, error) {
	f.mutex.Lock()
	f.create = *row
	if row.ID == 0 {
		row.ID = 1
	}
	if f.rows == nil {
		f.rows = map[string]Record{}
	}
	f.rows[row.Key] = *row
	f.mutex.Unlock()
	return f.runMutation(ctx)
}
func (f *fakeRepository) Update(ctx context.Context, _ string, row Record, _ int64) (cachegeneration.MutationResult, error) {
	f.mutex.Lock()
	f.update = row
	f.mutex.Unlock()
	return f.runMutation(ctx)
}
func (f *fakeRepository) UpdateStatus(ctx context.Context, _ string, _ yesno.Value, _ int64, _ time.Time) (cachegeneration.MutationResult, error) {
	return f.runMutation(ctx)
}
func (f *fakeRepository) Delete(ctx context.Context, _ string, _ int64, _ time.Time) (cachegeneration.MutationResult, error) {
	return f.runMutation(ctx)
}
func (f *fakeRepository) FindBrand(context.Context) (BrandSettings, error) {
	f.mutex.Lock()
	f.brandCallCount++
	findErr := f.findErr
	rows := f.rows
	f.mutex.Unlock()
	if findErr != nil {
		return BrandSettings{}, findErr
	}
	return BrandSettings{
		TitleZhCN: rows[BrandTitleZhCNKey].Value, TitleEnUS: rows[BrandTitleEnUSKey].Value, DefaultAvatar: rows[BrandDefaultAvatarKey].Value,
	}, nil
}
func (f *fakeRepository) UpdateBrand(ctx context.Context, brand BrandSettings, _ int64, _ time.Time) (cachegeneration.MutationResult, error) {
	if f.writeErr != nil {
		return cachegeneration.MutationResult{}, f.writeErr
	}
	f.brand = brand
	return f.runMutation(ctx)
}

func TestServiceCreateRejectsUnknownValueTypeAndMalformedJSON(t *testing.T) {
	service := NewService(&fakeRepository{})
	if _, err := service.Create(context.Background(), CreateInput{Key: "auth.captcha.ttl_minutes", Value: "2", ValueType: 99}); err == nil {
		t.Fatal("expected unknown value type to fail")
	}
	if _, err := service.Create(context.Background(), CreateInput{Key: "auth.captcha.policy", Value: "{", ValueType: ValueTypeJSON}); err == nil {
		t.Fatal("expected malformed json to fail")
	}
}

func TestServiceCreateNormalizesAndReturnsSharedRecord(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	repo := &fakeRepository{}
	service := harness.service(repo)
	id, err := service.Create(harness.ctx, CreateInput{Key: " auth.captcha.ttl_minutes ", Value: " 2 ", ValueType: ValueTypeNumber, Description: " ttl "})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if id != 1 || repo.create.Key != "auth.captcha.ttl_minutes" || repo.create.Value != "2" || repo.create.Description != "ttl" {
		t.Fatalf("unexpected record: %#v", repo.create)
	}
	shared, err := service.FindByKey(harness.ctx, "auth.captcha.ttl_minutes")
	if err != nil {
		t.Fatalf("FindByKey() error = %v", err)
	}
	if shared.Key != repo.create.Key {
		t.Fatalf("unexpected shared record: %#v", shared)
	}
}

func TestServiceBuiltinCannotDelete(t *testing.T) {
	repo := &fakeRepository{rows: map[string]Record{"auth.captcha.ttl_minutes": {Key: "auth.captcha.ttl_minutes", IsBuiltin: yesno.Yes}}}
	service := NewService(repo)
	if err := service.Delete(context.Background(), "auth.captcha.ttl_minutes"); err == nil {
		t.Fatal("expected builtin delete to fail")
	}
}

func TestServicePropagatesRepositoryFailure(t *testing.T) {
	repoErr := errors.New("database down")
	harness := openSettingGenerationHarness(t)
	service := harness.service(&fakeRepository{findErr: repoErr})
	if _, err := service.FindByKey(harness.ctx, "auth.captcha.ttl_minutes"); !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestServiceReadsAndAtomicallyUpdatesBrandSettings(t *testing.T) {
	repo := &fakeRepository{rows: map[string]Record{
		BrandTitleZhCNKey:     {Key: BrandTitleZhCNKey, Value: "智澜", ValueType: ValueTypeString, IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes},
		BrandTitleEnUSKey:     {Key: BrandTitleEnUSKey, Value: "ZHILAN", ValueType: ValueTypeString, IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes},
		BrandDefaultAvatarKey: {Key: BrandDefaultAvatarKey, Value: "", ValueType: ValueTypeString, IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes},
	}}
	harness := openSettingGenerationHarness(t)
	service := harness.service(repo)
	brand, err := service.Brand(harness.ctx)
	if err != nil || brand.TitleZhCN != "智澜" || brand.TitleEnUS != "ZHILAN" || brand.DefaultAvatar != "" {
		t.Fatalf("brand=%+v error=%v", brand, err)
	}

	err = service.UpdateBrand(harness.ctx, BrandSettings{
		TitleZhCN: " 新标题 ", TitleEnUS: " New title ", DefaultAvatar: "avatar/2026/09/15/default.png",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := BrandSettings{TitleZhCN: "新标题", TitleEnUS: "New title", DefaultAvatar: "avatar/2026/09/15/default.png"}
	if repo.brand != want {
		t.Fatalf("updated brand=%+v want=%+v", repo.brand, want)
	}
}

func TestServiceRejectsInvalidBrandSettings(t *testing.T) {
	service := NewService(&fakeRepository{})
	for _, input := range []BrandSettings{
		{TitleZhCN: "", TitleEnUS: "ZHILAN"},
		{TitleZhCN: "智澜", TitleEnUS: ""},
		{TitleZhCN: "智澜", TitleEnUS: "ZHILAN", DefaultAvatar: "other/default.png"},
		{TitleZhCN: "智澜", TitleEnUS: "ZHILAN", DefaultAvatar: "avatar/../secret.png"},
	} {
		if err := service.UpdateBrand(context.Background(), input); err == nil {
			t.Fatalf("input=%+v should fail", input)
		}
	}
}

func TestServiceBrandFailsClosedWhenCacheDependenciesAreMissing(t *testing.T) {
	service := NewService(&fakeRepository{})
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Brand() panicked without cache dependencies: %v", recovered)
		}
	}()

	if _, err := service.Brand(context.Background()); err == nil {
		t.Fatal("Brand() returned success without cache dependencies")
	}
}

func TestRepositoryUpdateBrandRollsBackWhenOneSettingIsMissing(t *testing.T) {
	db, ctx := openSettingDatabase(t)
	now := time.Now().UTC()
	if err := db.WithContext(ctx).Create([]Model{
		{Key: BrandTitleZhCNKey, Value: "旧中文", ValueType: ValueTypeString, Description: "", IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes, CreatedAt: now, UpdatedAt: now},
		{Key: BrandTitleEnUSKey, Value: "OLD", ValueType: ValueTypeString, Description: "", IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes, CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	repository.SetGenerations(cachegeneration.NewRepository(db))

	_, err := repository.UpdateBrand(ctx, BrandSettings{
		TitleZhCN: "新中文", TitleEnUS: "NEW", DefaultAvatar: "avatar/default.png",
	}, 1, now.Add(time.Minute))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateBrand() error=%v want=%v", err, ErrNotFound)
	}

	var rows []Model
	if err = db.WithContext(ctx).Order("setting_key ASC").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Value != "OLD" || rows[1].Value != "旧中文" {
		t.Fatalf("rows=%+v, transaction partially updated brand settings", rows)
	}
	assertSettingGeneration(t, db, ctx, "global", 1)
	assertSettingOutbox(t, db, ctx, "global", 0, 0, false)
}

func TestServiceMutationAcquiresLeaseBeforeDatabaseMutation(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	seedSettingRow(t, harness.db, harness.ctx, numericSettingRow(generationTestKey, "2", yesno.No))
	fake := &fakeRepository{rows: map[string]Record{
		generationTestKey: {Key: generationTestKey, Value: "2", ValueType: ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.No},
	}}
	observed := ""
	fake.onMutation = func(mutationCtx context.Context) error {
		state, found, err := cachegeneration.NewStore(harness.client).Read(mutationCtx, harness.scope)
		if err != nil || !found {
			return fmt.Errorf("state unavailable during mutation: found=%v err=%v", found, err)
		}
		observed = state.State
		return nil
	}
	service := harness.service(fake)

	if err := service.Update(harness.ctx, generationTestKey, UpdateInput{Value: "5", ValueType: ValueTypeNumber}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if observed != cachegeneration.StateInvalidating {
		t.Fatalf("mutation observed state %q want %q", observed, cachegeneration.StateInvalidating)
	}
	state, found, err := harness.store.Read(harness.ctx, harness.scope)
	if err != nil || !found || state.State != cachegeneration.StateReady || state.Generation != 2 {
		t.Fatalf("state after mutation = %+v found=%v err=%v", state, found, err)
	}
}

func TestServiceMutationPublishesGenerationAndMarksOutbox(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	seedSettingRow(t, harness.db, harness.ctx, numericSettingRow(generationTestKey, "2", yesno.No))
	repository := NewRepository(harness.db)
	repository.scope = harness.scope
	repository.SetGenerations(harness.generations)
	service := harness.service(repository)

	if err := service.Update(harness.ctx, generationTestKey, UpdateInput{Value: "5", ValueType: ValueTypeNumber}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	assertSettingGeneration(t, harness.db, harness.ctx, harness.scope.ScopeKey, 2)
	assertSettingOutbox(t, harness.db, harness.ctx, harness.scope.ScopeKey, 1, 2, true)
	state, found, err := harness.store.Read(harness.ctx, harness.scope)
	if err != nil || !found || state.State != cachegeneration.StateReady || state.Generation != 2 {
		t.Fatalf("state after mutation = %+v found=%v err=%v", state, found, err)
	}
}

func TestServiceMutationRollsBackLeaseWhenDatabaseFails(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{
		rows: map[string]Record{
			generationTestKey: {Key: generationTestKey, Value: "2", ValueType: ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.No},
		},
		mutationErr: errors.New("database down"),
	}
	service := harness.service(fake)

	if err := service.Update(harness.ctx, generationTestKey, UpdateInput{Value: "5", ValueType: ValueTypeNumber}); err == nil {
		t.Fatal("expected dependency failure")
	}
	state, found, err := harness.store.Read(harness.ctx, harness.scope)
	if err != nil || !found || state.State != cachegeneration.StateReady || state.Generation != 1 {
		t.Fatalf("state after rollback = %+v found=%v err=%v", state, found, err)
	}
	assertSettingGeneration(t, harness.db, harness.ctx, harness.scope.ScopeKey, 1)
}

func TestServiceMutationRollsBackLeaseOnNoOp(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{
		rows: map[string]Record{
			generationTestKey: {Key: generationTestKey, Value: "2", ValueType: ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.No},
		},
		hasMutation: true,
		mutation:    cachegeneration.MutationResult{Changed: false},
	}
	service := harness.service(fake)

	if err := service.UpdateStatus(harness.ctx, generationTestKey, yesno.Yes); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	state, found, err := harness.store.Read(harness.ctx, harness.scope)
	if err != nil || !found || state.State != cachegeneration.StateReady || state.Generation != 1 {
		t.Fatalf("state after no-op = %+v found=%v err=%v", state, found, err)
	}
}

func TestServiceMutationReturnsSuccessWhenPublishFails(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{rows: map[string]Record{
		generationTestKey: {Key: generationTestKey, Value: "2", ValueType: ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.No},
	}}
	fake.onMutation = func(context.Context) error {
		return harness.client.Delete(context.Background(), cachegeneration.StateKey(harness.scope))
	}
	service := harness.service(fake)

	if err := service.Update(harness.ctx, generationTestKey, UpdateInput{Value: "5", ValueType: ValueTypeNumber}); err != nil {
		t.Fatalf("publish failure must not fail the committed business write: %v", err)
	}
	if _, found, err := harness.store.Read(harness.ctx, harness.scope); err != nil || found {
		t.Fatalf("state should be gone after lease loss: found=%v err=%v", found, err)
	}
}

func TestServiceMutationTreatsUncertainCommitAsCommitted(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{rows: map[string]Record{
		generationTestKey: {Key: generationTestKey, Value: "2", ValueType: ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.No},
	}}
	fake.onMutation = func(mutationCtx context.Context) error {
		if err := harness.db.WithContext(mutationCtx).Exec(
			`UPDATE system_config_cache_generation SET generation = 2 WHERE namespace = 'system.setting' AND scope_key = ?`,
			harness.scope.ScopeKey).Error; err != nil {
			return err
		}
		return errors.New("connection reset after commit")
	}
	service := harness.service(fake)

	if err := service.Update(harness.ctx, generationTestKey, UpdateInput{Value: "5", ValueType: ValueTypeNumber}); err != nil {
		t.Fatalf("committed write with uncertain acknowledgement must succeed: %v", err)
	}
	state, found, err := harness.store.Read(harness.ctx, harness.scope)
	if err != nil || !found || state.State != cachegeneration.StateReady || state.Generation != 2 {
		t.Fatalf("state after uncertain commit = %+v found=%v err=%v", state, found, err)
	}
}

func TestServiceMutationCancelsTransactionWhenLeaseRenewalFails(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{rows: map[string]Record{
		generationTestKey: {Key: generationTestKey, Value: "2", ValueType: ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.No},
	}}
	cancelled := make(chan error, 1)
	fake.onMutation = func(mutationCtx context.Context) error {
		if err := harness.client.Delete(context.Background(), cachegeneration.StateKey(harness.scope)); err != nil {
			return err
		}
		select {
		case <-mutationCtx.Done():
			cancelled <- mutationCtx.Err()
			return mutationCtx.Err()
		case <-time.After(5 * time.Second):
			return errors.New("mutation context was not cancelled by the lease renewal")
		}
	}
	service := harness.service(fake)

	if err := service.Update(harness.ctx, generationTestKey, UpdateInput{Value: "5", ValueType: ValueTypeNumber}); err == nil {
		t.Fatal("expected dependency failure")
	}
	select {
	case cause := <-cancelled:
		if cause == nil {
			t.Fatal("mutation context was cancelled without cause")
		}
	default:
		t.Fatal("mutation context was not cancelled")
	}
	assertSettingGeneration(t, harness.db, harness.ctx, harness.scope.ScopeKey, 1)
}

func TestServiceCacheHitSkipsPostgres(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{}
	service := harness.service(fake)
	now := time.Now().UTC()
	row := Record{
		ID: 7, Key: generationTestKey, Value: "2", ValueType: ValueTypeNumber, Description: "",
		IsEnabled: yesno.Yes, IsBuiltin: yesno.No, CreatedAt: now, UpdatedAt: now,
	}
	if err := harness.cache.PutRecord(harness.ctx, 1, row); err != nil {
		t.Fatal(err)
	}
	got, err := service.Find(harness.ctx, generationTestKey)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if got.ID != 7 || got.Value != "2" {
		t.Fatalf("cached record = %+v", got)
	}
	if calls := fake.findCalls(); calls != 0 {
		t.Fatalf("ready hit must not read PostgreSQL: calls=%d", calls)
	}
}

func TestServiceCacheRejectsSnapshotCoordinateMismatch(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{rows: map[string]Record{
		generationTestKey: {Key: generationTestKey, Value: "5", ValueType: ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.No},
	}}
	service := harness.service(fake)
	snapshotKey, err := cachegeneration.SnapshotKey(harness.scope, 1, recordVariant(generationTestKey))
	if err != nil {
		t.Fatal(err)
	}
	payload, err := encodeRecordPayload(2, Record{Key: generationTestKey, Value: "9", ValueType: ValueTypeNumber})
	if err != nil {
		t.Fatal(err)
	}
	if err := harness.client.SetString(harness.ctx, snapshotKey, payload, 0); err != nil {
		t.Fatal(err)
	}

	got, err := service.Find(harness.ctx, generationTestKey)
	if err != nil || got.Value != "5" {
		t.Fatalf("record = %+v err = %v", got, err)
	}
	if calls := fake.findCalls(); calls != 1 {
		t.Fatalf("coordinate mismatch must trigger repair: calls=%d", calls)
	}
}

func TestServiceCacheMissingStateRecoversFromPostgres(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{rows: map[string]Record{
		generationTestKey: {Key: generationTestKey, Value: "2", ValueType: ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.No},
	}}
	service := harness.service(fake)
	if err := harness.client.Delete(harness.ctx, cachegeneration.StateKey(harness.scope)); err != nil {
		t.Fatal(err)
	}

	got, err := service.Find(harness.ctx, generationTestKey)
	if err != nil || got.Value != "2" {
		t.Fatalf("record = %+v err = %v", got, err)
	}
	state, found, err := harness.store.Read(harness.ctx, harness.scope)
	if err != nil || !found || state.State != cachegeneration.StateReady || state.Generation != 1 {
		t.Fatalf("recovered state = %+v found=%v err=%v", state, found, err)
	}
	if calls := fake.findCalls(); calls != 1 {
		t.Fatalf("missing state recovery reads = %d want 1", calls)
	}
}

func TestServiceColdFillSingleLeaderAndFollowerBudget(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{
		rows: map[string]Record{
			generationTestKey: {Key: generationTestKey, Value: "2", ValueType: ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.No},
		},
		findDelay: 30 * time.Millisecond,
	}
	service := harness.service(fake)

	// 本机 Redis 在高并发下单条命令可达 ~20ms，因此这里断言“有界失败闭合 +
	// 单 leader 回源”；超预算返回依赖错误是协议定义的行为，
	// 恢复能力由 TestServiceColdFillFollowersRecoverWithinBudget 覆盖。
	const workers = 32
	start := time.Now()
	var waitGroup sync.WaitGroup
	failures := make(chan error, workers)
	for index := 0; index < workers; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			if _, err := service.Find(harness.ctx, generationTestKey); err != nil {
				failures <- err
			}
		}()
	}
	waitGroup.Wait()
	close(failures)
	elapsed := time.Since(start)

	if elapsed > 3*time.Second {
		t.Fatalf("cold fill exceeded the bounded budget: %v", elapsed)
	}
	for err := range failures {
		var appErr *apperror.Error
		if !errors.As(err, &appErr) || appErr.Code != apperror.CodeDependencyUnavailable {
			t.Fatalf("follower failure = %v want dependency unavailable", err)
		}
	}
	if calls := fake.findCalls(); calls != 1 {
		t.Fatalf("cold fill must have a single PostgreSQL leader: calls=%d", calls)
	}
	snapshotKey, err := cachegeneration.SnapshotKey(harness.scope, 1, recordVariant(generationTestKey))
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := harness.client.GetString(harness.ctx, snapshotKey); err != nil || !found {
		t.Fatalf("cold fill snapshot missing: found=%v err=%v", found, err)
	}
	followUp, err := service.Find(harness.ctx, generationTestKey)
	if err != nil || followUp.Value != "2" {
		t.Fatalf("follow-up read = %+v err = %v", followUp, err)
	}
	if calls := fake.findCalls(); calls != 1 {
		t.Fatalf("follow-up read must hit the snapshot: calls=%d", calls)
	}
}

// TestServiceColdFillFollowersRecoverWithinBudget 用较小并发验证 followers
// 能在约 500ms 预算内等到 leader 发布快照。
func TestServiceColdFillFollowersRecoverWithinBudget(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{
		rows: map[string]Record{
			generationTestKey: {Key: generationTestKey, Value: "2", ValueType: ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.No},
		},
		findDelay: 20 * time.Millisecond,
	}
	service := harness.service(fake)

	const workers = 4
	var waitGroup sync.WaitGroup
	failures := make(chan error, workers)
	for index := 0; index < workers; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			if _, err := service.Find(harness.ctx, generationTestKey); err != nil {
				failures <- err
			}
		}()
	}
	waitGroup.Wait()
	close(failures)
	for err := range failures {
		t.Fatalf("follower did not recover within the budget: %v", err)
	}
	if calls := fake.findCalls(); calls != 1 {
		t.Fatalf("followers must reuse the leader snapshot: calls=%d", calls)
	}
}

func TestServiceColdFillDropsStaleGenerationResult(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	var mutex sync.Mutex
	loaded := make(chan struct{})
	release := make(chan struct{})
	first := true
	current := Record{Key: generationTestKey, Value: "old", ValueType: ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.No}
	fake := &fakeRepository{}
	fake.onFind = func(ctx context.Context, _ string) (Record, error) {
		mutex.Lock()
		isFirst := first
		first = false
		row := current
		mutex.Unlock()
		if isFirst {
			close(loaded)
			select {
			case <-release:
			case <-ctx.Done():
				return Record{}, ctx.Err()
			}
		}
		return row, nil
	}
	service := harness.service(fake)

	results := make(chan Record, 1)
	failures := make(chan error, 1)
	go func() {
		row, err := service.Find(harness.ctx, generationTestKey)
		if err != nil {
			failures <- err
			return
		}
		results <- row
	}()
	<-loaded

	mutex.Lock()
	current = Record{Key: generationTestKey, Value: "new", ValueType: ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.No}
	mutex.Unlock()
	if err := harness.db.WithContext(harness.ctx).Exec(
		`UPDATE system_config_cache_generation SET generation = 2 WHERE namespace = 'system.setting' AND scope_key = ?`,
		harness.scope.ScopeKey).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := harness.store.Reconcile(harness.ctx, harness.scope, 2); err != nil {
		t.Fatal(err)
	}
	close(release)

	select {
	case err := <-failures:
		t.Fatalf("Find() error = %v", err)
	case row := <-results:
		if row.Value != "new" {
			t.Fatalf("stale generation result was returned: %+v", row)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Find did not return after the generation changed")
	}

	staleKey, err := cachegeneration.SnapshotKey(harness.scope, 1, recordVariant(generationTestKey))
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := harness.client.GetString(harness.ctx, staleKey); err != nil || found {
		t.Fatalf("generation 1 snapshot must not be written: found=%v err=%v", found, err)
	}
	freshKey, err := cachegeneration.SnapshotKey(harness.scope, 2, recordVariant(generationTestKey))
	if err != nil {
		t.Fatal(err)
	}
	raw, found, err := harness.client.GetString(harness.ctx, freshKey)
	if err != nil || !found || !strings.Contains(raw, `"new"`) {
		t.Fatalf("generation 2 snapshot = %q found=%v err=%v", raw, found, err)
	}
}

func TestServiceCacheRedisFailureDoesNotReadPostgres(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{}
	service := harness.service(fake)
	if err := harness.client.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := service.Find(harness.ctx, generationTestKey); err == nil {
		t.Fatal("expected dependency failure")
	}
	if calls := fake.findCalls(); calls != 0 {
		t.Fatalf("redis failure must not fall back to PostgreSQL: calls=%d", calls)
	}
}

func TestServiceCacheInvalidatingWaitExhaustsBudget(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{rows: map[string]Record{
		generationTestKey: {Key: generationTestKey, Value: "2", ValueType: ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.No},
	}}
	service := harness.service(fake)
	service.readBudget = 80 * time.Millisecond
	lease, err := harness.store.Acquire(harness.ctx, harness.scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lease.Rollback(context.Background()) }()

	if _, err := service.Find(harness.ctx, generationTestKey); err == nil {
		t.Fatal("expected dependency failure while invalidating")
	}
	if calls := fake.findCalls(); calls != 0 {
		t.Fatalf("invalidating wait must not read PostgreSQL: calls=%d", calls)
	}
}

func TestServiceBrandUsesSingleSnapshotVariant(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{rows: map[string]Record{
		BrandTitleZhCNKey:     {Key: BrandTitleZhCNKey, Value: "智澜", ValueType: ValueTypeString, IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes},
		BrandTitleEnUSKey:     {Key: BrandTitleEnUSKey, Value: "ZHILAN", ValueType: ValueTypeString, IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes},
		BrandDefaultAvatarKey: {Key: BrandDefaultAvatarKey, Value: "", ValueType: ValueTypeString, IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes},
	}}
	service := harness.service(fake)

	first, err := service.Brand(harness.ctx)
	if err != nil || first.TitleZhCN != "智澜" {
		t.Fatalf("brand = %+v err = %v", first, err)
	}
	second, err := service.Brand(harness.ctx)
	if err != nil || second != first {
		t.Fatalf("cached brand = %+v err = %v", second, err)
	}
	if calls := fake.brandCalls(); calls != 1 {
		t.Fatalf("brand must be one PostgreSQL read: calls=%d", calls)
	}
	snapshotKey, err := cachegeneration.SnapshotKey(harness.scope, 1, brandSnapshotVariant)
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := harness.client.GetString(harness.ctx, snapshotKey); err != nil || !found {
		t.Fatalf("brand snapshot missing: found=%v err=%v", found, err)
	}
}

func TestServiceCacheCorruptSnapshotIsRepairedByLeader(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{rows: map[string]Record{
		generationTestKey: {Key: generationTestKey, Value: "2", ValueType: ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.No},
	}}
	service := harness.service(fake)
	snapshotKey, err := cachegeneration.SnapshotKey(harness.scope, 1, recordVariant(generationTestKey))
	if err != nil {
		t.Fatal(err)
	}
	if err := harness.client.SetString(harness.ctx, snapshotKey, "not-json", 0); err != nil {
		t.Fatal(err)
	}

	got, err := service.Find(harness.ctx, generationTestKey)
	if err != nil || got.Value != "2" {
		t.Fatalf("record = %+v err = %v", got, err)
	}
	if calls := fake.findCalls(); calls != 1 {
		t.Fatalf("corrupt snapshot repair reads = %d want 1", calls)
	}
	raw, found, err := harness.client.GetString(harness.ctx, snapshotKey)
	if err != nil || !found || strings.Contains(raw, "not-json") {
		t.Fatalf("snapshot not repaired: %q found=%v err=%v", raw, found, err)
	}
}

// commandCounter 记录单个 Redis client 实际发出的命令，用于热点预算断言。
type commandCounter struct {
	mutex  sync.Mutex
	counts map[string]int
}

func newCommandCounter() *commandCounter {
	return &commandCounter{counts: map[string]int{}}
}

func (c *commandCounter) DialHook(next goredis.DialHook) goredis.DialHook { return next }

func (c *commandCounter) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, command goredis.Cmder) error {
		c.record([]goredis.Cmder{command})
		return next(ctx, command)
	}
}

func (c *commandCounter) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return func(ctx context.Context, commands []goredis.Cmder) error {
		c.record(commands)
		return next(ctx, commands)
	}
}

func (c *commandCounter) record(commands []goredis.Cmder) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	for _, command := range commands {
		c.counts[command.Name()]++
	}
}

func (c *commandCounter) countsOf(names ...string) int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	total := 0
	for _, name := range names {
		total += c.counts[name]
	}
	return total
}

func (c *commandCounter) reset() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.counts = map[string]int{}
}

func TestServiceHotReadUsesOneStateAndOneSnapshotCommand(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{rows: map[string]Record{
		generationTestKey: {Key: generationTestKey, Value: "2", ValueType: ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.No},
	}}
	counter := newCommandCounter()
	harness.client.UniversalClient().AddHook(counter)
	service := harness.service(fake)

	if _, err := service.Find(harness.ctx, generationTestKey); err != nil {
		t.Fatalf("cold read error = %v", err)
	}
	counter.reset()
	row, err := service.Find(harness.ctx, generationTestKey)
	if err != nil || row.Value != "2" {
		t.Fatalf("hot read = %+v err=%v", row, err)
	}
	if gets := counter.countsOf("get"); gets != 2 {
		t.Fatalf("hot read redis GETs = %d want 2 (state + snapshot)", gets)
	}
	if calls := fake.findCalls(); calls != 1 {
		t.Fatalf("hot read must not hit PostgreSQL again: calls=%d", calls)
	}
}

func TestServiceMutationWritesSingleStateWithoutKeyScan(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	seedSettingRow(t, harness.db, harness.ctx, numericSettingRow(generationTestKey, "2", yesno.No))
	repository := NewRepository(harness.db)
	repository.scope = harness.scope
	repository.SetGenerations(harness.generations)
	counter := newCommandCounter()
	harness.client.UniversalClient().AddHook(counter)
	service := harness.service(repository)
	// 关闭续租，确保计数只包含 acquire 与 commit 两条脚本。
	service.renewInterval = time.Hour

	if err := service.Update(harness.ctx, generationTestKey, UpdateInput{Value: "5", ValueType: ValueTypeNumber}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if scripts := counter.countsOf("eval", "evalsha"); scripts != 2 {
		t.Fatalf("mutation redis scripts = %d want 2 (acquire + commit)", scripts)
	}
	for _, forbidden := range []string{"scan", "del", "flushdb", "flushall", "set"} {
		if count := counter.countsOf(forbidden); count != 0 {
			t.Fatalf("mutation must not use %s: %d", forbidden, count)
		}
	}
	assertSettingGeneration(t, harness.db, harness.ctx, harness.scope.ScopeKey, 2)
	assertSettingOutbox(t, harness.db, harness.ctx, harness.scope.ScopeKey, 1, 2, true)
}

type blockingUpdateRepository struct {
	repository
	entered chan struct{}
	release <-chan struct{}
	once    sync.Once
}

func (r *blockingUpdateRepository) Update(ctx context.Context, key string, row Record, expected int64) (cachegeneration.MutationResult, error) {
	r.once.Do(func() { close(r.entered) })
	select {
	case <-r.release:
	case <-ctx.Done():
		return cachegeneration.MutationResult{}, ctx.Err()
	}
	return r.repository.Update(ctx, key, row, expected)
}

type generationRaceRepository struct {
	repository
	db              *gorm.DB
	generations     *cachegeneration.Repository
	scope           cachegeneration.Scope
	mutex           sync.Mutex
	calls           int
	rolledBackValue string
}

func (r *generationRaceRepository) Update(ctx context.Context, key string, row Record, expected int64) (cachegeneration.MutationResult, error) {
	r.mutex.Lock()
	r.calls++
	first := r.calls == 1
	r.mutex.Unlock()
	if first {
		if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			_, err := r.generations.AdvanceTx(ctx, tx, r.scope, expected, time.Now().UTC())
			return err
		}); err != nil {
			return cachegeneration.MutationResult{}, err
		}
	}
	result, err := r.repository.Update(ctx, key, row, expected)
	if first && err != nil {
		current, findErr := r.repository.Find(ctx, key)
		if findErr != nil {
			return cachegeneration.MutationResult{}, errors.Join(err, findErr)
		}
		r.mutex.Lock()
		r.rolledBackValue = current.Value
		r.mutex.Unlock()
	}
	return result, err
}

func TestServiceTwoInstancesSerializeConcurrentMutationsWithinBudget(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	seedSettingRow(t, harness.db, harness.ctx, numericSettingRow(generationTestKey, "2", yesno.No))
	settings := loadSettingTestSettings(t)
	secondClient, err := projectredis.Open(harness.ctx, settings.RedisURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = secondClient.Close() })
	secondStore := cachegeneration.NewStore(secondClient)

	release := make(chan struct{})
	holding := make(chan struct{})
	firstRepository := NewRepository(harness.db)
	firstRepository.scope = harness.scope
	firstRepository.SetGenerations(harness.generations)
	firstService := harness.service(&blockingUpdateRepository{
		repository: firstRepository,
		entered:    holding,
		release:    release,
	})
	secondRepository := NewRepository(harness.db)
	secondRepository.scope = harness.scope
	secondRepository.SetGenerations(harness.generations)
	secondService := harness.service(secondRepository)
	secondService.states = secondStore
	secondWaiting := make(chan struct{})
	continueSecond := make(chan struct{})
	var waitOnce sync.Once
	secondService.wait = func(ctx context.Context, _ time.Duration) error {
		waitOnce.Do(func() { close(secondWaiting) })
		select {
		case <-continueSecond:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- firstService.Update(harness.ctx, generationTestKey, UpdateInput{Value: "5", ValueType: ValueTypeNumber})
	}()
	<-holding

	secondDone := make(chan error, 1)
	go func() {
		secondDone <- secondService.Update(harness.ctx, generationTestKey, UpdateInput{Value: "6", ValueType: ValueTypeNumber})
	}()
	<-secondWaiting
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first mutation error = %v", err)
	}
	close(continueSecond)
	if err := <-secondDone; err != nil {
		t.Fatalf("second mutation error = %v", err)
	}

	row, err := secondRepository.Find(harness.ctx, generationTestKey)
	if err != nil || row.Value != "6" {
		t.Fatalf("final setting = %+v err=%v want last writer value 6", row, err)
	}
	assertSettingGeneration(t, harness.db, harness.ctx, harness.scope.ScopeKey, 3)
	assertSettingOutboxGenerations(t, harness.db, harness.ctx, harness.scope.ScopeKey, []int64{2, 3})
	state, found, err := secondStore.Read(harness.ctx, harness.scope)
	if err != nil || !found || state.State != cachegeneration.StateReady || state.Generation != 3 {
		t.Fatalf("state after concurrent mutations = %+v found=%v err=%v", state, found, err)
	}
}

func TestServiceMutationRetriesWholeFlowAfterGenerationChangesInsideTransaction(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	seedSettingRow(t, harness.db, harness.ctx, numericSettingRow(generationTestKey, "2", yesno.No))
	base := NewRepository(harness.db)
	base.scope = harness.scope
	base.SetGenerations(harness.generations)
	racing := &generationRaceRepository{
		repository:  base,
		db:          harness.db,
		generations: harness.generations,
		scope:       harness.scope,
	}
	service := harness.service(racing)
	service.wait = func(context.Context, time.Duration) error { return nil }

	if err := service.Update(harness.ctx, generationTestKey, UpdateInput{Value: "5", ValueType: ValueTypeNumber}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	row, err := base.Find(harness.ctx, generationTestKey)
	if err != nil || row.Value != "5" {
		t.Fatalf("final setting = %+v err=%v", row, err)
	}
	if racing.calls != 2 {
		t.Fatalf("repository attempts = %d want 2", racing.calls)
	}
	if racing.rolledBackValue != "2" {
		t.Fatalf("value after generation conflict = %q want rolled-back value 2", racing.rolledBackValue)
	}
	assertSettingGeneration(t, harness.db, harness.ctx, harness.scope.ScopeKey, 3)
	assertSettingOutboxGenerations(t, harness.db, harness.ctx, harness.scope.ScopeKey, []int64{2, 3})
}

func TestServiceMutationBudgetExhaustionIsDependencyFailureWithoutDatabaseMutation(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{rows: map[string]Record{
		generationTestKey: {Key: generationTestKey, Value: "2", ValueType: ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.No},
	}}
	lease, err := harness.store.Acquire(harness.ctx, harness.scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lease.Rollback(context.Background()) })
	service := harness.service(fake)
	current := time.Date(2026, time.September, 17, 0, 0, 0, 0, time.UTC)
	waits := 0
	service.now = func() time.Time { return current }
	service.wait = func(context.Context, time.Duration) error {
		waits++
		current = current.Add(20 * time.Millisecond)
		return nil
	}

	err = service.Update(harness.ctx, generationTestKey, UpdateInput{Value: "5", ValueType: ValueTypeNumber})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeDependencyUnavailable {
		t.Fatalf("budget error = %v want dependency unavailable", err)
	}
	if waits > 25 {
		t.Fatalf("budget waits = %d want at most 25", waits)
	}
	if calls := fake.mutations(); calls != 0 {
		t.Fatalf("database mutations = %d want 0", calls)
	}
}

func TestServiceMutationRedisFailureDoesNotEnterDatabaseMutation(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{rows: map[string]Record{
		generationTestKey: {Key: generationTestKey, Value: "2", ValueType: ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.No},
	}}
	service := harness.service(fake)
	if err := harness.client.Close(); err != nil {
		t.Fatal(err)
	}

	err := service.Update(harness.ctx, generationTestKey, UpdateInput{Value: "5", ValueType: ValueTypeNumber})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeDependencyUnavailable {
		t.Fatalf("redis error = %v want dependency unavailable", err)
	}
	if calls := fake.mutations(); calls != 0 {
		t.Fatalf("database mutations = %d want 0", calls)
	}
}

func TestServiceCacheCorruptStateIsRepairedByAuthority(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{rows: map[string]Record{
		generationTestKey: {Key: generationTestKey, Value: "2", ValueType: ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.No},
	}}
	service := harness.service(fake)
	if err := harness.client.SetString(harness.ctx, cachegeneration.StateKey(harness.scope), "not-json", 0); err != nil {
		t.Fatal(err)
	}

	row, err := service.Find(harness.ctx, generationTestKey)
	if err != nil || row.Value != "2" {
		t.Fatalf("read with corrupt state = %+v err=%v", row, err)
	}
	state, found, err := harness.store.Read(harness.ctx, harness.scope)
	if err != nil || !found || state.State != cachegeneration.StateReady || state.Generation != 1 {
		t.Fatalf("repaired state = %+v found=%v err=%v", state, found, err)
	}
	if calls := fake.findCalls(); calls != 1 {
		t.Fatalf("authority repair reads = %d want 1", calls)
	}
}

func TestServiceCacheScopeFlushRecoversFromAuthority(t *testing.T) {
	harness := openSettingGenerationHarness(t)
	fake := &fakeRepository{rows: map[string]Record{
		generationTestKey: {Key: generationTestKey, Value: "2", ValueType: ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.No},
	}}
	service := harness.service(fake)
	if _, err := service.Find(harness.ctx, generationTestKey); err != nil {
		t.Fatalf("warm read error = %v", err)
	}
	// 只清掉本测试 scope 的 state 与 snapshot，模拟隔离测试 Redis 的 FLUSHDB。
	snapshotKey, err := cachegeneration.SnapshotKey(harness.scope, 1, recordVariant(generationTestKey))
	if err != nil {
		t.Fatal(err)
	}
	if err := harness.client.Delete(harness.ctx, cachegeneration.StateKey(harness.scope)); err != nil {
		t.Fatal(err)
	}
	if err := harness.client.Delete(harness.ctx, snapshotKey); err != nil {
		t.Fatal(err)
	}
	fake.mutex.Lock()
	fake.findCallCount = 0
	fake.mutex.Unlock()

	const workers = 4
	var waitGroup sync.WaitGroup
	failures := make(chan error, workers)
	for index := 0; index < workers; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			if _, err := service.Find(harness.ctx, generationTestKey); err != nil {
				failures <- err
			}
		}()
	}
	waitGroup.Wait()
	close(failures)
	for err := range failures {
		t.Fatalf("read after scope flush failed: %v", err)
	}
	if calls := fake.findCalls(); calls != 1 {
		t.Fatalf("flush recovery leaders = %d want 1", calls)
	}
	state, found, err := harness.store.Read(harness.ctx, harness.scope)
	if err != nil || !found || state.State != cachegeneration.StateReady || state.Generation != 1 {
		t.Fatalf("state after flush recovery = %+v found=%v err=%v", state, found, err)
	}
}

var _ sharedsetting.Reader = (*Service)(nil)

func TestServiceValidateDependenciesRejectsIncompleteStartupWiring(t *testing.T) {
	if err := NewService(nil).ValidateDependencies(); err == nil {
		t.Fatal("ValidateDependencies accepted missing setting dependencies")
	}
	service := NewService(&fakeRepository{})
	service.SetCache(NewCache(&projectredis.Client{}))
	service.SetGenerations(&cachegeneration.Repository{}, &cachegeneration.Store{})
	if err := service.ValidateDependencies(); err == nil {
		t.Fatal("ValidateDependencies accepted a cache without its state store")
	}
}
