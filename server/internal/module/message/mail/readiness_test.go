package mail

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"admin/server/internal/config"
	projectredis "admin/server/internal/redis"
	cachegeneration "admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

type countingReadinessRepository struct {
	configCalls       atomic.Int64
	templateCalls     atomic.Int64
	listTemplateCalls atomic.Int64
	listRuleCalls     atomic.Int64
	mu                sync.RWMutex
	config            Config
	templates         []Template
	rules             []RecipientRule
	delay             time.Duration
	onFindConfig      func(context.Context)
	onFindConfigOnce  sync.Once
}

func (r *countingReadinessRepository) FindConfig(ctx context.Context) (Config, error) {
	r.configCalls.Add(1)
	if err := waitForReadinessTest(ctx, r.delay); err != nil {
		return Config{}, err
	}
	if r.onFindConfig != nil {
		r.onFindConfigOnce.Do(func() { r.onFindConfig(ctx) })
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.config.ID == 0 {
		return Config{}, gorm.ErrRecordNotFound
	}
	return r.config, nil
}

func (r *countingReadinessRepository) FindTemplateByScene(ctx context.Context, scene string) (Template, error) {
	r.templateCalls.Add(1)
	if err := waitForReadinessTest(ctx, r.delay); err != nil {
		return Template{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, template := range r.templates {
		if template.Scene == scene {
			return template, nil
		}
	}
	return Template{}, gorm.ErrRecordNotFound
}

func (r *countingReadinessRepository) ListTemplates(ctx context.Context) ([]Template, error) {
	r.listTemplateCalls.Add(1)
	if err := waitForReadinessTest(ctx, r.delay); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]Template(nil), r.templates...), nil
}

func (r *countingReadinessRepository) ListRecipientRules(ctx context.Context) ([]RecipientRule, error) {
	r.listRuleCalls.Add(1)
	if err := waitForReadinessTest(ctx, r.delay); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]RecipientRule(nil), r.rules...), nil
}

func readyMailRepository(delay time.Duration) *countingReadinessRepository {
	now := time.Now().UTC()
	return &countingReadinessRepository{
		delay: delay,
		config: Config{
			ID: 1, SecretIDCiphertext: "mail:v1:cipher-id", SecretKeyCiphertext: "mail:v1:cipher-key",
			Region: "ap-guangzhou", FromEmail: "sender@example.com", FromName: "Sender",
			TTLMinutes: 5, IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now,
		},
		templates: []Template{{
			ID: 1, Scene: SceneLogin, Name: "Login", Subject: "Code", Content: "{{code}}",
			TencentTemplateID: intPointer(47941), VariableKeys: []byte(`["code"]`),
			ExampleVariables: []byte(`{"code":"123456"}`), IsEnabled: yesno.Yes,
			CreatedAt: now, UpdatedAt: now,
		}},
		rules: []RecipientRule{},
	}
}

func waitForReadinessTest(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func openMailReadinessRedis(t *testing.T) *projectredis.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("Redis integration test")
	}
	if err := godotenv.Load("../../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	client, err := projectredis.Open(context.Background(), settings.RedisURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestMailCacheRuntimeAndReadinessUseOneGenerationWithSeparateVariants(t *testing.T) {
	database, ctx := openMailGenerationDatabase(t)
	client := openMailReadinessRedis(t)
	repository := readyMailRepository(0)
	store := newMailCacheTestStore(t, database, repository, client)

	readiness, err := store.LoadReadiness(ctx, SceneLogin)
	if err != nil || !readiness.Ready || readiness.TTLMinutes != 5 {
		t.Fatalf("readiness = %+v, %v", readiness, err)
	}
	runtime, err := store.Load(ctx, SceneLogin)
	if err != nil || runtime.Generation != 1 || runtime.Config.ID != 1 {
		t.Fatalf("runtime = %+v, %v", runtime, err)
	}
	readinessKey, _ := cachegeneration.SnapshotKey(mailGenerationScope, 1, "readiness:"+SceneLogin)
	runtimeKey, _ := cachegeneration.SnapshotKey(mailGenerationScope, 1, "runtime:"+SceneLogin)
	readinessRaw, readinessFound, readinessErr := client.GetString(ctx, readinessKey)
	runtimeRaw, runtimeFound, runtimeErr := client.GetString(ctx, runtimeKey)
	if readinessErr != nil || runtimeErr != nil || !readinessFound || !runtimeFound {
		t.Fatalf("variant snapshots readiness=%v/%v runtime=%v/%v", readinessFound, readinessErr, runtimeFound, runtimeErr)
	}
	if strings.Contains(readinessRaw, "cipher-id") || !strings.Contains(runtimeRaw, "cipher-id") {
		t.Fatalf("credential boundary readiness=%s runtime=%s", readinessRaw, runtimeRaw)
	}
}

func TestMailReadinessTwoInstancesUseOneColdFillLeaderAndHotHitsAvoidPostgres(t *testing.T) {
	database, ctx := openMailGenerationDatabase(t)
	firstClient := openMailReadinessRedis(t)
	secondClient := openMailReadinessRedis(t)
	repository := readyMailRepository(20 * time.Millisecond)
	firstStore := newMailCacheTestStore(t, database, repository, firstClient)
	secondStore := newRuntimeStore(repository, secondClient)
	secondStore.SetGenerations(cachegeneration.NewRepository(database), cachegeneration.NewStore(secondClient))

	start := make(chan struct{})
	errorsFound := make(chan error, 8)
	var wait sync.WaitGroup
	stores := []*runtimeStore{firstStore, secondStore}
	for index := 0; index < 8; index++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			<-start
			ready, err := stores[worker%len(stores)].LoadReadiness(context.Background(), SceneLogin)
			if err != nil {
				errorsFound <- err
				return
			}
			if !ready.Ready || ready.TTLMinutes != 5 {
				errorsFound <- errors.New("readiness snapshot is false or missing TTL")
			}
		}(index)
	}
	close(start)
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Fatal(err)
	}
	if repository.configCalls.Load() != 1 || repository.templateCalls.Load() != 1 {
		t.Fatalf("cold PostgreSQL reads = config:%d template:%d, want 1 each", repository.configCalls.Load(), repository.templateCalls.Load())
	}

	for index := 0; index < 50; index++ {
		ready, err := stores[index%len(stores)].LoadReadiness(ctx, SceneLogin)
		if err != nil || !ready.Ready {
			t.Fatalf("hot readiness %d = %+v, %v", index, ready, err)
		}
	}
	if repository.configCalls.Load() != 1 || repository.templateCalls.Load() != 1 {
		t.Fatalf("hot hits queried PostgreSQL: config:%d template:%d", repository.configCalls.Load(), repository.templateCalls.Load())
	}
}

func TestMailReadinessCorruptSnapshotIsRebuilt(t *testing.T) {
	database, ctx := openMailGenerationDatabase(t)
	client := openMailReadinessRedis(t)
	repository := readyMailRepository(0)
	store := newMailCacheTestStore(t, database, repository, client)
	key, _ := cachegeneration.SnapshotKey(mailGenerationScope, 1, "readiness:"+SceneLogin)
	if err := client.SetString(ctx, key, `{"schemaVersion":1,"generation":1,"ready":true,"ttlMinutes":5,"extra":true}`, time.Minute); err != nil {
		t.Fatal(err)
	}

	ready, err := store.LoadReadiness(ctx, SceneLogin)
	if err != nil || !ready.Ready || ready.TTLMinutes != 5 {
		t.Fatalf("rebuilt readiness = %+v, %v", ready, err)
	}
	if repository.configCalls.Load() != 1 || repository.templateCalls.Load() != 1 {
		t.Fatalf("rebuild reads = config:%d template:%d, want 1 each", repository.configCalls.Load(), repository.templateCalls.Load())
	}
	raw, found, err := client.GetString(ctx, key)
	if err != nil || !found || strings.Contains(raw, `"extra"`) {
		t.Fatalf("rebuilt snapshot found=%v err=%v raw=%s", found, err, raw)
	}
}

func TestMailReadinessRedisFailureFailsClosedWithoutPostgres(t *testing.T) {
	database, ctx := openMailGenerationDatabase(t)
	client := openMailReadinessRedis(t)
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	repository := readyMailRepository(0)
	store := newRuntimeStore(repository, client)
	store.SetGenerations(cachegeneration.NewRepository(database), cachegeneration.NewStore(client))

	if _, err := store.LoadReadiness(ctx, SceneLogin); err == nil {
		t.Fatal("closed Redis client was accepted")
	}
	if repository.configCalls.Load() != 0 || repository.templateCalls.Load() != 0 {
		t.Fatalf("Redis failure fell back to PostgreSQL: config:%d template:%d", repository.configCalls.Load(), repository.templateCalls.Load())
	}
}

func TestMailReadinessDropsOldFillWhenGenerationAdvances(t *testing.T) {
	database, ctx := openMailGenerationDatabase(t)
	client := openMailReadinessRedis(t)
	repository := readyMailRepository(0)
	store := newMailCacheTestStore(t, database, repository, client)
	repository.onFindConfig = func(hookContext context.Context) {
		result, err := advanceMailGeneration(hookContext, database, 1)
		if err != nil {
			t.Errorf("advance generation: %v", err)
			return
		}
		if _, err := store.states.Reconcile(hookContext, mailGenerationScope, result.Generation); err != nil {
			t.Errorf("publish advanced generation: %v", err)
		}
	}

	ready, err := store.LoadReadiness(ctx, SceneLogin)
	if err != nil || !ready.Ready {
		t.Fatalf("readiness after generation advance = %+v, %v", ready, err)
	}
	oldKey, _ := cachegeneration.SnapshotKey(mailGenerationScope, 1, "readiness:"+SceneLogin)
	if _, found, err := client.GetString(ctx, oldKey); err != nil || found {
		t.Fatalf("old generation snapshot found=%v err=%v", found, err)
	}
	newKey, _ := cachegeneration.SnapshotKey(mailGenerationScope, 2, "readiness:"+SceneLogin)
	if _, found, err := client.GetString(ctx, newKey); err != nil || !found {
		t.Fatalf("new generation snapshot found=%v err=%v", found, err)
	}
}

func TestVerifyCodeReadinessSnapshotCodecIsStrict(t *testing.T) {
	raw, err := encodeVerifyCodeReadinessSnapshot(newVerifyCodeReadinessSnapshot(2, VerifyCodeReadiness{Ready: true, TTLMinutes: 5}))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := decodeVerifyCodeReadinessSnapshot(raw)
	if err != nil || snapshot.Generation != 2 || !snapshot.Ready || snapshot.TTLMinutes != 5 {
		t.Fatalf("decoded snapshot = %+v, %v", snapshot, err)
	}
	for name, payload := range map[string]string{
		"empty":            `{}`,
		"unknown field":    strings.Replace(raw, `{`, `{"extra":true,`, 1),
		"duplicate field":  strings.Replace(raw, `"ready":true`, `"ready":true,"ready":false`, 1),
		"zero generation":  strings.Replace(raw, `"generation":2`, `"generation":0`, 1),
		"invalid TTL":      strings.Replace(raw, `"ttlMinutes":5`, `"ttlMinutes":61`, 1),
		"unready with TTL": `{"schemaVersion":1,"generation":2,"ready":false,"ttlMinutes":5}`,
		"trailing value":   raw + `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeVerifyCodeReadinessSnapshot(payload); !errors.Is(err, ErrReadinessSnapshotCorrupt) {
				t.Fatalf("decode error = %v, want ErrReadinessSnapshotCorrupt", err)
			}
		})
	}
}

func TestMailReadinessRejectsMissingDependencies(t *testing.T) {
	if _, err := newRuntimeStore(nil, nil).LoadReadiness(context.Background(), SceneLogin); err == nil {
		t.Fatal("readiness accepted missing dependencies")
	}
}

func openMailGenerationDatabase(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	database, ctx := openMailRepositoryDatabase(t)
	if err := database.WithContext(ctx).Exec(`
CREATE TABLE system_config_cache_generation(
 namespace varchar(128) NOT NULL, scope_key varchar(128) NOT NULL, generation bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(namespace,scope_key));
CREATE TABLE system_config_cache_outbox(
 id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
 namespace varchar(128) NOT NULL, scope_key varchar(128) NOT NULL, generation bigint NOT NULL,
 attempts integer NOT NULL DEFAULT 0, available_at timestamptz NOT NULL DEFAULT now(),
 locked_until timestamptz, lock_token varchar(64), last_error varchar(512) NOT NULL DEFAULT '',
 published_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(namespace,scope_key,generation),
 FOREIGN KEY(namespace,scope_key) REFERENCES system_config_cache_generation(namespace,scope_key));
INSERT INTO system_config_cache_generation(namespace,scope_key,generation) VALUES ('message.mail','global',1);`).Error; err != nil {
		t.Fatal(err)
	}
	return database, ctx
}

func newMailCacheTestStore(t *testing.T, database *gorm.DB, repository runtimeRepository, client *projectredis.Client) *runtimeStore {
	t.Helper()
	keys := mailCacheTestKeys(t)
	if err := client.DeleteMany(context.Background(), keys); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), keys) })
	store := newRuntimeStore(repository, client)
	store.SetGenerations(cachegeneration.NewRepository(database), cachegeneration.NewStore(client))
	generation, err := store.generations.Current(context.Background(), mailGenerationScope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.states.Reconcile(context.Background(), mailGenerationScope, generation); err != nil {
		t.Fatal(err)
	}
	return store
}

func mailCacheTestKeys(t *testing.T) []string {
	t.Helper()
	keys := []string{cachegeneration.StateKey(mailGenerationScope)}
	for generation := int64(1); generation <= 8; generation++ {
		for _, scene := range []string{SceneLogin, SceneForget, SceneBindEmail, SceneChangePassword} {
			for _, variant := range []string{"runtime:" + scene, "readiness:" + scene} {
				snapshot, err := cachegeneration.SnapshotKey(mailGenerationScope, generation, variant)
				if err != nil {
					t.Fatal(err)
				}
				keys = append(keys, snapshot)
			}
		}
	}
	return keys
}
