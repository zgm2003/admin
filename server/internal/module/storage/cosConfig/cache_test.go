package cosconfig

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"admin/server/internal/database/testquery"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	cachegeneration "admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

func TestRuntimeSnapshotCodecRoundTripAndRejections(t *testing.T) {
	endpoint := "https://cos-internal.example.com"
	domain := "https://cdn.example.com"
	config := RuntimeConfig{
		ID: 7, AppID: "1250000000", SecretIDCiphertext: "v1:cipher-id", SecretKeyCiphertext: "v1:cipher-key",
		CurrentVersion: 2, IsEnabled: yesno.Yes, Deleted: false,
		Versions: []RuntimeVersion{
			{Version: 1, Bucket: "bucket-a", Region: "ap-guangzhou", Endpoint: &endpoint, BucketDomain: &domain},
			{Version: 2, Bucket: "bucket-b", Region: "ap-shanghai"},
		},
	}
	raw, err := encodeRuntimePayload(3, 7, config)
	if err != nil {
		t.Fatal(err)
	}
	// 快照只允许出现密文，不能出现任何 Secret 明文。
	if !strings.Contains(raw, "v1:cipher-id") || strings.Contains(raw, "secret-id") || strings.Contains(raw, "secret-key") {
		t.Fatalf("runtime payload leaked plaintext: %s", raw)
	}
	decoded, err := decodeRuntimePayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	roundTrip := decoded.runtime()
	if roundTrip.ID != config.ID || roundTrip.CurrentVersion != 2 || len(roundTrip.Versions) != 2 ||
		roundTrip.Versions[1].Bucket != "bucket-b" || roundTrip.Versions[0].Endpoint == nil || *roundTrip.Versions[0].Endpoint != endpoint {
		t.Fatalf("round trip = %+v", roundTrip)
	}

	rejections := []struct {
		name    string
		payload string
	}{
		{"unknown field", strings.Replace(raw, `{`, `{"extra":1,`, 1)},
		{"duplicate field", strings.Replace(raw, `"schemaVersion":1`, `"schemaVersion":1,"schemaVersion":1`, 1)},
		{"trailing data", raw + "{}"},
		{"not an object", `[]`},
		{"schema version", strings.Replace(raw, `"schemaVersion":1`, `"schemaVersion":2`, 1)},
		{"empty ciphertext", strings.Replace(raw, `"secretIdCiphertext":"v1:cipher-id"`, `"secretIdCiphertext":""`, 1)},
		{"invalid enabled flag", strings.Replace(raw, `"isEnabled":1`, `"isEnabled":9`, 1)},
		{"empty versions", strings.Replace(raw, `"versions":[`, `"versions":[],"ignored":[`, 1)},
		{"out of order versions", strings.Replace(raw, `"version":2,"bucket":"bucket-b"`, `"version":3,"bucket":"bucket-b"`, 1)},
		{"empty bucket", strings.Replace(raw, `"bucket":"bucket-a"`, `"bucket":""`, 1)},
		{"current version mismatch", strings.Replace(raw, `"currentVersion":2`, `"currentVersion":3`, 1)},
	}
	for _, test := range rejections {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeRuntimePayload(test.payload); !errors.Is(err, ErrSnapshotCorrupt) {
				t.Fatalf("decode error = %v want ErrSnapshotCorrupt", err)
			}
		})
	}
	if _, err := encodeRuntimePayload(0, 7, config); err == nil {
		t.Fatal("encoder accepted generation 0")
	}
	broken := config
	broken.Versions = []RuntimeVersion{{Version: 2, Bucket: "bucket-b", Region: "ap-shanghai"}}
	if _, err := encodeRuntimePayload(1, 7, broken); err == nil {
		t.Fatal("encoder accepted versions that do not start at 1")
	}
}

// TestRuntimeReadRepairsMissingStateFromPostgres：state 丢失时由单 leader 读 PostgreSQL 权威代际并重建 state 与快照。
func TestRuntimeReadRepairsMissingStateFromPostgres(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	service := newTestService(t, db, mustKeys(t, "r"), &recordingConnectionTester{})
	id := createRuntimeConfig(t, ctx, service, "assets")

	client := testRedisClient(t)
	stateKey := "config-cache:state:v1:storage.cosconfig:" + decimalKey(id)
	if err := client.DeleteMany(ctx, []string{stateKey}); err != nil {
		t.Fatal(err)
	}
	runtime, err := service.Runtime(ctx, id)
	if err != nil {
		t.Fatalf("runtime error = %s", appCause(err))
	}
	if runtime.ID != id || runtime.CurrentVersion != 1 || len(runtime.Versions) != 1 || runtime.Versions[0].Bucket != "assets" {
		t.Fatalf("runtime = %+v", runtime)
	}
	state, found, err := service.states.Read(ctx, mustConfigScope(t, id))
	if err != nil || !found || state.State != cachegeneration.StateReady || state.Generation != 1 {
		t.Fatalf("repaired state = %+v found=%v err=%v", state, found, err)
	}
}

func TestRuntimeColdAndHotPostgresSelectBudgetsAcrossInstances(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	counter := testquery.New(db.Logger, "storage_cos_config", "storage_cos_config_version")
	db = db.Session(&gorm.Session{Logger: counter})
	first := newTestService(t, db, mustKeys(t, "q"), &recordingConnectionTester{})
	id := createRuntimeConfig(t, ctx, first, "assets")

	secondClient := testRedisClient(t)
	secondStates := cachegeneration.NewStore(secondClient)
	secondCache := NewCache(secondClient)
	secondCache.SetStateStore(secondStates)
	secondRepository := NewRepository(db)
	secondRepository.SetGenerations(cachegeneration.NewRepository(db))
	second := NewService(secondRepository, mustKeys(t, "q"), &recordingConnectionTester{})
	second.SetGenerations(cachegeneration.NewRepository(db), secondStates)
	second.SetCache(secondCache)
	second.readBudget = 10 * time.Second

	counter.Reset()
	start := make(chan struct{})
	errorsFound := make(chan error, 2)
	var wait sync.WaitGroup
	for _, service := range []*Service{first, second} {
		wait.Add(1)
		go func(service *Service) {
			defer wait.Done()
			<-start
			if _, err := service.Runtime(ctx, id); err != nil {
				errorsFound <- err
			}
		}(service)
	}
	close(start)
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Fatal(err)
	}
	if logical, versions, total := counter.Count("storage_cos_config"), counter.Count("storage_cos_config_version"), counter.Total(); logical != 1 || versions != 1 || total != 2 {
		t.Fatalf("cold COS SELECT budget: config=%d versions=%d total=%d, want 1/1/2", logical, versions, total)
	}

	counter.Reset()
	if _, err := second.Runtime(ctx, id); err != nil {
		t.Fatal(err)
	}
	if got := counter.Total(); got != 0 {
		t.Fatalf("hot COS runtime SELECT budget = %d, want 0", got)
	}
}

// TestRuntimeReadRebuildsCorruptSnapshot：快照损坏时单 leader 重建，不把损坏内容返回给调用方。
func TestRuntimeReadRebuildsCorruptSnapshot(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	service := newTestService(t, db, mustKeys(t, "c"), &recordingConnectionTester{})
	id := createRuntimeConfig(t, ctx, service, "assets")
	scope := mustConfigScope(t, id)
	snapshotKey, err := cachegeneration.SnapshotKey(scope, 1, runtimeSnapshotVariant)
	if err != nil {
		t.Fatal(err)
	}
	client := testRedisClient(t)
	if err := client.SetString(ctx, snapshotKey, `{"schemaVersion":1,"generation":1}`, time.Minute); err != nil {
		t.Fatal(err)
	}
	runtime, err := service.Runtime(ctx, id)
	if err != nil {
		t.Fatalf("runtime error = %s", appCause(err))
	}
	if runtime.Versions[0].Bucket != "assets" {
		t.Fatalf("runtime = %+v", runtime)
	}
	raw, found, err := client.GetString(ctx, snapshotKey)
	if err != nil || !found || !strings.Contains(raw, `"generation":1`) {
		t.Fatalf("snapshot not rebuilt: found=%v err=%v raw=%s", found, err, raw)
	}
}

// TestRuntimeInvalidatingExhaustsBudgetWithoutPostgres：活跃 invalidating 期间有界等待并失败闭合，绝不回源 PostgreSQL。
func TestRuntimeInvalidatingExhaustsBudgetWithoutPostgres(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	repository := &fakeConfigRepository{}
	client := testRedisClient(t)
	generations := cachegeneration.NewRepository(db)
	states := cachegeneration.NewStore(client)
	cache := NewCache(client)
	cache.SetStateStore(states)
	service := NewService(repository, mustKeys(t, "i"), &recordingConnectionTester{})
	service.SetGenerations(generations, states)
	service.SetCache(cache)
	scope, err := cachegeneration.NewScope(cosConfigGenerationNamespace, "5")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := generations.InitializeTx(ctx, db, scope, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.states.Reconcile(ctx, scope, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := service.states.Acquire(ctx, scope, 1); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(0, 0)
	service.now = func() time.Time { return now }
	waits := 0
	service.wait = func(context.Context, time.Duration) error {
		waits++
		now = now.Add(20 * time.Millisecond)
		return nil
	}
	_, err = service.Runtime(ctx, 5)
	if appCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("runtime error = %s want dependency unavailable", appCause(err))
	}
	if waits == 0 || waits > 26 {
		t.Fatalf("waits = %d want bounded 1..26", waits)
	}
	if repository.runtimeFactsCalls != 0 {
		t.Fatalf("runtime read touched PostgreSQL %d times during invalidating", repository.runtimeFactsCalls)
	}
}

// TestRuntimeRedisFailureFailsClosed：Redis I/O 错误直接失败闭合，不用 PostgreSQL 充当每请求兜底。
func TestRuntimeRedisFailureFailsClosed(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	repository := &fakeConfigRepository{}
	broken := closedRedisClient(t)
	service := NewService(repository, mustKeys(t, "f"), &recordingConnectionTester{})
	service.SetGenerations(cachegeneration.NewRepository(db), cachegeneration.NewStore(broken))
	service.SetCache(NewCache(broken))
	_, err := service.Runtime(ctx, 6)
	if appCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("runtime error = %s want dependency unavailable", appCause(err))
	}
	if repository.runtimeFactsCalls != 0 {
		t.Fatalf("runtime read touched PostgreSQL %d times on Redis failure", repository.runtimeFactsCalls)
	}
}

// TestMutationNoOpRestoresReadyWithoutAdvancingGeneration：no-op 不推进 generation，并恢复原 ready。
func TestMutationNoOpRestoresReadyWithoutAdvancingGeneration(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	repository := &fakeConfigRepository{}
	service := newFakeService(t, db, repository)
	scope, err := cachegeneration.NewScope(cosConfigGenerationNamespace, "9")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cachegeneration.NewRepository(db).InitializeTx(ctx, db, scope, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.states.Reconcile(ctx, scope, 1); err != nil {
		t.Fatal(err)
	}
	repository.findByIDRow = Current{Model: Model{ID: 9, IsEnabled: yesno.Yes, CurrentVersion: 1}}
	if err := service.Update(ctx, 9, UpdateInput{Name: "Main", Bucket: "assets", Region: "ap-guangzhou"}); err != nil {
		t.Fatal(err)
	}
	if repository.updateCalls != 1 {
		t.Fatalf("update calls = %d want 1", repository.updateCalls)
	}
	generation, err := cachegeneration.NewRepository(db).Current(ctx, scope)
	if err != nil || generation != 1 {
		t.Fatalf("generation = %d,%v want 1", generation, err)
	}
	state, found, err := service.states.Read(ctx, scope)
	if err != nil || !found || state.State != cachegeneration.StateReady || state.Generation != 1 {
		t.Fatalf("state after no-op = %+v found=%v err=%v", state, found, err)
	}
}

// TestMutationPublishFailureStillReportsBusinessSuccess：PostgreSQL 已提交但 Redis 发布失败时接口返回成功，outbox 保持 pending。
func TestMutationPublishFailureStillReportsBusinessSuccess(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	client := testRedisClient(t)
	scopeKey := "11"
	scope, err := cachegeneration.NewScope(cosConfigGenerationNamespace, scopeKey)
	if err != nil {
		t.Fatal(err)
	}
	generations := cachegeneration.NewRepository(db)
	if _, err := generations.InitializeTx(ctx, db, scope, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	repository := &fakeConfigRepository{findByIDRow: Current{Model: Model{ID: 11, IsEnabled: yesno.Yes, CurrentVersion: 1}}}
	service := NewService(repository, mustKeys(t, "p"), &recordingConnectionTester{})
	service.SetGenerations(generations, cachegeneration.NewStore(client))
	service.SetCache(NewCache(client))
	if _, err := service.states.Reconcile(ctx, scope, 1); err != nil {
		t.Fatal(err)
	}
	// 业务事务已提交：写入本代 outbox（pending），随后注入发布失败。
	if err := db.WithContext(ctx).Exec(`INSERT INTO system_config_cache_outbox (namespace, scope_key, generation, attempts, available_at, created_at, updated_at)
		VALUES (?, ?, 2, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, cosConfigGenerationNamespace, scopeKey).Error; err != nil {
		t.Fatal(err)
	}
	repository.onUpdate = func() {
		_ = client.DeleteMany(context.Background(), []string{"config-cache:state:v1:storage.cosconfig:" + scopeKey})
	}
	repository.updateResult = cachegeneration.MutationResult{Changed: true, Generation: 2, OutboxID: 1}
	if err := service.Update(ctx, 11, UpdateInput{Name: "Main", Bucket: "assets", Region: "ap-guangzhou"}); err != nil {
		t.Fatalf("mutation returned failure despite committed PostgreSQL write: %v", err)
	}
	var pending int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM system_config_cache_outbox WHERE namespace = ? AND scope_key = ? AND published_at IS NULL`, cosConfigGenerationNamespace, scopeKey).Scan(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if pending == 0 {
		t.Fatal("outbox row must stay pending for the relay after publish failure")
	}
}

// TestMutationUncertainCommitUsesAuthoritativeGeneration：提交结果不确定时按权威 generation 处理，不恢复 stale ready。
func TestMutationUncertainCommitUsesAuthoritativeGeneration(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	client := testRedisClient(t)
	scopeKey := "12"
	scope, err := cachegeneration.NewScope(cosConfigGenerationNamespace, scopeKey)
	if err != nil {
		t.Fatal(err)
	}
	generations := cachegeneration.NewRepository(db)
	if _, err := generations.InitializeTx(ctx, db, scope, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	repository := &fakeConfigRepository{findByIDRow: Current{Model: Model{ID: 12, IsEnabled: yesno.Yes, CurrentVersion: 1}}}
	service := NewService(repository, mustKeys(t, "u"), &recordingConnectionTester{})
	service.SetGenerations(generations, cachegeneration.NewStore(client))
	service.SetCache(NewCache(client))
	if _, err := service.states.Reconcile(ctx, scope, 1); err != nil {
		t.Fatal(err)
	}
	// 模拟"另一个写入者已提交 + 本事务结果不确定"：事务报错，但权威 generation 已前进到 3。
	repository.updateErr = errors.New("connection reset after commit")
	repository.onUpdate = func() {
		if _, err := generations.AdvanceTx(context.Background(), db, scope, 1, time.Now().UTC()); err != nil {
			t.Errorf("advance authoritative generation: %v", err)
		}
	}
	if err := service.Update(ctx, 12, UpdateInput{Name: "Main", Bucket: "assets", Region: "ap-guangzhou"}); err == nil {
		t.Fatal("uncertain mutation should surface the business error")
	}
	state, found, err := service.states.Read(ctx, scope)
	if err != nil || !found || state.State != cachegeneration.StateReady || state.Generation != 2 {
		t.Fatalf("state = %+v found=%v err=%v want ready at authoritative generation", state, found, err)
	}
}

// TestMutationBudgetExhaustionReturnsDependencyBusy：内部 generation 竞争在 500ms/20ms 预算内重试，耗尽后返回依赖繁忙（非 409）。
func TestMutationBudgetExhaustionReturnsDependencyBusy(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	repository := &fakeConfigRepository{
		findByIDRow: Current{Model: Model{ID: 13, IsEnabled: yesno.Yes, CurrentVersion: 1}},
		updateErr:   cachegeneration.ErrGenerationChanged,
	}
	service := newFakeService(t, db, repository)
	scope, err := cachegeneration.NewScope(cosConfigGenerationNamespace, "13")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cachegeneration.NewRepository(db).InitializeTx(ctx, db, scope, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.states.Reconcile(ctx, scope, 1); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(0, 0)
	service.now = func() time.Time { return now }
	service.wait = func(context.Context, time.Duration) error {
		now = now.Add(20 * time.Millisecond)
		return nil
	}
	err = service.Update(ctx, 13, UpdateInput{Name: "Main", Bucket: "assets", Region: "ap-guangzhou"})
	if appCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("budget exhaustion error = %v want dependency unavailable (never conflict)", err)
	}
	if repository.updateCalls == 0 || repository.updateCalls > 26 {
		t.Fatalf("update attempts = %d want bounded 1..26", repository.updateCalls)
	}
}

// TestMutationRedisErrorDoesNotWritePostgres：Redis I/O 错误直接失败闭合，不绕过 lease 写 PostgreSQL。
func TestMutationRedisErrorDoesNotWritePostgres(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	broken := closedRedisClient(t)
	repository := &fakeConfigRepository{findByIDRow: Current{Model: Model{ID: 14, IsEnabled: yesno.Yes, CurrentVersion: 1}}}
	service := NewService(repository, mustKeys(t, "w"), &recordingConnectionTester{})
	service.SetGenerations(cachegeneration.NewRepository(db), cachegeneration.NewStore(broken))
	service.SetCache(NewCache(broken))
	if err := service.Update(ctx, 14, UpdateInput{Name: "Main", Bucket: "assets", Region: "ap-guangzhou"}); appCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("redis failure error = %s want dependency unavailable", appCause(err))
	}
	if repository.updateCalls != 0 {
		t.Fatalf("update calls = %d want 0 when Redis is unavailable", repository.updateCalls)
	}
}

// closedRedisClient 返回一个已关闭连接池的客户端：Open 成功但所有命令失败，用于模拟 Redis I/O 故障。
func closedRedisClient(t *testing.T) *projectredis.Client {
	t.Helper()
	client := testRedisClient(t)
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	return client
}

// appCause 暴露 apperror 的底层原因，避免测试只看到 "error.dependencyUnavailable"。
func appCause(err error) string {
	var appErr *apperror.Error
	if errors.As(err, &appErr) && appErr.Cause != nil {
		return appErr.Cause.Error()
	}
	return err.Error()
}

func newFakeService(t *testing.T, db *gorm.DB, repository repository) *Service {
	t.Helper()
	client := testRedisClient(t)
	states := cachegeneration.NewStore(client)
	cache := NewCache(client)
	cache.SetStateStore(states)
	service := NewService(repository, mustKeys(t, "s"), &recordingConnectionTester{})
	service.SetGenerations(cachegeneration.NewRepository(db), states)
	service.SetCache(cache)
	service.readBudget = 10 * time.Second
	return service
}

func createRuntimeConfig(t *testing.T, ctx context.Context, service *Service, bucket string) int64 {
	t.Helper()
	id, err := service.Create(ctx, CreateInput{Name: "Main", AppID: "1250000000", SecretID: "secret-id", SecretKey: "secret-key", Bucket: bucket, Region: "ap-guangzhou", IsEnabled: yesno.Yes})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustKeys(t *testing.T, seed string) *secretkey.KeyRing {
	t.Helper()
	keys, err := secretkey.New(strings.Repeat(seed, 64))
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

// fakeConfigRepository 提供可控的持久化事实与故障注入点。
type fakeConfigRepository struct {
	mutex             sync.Mutex
	findByIDRow       Current
	findByIDErr       error
	createResult      cachegeneration.MutationResult
	updateResult      cachegeneration.MutationResult
	updateErr         error
	onUpdate          func()
	updateCalls       int
	runtimeFacts      RuntimeConfig
	runtimeFactsErr   error
	runtimeFactsCalls int
}

func (f *fakeConfigRepository) Count(context.Context, ListQuery) (int64, error) { return 0, nil }
func (f *fakeConfigRepository) List(context.Context, ListQuery) ([]Current, error) {
	return nil, nil
}
func (f *fakeConfigRepository) FindByID(context.Context, int64) (Current, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.findByIDRow, f.findByIDErr
}
func (f *fakeConfigRepository) Create(context.Context, *Model, Version) (cachegeneration.MutationResult, error) {
	return f.createResult, nil
}
func (f *fakeConfigRepository) Update(context.Context, int64, UpdateValues, int64, time.Time) (cachegeneration.MutationResult, error) {
	f.mutex.Lock()
	f.updateCalls++
	onUpdate := f.onUpdate
	err := f.updateErr
	result := f.updateResult
	f.mutex.Unlock()
	if onUpdate != nil {
		onUpdate()
	}
	return result, err
}
func (f *fakeConfigRepository) UpdateStatus(context.Context, int64, yesno.Value, int64, time.Time) (cachegeneration.MutationResult, error) {
	return cachegeneration.MutationResult{}, nil
}
func (f *fakeConfigRepository) MarkDeleted(context.Context, int64, int64, time.Time) (cachegeneration.MutationResult, error) {
	return cachegeneration.MutationResult{}, nil
}
func (f *fakeConfigRepository) CountRuleReferences(context.Context, int64) (int64, error) {
	return 0, nil
}
func (f *fakeConfigRepository) RuntimeFacts(context.Context, int64) (RuntimeConfig, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.runtimeFactsCalls++
	return f.runtimeFacts, f.runtimeFactsErr
}
