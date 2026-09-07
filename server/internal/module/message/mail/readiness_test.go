package mail

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"admin/server/internal/config"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

type countingReadinessRepository struct {
	configCalls   atomic.Int64
	templateCalls atomic.Int64
	mu            sync.RWMutex
	config        Config
	template      Template
	delay         time.Duration
}

func (r *countingReadinessRepository) FindConfig(ctx context.Context, _ int64) (Config, error) {
	r.configCalls.Add(1)
	if err := waitForReadinessTest(ctx, r.delay); err != nil {
		return Config{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.config.ID == 0 {
		return Config{}, gorm.ErrRecordNotFound
	}
	return r.config, nil
}

func (r *countingReadinessRepository) FindTemplateByScene(ctx context.Context, _ int64, _ string) (Template, error) {
	r.templateCalls.Add(1)
	if err := waitForReadinessTest(ctx, r.delay); err != nil {
		return Template{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.template.ID == 0 {
		return Template{}, gorm.ErrRecordNotFound
	}
	return r.template, nil
}

func (r *countingReadinessRepository) setConfigEnabled(enabled yesno.Value) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config.IsEnabled = enabled
}

func readyMailRepository(delay time.Duration) *countingReadinessRepository {
	return &countingReadinessRepository{
		delay: delay,
		config: Config{
			ID: 1, SecretIDCiphertext: "configured", SecretKeyCiphertext: "configured", IsEnabled: yesno.Yes,
		},
		template: Template{ID: 1, Scene: SceneLogin, IsEnabled: yesno.Yes},
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

func TestVerifyCodeReadinessTwoInstancesRecoverMissingSnapshotOnce(t *testing.T) {
	firstClient := openMailReadinessRedis(t)
	secondClient := openMailReadinessRedis(t)
	repository := readyMailRepository(100 * time.Millisecond)
	firstStore := NewVerifyCodeReadinessStore(repository, firstClient)
	secondStore := NewVerifyCodeReadinessStore(repository, secondClient)
	platformID := time.Now().UnixNano()
	keys := []string{verifyCodeReadinessKey(platformID, SceneLogin), verifyCodeReadinessLoadLockKey(platformID, SceneLogin)}
	if err := firstClient.DeleteMany(context.Background(), keys); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = firstClient.DeleteMany(context.Background(), keys) })

	start := make(chan struct{})
	errorsFound := make(chan error, 64)
	var wait sync.WaitGroup
	stores := []*verifyCodeReadinessStore{firstStore, secondStore}
	for index := 0; index < 64; index++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			<-start
			ready, err := stores[worker%len(stores)].Current(context.Background(), platformID, SceneLogin)
			if err != nil {
				errorsFound <- err
				return
			}
			if !ready {
				errorsFound <- errors.New("readiness snapshot is false")
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
		t.Fatalf("PostgreSQL readiness reads = config:%d template:%d, want 1 each", repository.configCalls.Load(), repository.templateCalls.Load())
	}

	for index := 0; index < 100; index++ {
		ready, err := stores[index%len(stores)].Current(context.Background(), platformID, SceneLogin)
		if err != nil || !ready {
			t.Fatalf("ready hit %d = %v, %v", index, ready, err)
		}
	}
	if repository.configCalls.Load() != 1 || repository.templateCalls.Load() != 1 {
		t.Fatalf("ready hits queried PostgreSQL: config:%d template:%d", repository.configCalls.Load(), repository.templateCalls.Load())
	}
}

func TestVerifyCodeReadinessRejectsMissingDependencies(t *testing.T) {
	if _, err := NewVerifyCodeReadinessStore(nil, nil).Current(context.Background(), 1, SceneLogin); err == nil {
		t.Fatal("Current accepted missing Redis and repository dependencies")
	}
	client := openMailReadinessRedis(t)
	store := NewVerifyCodeReadinessStore(nil, client)
	platformID := time.Now().UnixNano()
	keys := []string{verifyCodeReadinessKey(platformID, SceneLogin), verifyCodeReadinessLoadLockKey(platformID, SceneLogin)}
	if err := client.DeleteMany(context.Background(), keys); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), keys) })
	if _, err := store.Current(context.Background(), platformID, SceneLogin); err == nil {
		t.Fatal("Current accepted a missing repository")
	}
	mutation := VerifyCodeReadinessMutation{
		platformID: platformID, scene: SceneLogin, priorPayload: "prior", invalidatingPayload: "invalidating",
	}
	if err := store.PublishMutation(context.Background(), mutation); err == nil {
		t.Fatal("PublishMutation accepted a missing repository")
	}
}

func TestVerifyCodeReadinessSnapshotRejectsMalformedPayloads(t *testing.T) {
	for _, raw := range []string{
		`{}`,
		`{"schemaVersion":1,"state":"ready"}`,
		`{"schemaVersion":1,"state":"ready","ready":true,"extra":true}`,
		`{"schemaVersion":1,"state":"invalidating","ready":false,"mutationToken":"token"}`,
		`{"schemaVersion":1,"state":"invalidating","mutationToken":""}`,
		`{"schemaVersion":1,"state":"ready","ready":true}{"schemaVersion":1,"state":"ready","ready":false}`,
	} {
		if _, err := decodeVerifyCodeReadinessSnapshot(raw); err == nil {
			t.Fatalf("accepted malformed readiness snapshot: %s", raw)
		}
	}
}

func TestVerifyCodeReadinessFailsClosedWithoutPostgresFallback(t *testing.T) {
	client := openMailReadinessRedis(t)
	repository := readyMailRepository(0)
	store := NewVerifyCodeReadinessStore(repository, client)
	platformID := time.Now().UnixNano()
	key := verifyCodeReadinessKey(platformID, SceneLogin)
	t.Cleanup(func() { _ = client.Delete(context.Background(), key) })

	if err := client.SetString(context.Background(), key, "not-json", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Current(context.Background(), platformID, SceneLogin); err == nil {
		t.Fatal("corrupt readiness snapshot was accepted")
	}
	if repository.configCalls.Load() != 0 || repository.templateCalls.Load() != 0 {
		t.Fatal("corrupt readiness snapshot fell back to PostgreSQL")
	}

	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Current(context.Background(), platformID, SceneLogin); err == nil {
		t.Fatal("closed Redis client was accepted")
	}
	if repository.configCalls.Load() != 0 || repository.templateCalls.Load() != 0 {
		t.Fatal("Redis failure fell back to PostgreSQL")
	}
}

func TestVerifyCodeReadinessMutationBlocksOldSnapshotAcrossInstances(t *testing.T) {
	firstClient := openMailReadinessRedis(t)
	secondClient := openMailReadinessRedis(t)
	repository := readyMailRepository(0)
	firstStore := NewVerifyCodeReadinessStore(repository, firstClient)
	secondStore := NewVerifyCodeReadinessStore(repository, secondClient)
	platformID := time.Now().UnixNano()
	keys := []string{verifyCodeReadinessKey(platformID, SceneLogin), verifyCodeReadinessLoadLockKey(platformID, SceneLogin)}
	if err := firstClient.DeleteMany(context.Background(), keys); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = firstClient.DeleteMany(context.Background(), keys) })

	if ready, err := firstStore.Current(context.Background(), platformID, SceneLogin); err != nil || !ready {
		t.Fatalf("initial readiness = %v, %v", ready, err)
	}
	mutation, err := firstStore.BeginMutation(context.Background(), platformID, SceneLogin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := secondStore.Current(context.Background(), platformID, SceneLogin); err == nil {
		t.Fatal("second instance returned an old ready snapshot during mutation")
	}

	repository.setConfigEnabled(yesno.No)
	if err := firstStore.PublishMutation(context.Background(), mutation); err != nil {
		t.Fatal(err)
	}
	if ready, err := secondStore.Current(context.Background(), platformID, SceneLogin); err != nil || ready {
		t.Fatalf("published disabled readiness = %v, %v", ready, err)
	}
}

func TestVerifyCodeReadinessRollbackRequiresMutationOwner(t *testing.T) {
	client := openMailReadinessRedis(t)
	store := NewVerifyCodeReadinessStore(readyMailRepository(0), client)
	platformID := time.Now().UnixNano()
	keys := []string{verifyCodeReadinessKey(platformID, SceneLogin), verifyCodeReadinessLoadLockKey(platformID, SceneLogin)}
	if err := client.DeleteMany(context.Background(), keys); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), keys) })
	if _, err := store.Current(context.Background(), platformID, SceneLogin); err != nil {
		t.Fatal(err)
	}
	mutation, err := store.BeginMutation(context.Background(), platformID, SceneLogin)
	if err != nil {
		t.Fatal(err)
	}
	stale := mutation
	stale.invalidatingPayload = fmt.Sprintf("%s-stale", stale.invalidatingPayload)
	if err := store.RollbackMutation(context.Background(), stale); err == nil {
		t.Fatal("rollback accepted a stale mutation owner")
	}
	if err := store.RollbackMutation(context.Background(), mutation); err != nil {
		t.Fatal(err)
	}
}
