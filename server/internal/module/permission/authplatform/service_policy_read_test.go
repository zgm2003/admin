package authplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"admin/server/internal/config"
	projectredis "admin/server/internal/redis"
	"github.com/joho/godotenv"
)

type countingPolicyRepository struct {
	findCalls    int
	platform     Platform
	beforeReturn func()
}

func (c *countingPolicyRepository) FindPolicy(context.Context, string) (Platform, error) {
	c.findCalls++
	if c.beforeReturn != nil {
		c.beforeReturn()
	}
	return c.platform, nil
}

func TestCurrentPolicyDoesNotUsePostgresWhenRedisSnapshotIsReady(t *testing.T) {
	store := newReadyPolicyStore(t)
	repository := &countingPolicyRepository{}
	service := &Service{policyReader: repository, policies: store}
	policy, err := service.CurrentPolicy(context.Background(), "admin")
	if err != nil || repository.findCalls != 0 {
		t.Fatalf("policy=%+v err=%v findCalls=%d", policy, err, repository.findCalls)
	}
}

func TestCurrentPolicyCoalescesConcurrentMissingReads(t *testing.T) {
	redis := openPolicyRedis(t)
	if err := redis.Delete(context.Background(), PolicyKey("admin")); err != nil {
		t.Fatal(err)
	}
	store := NewPolicyStore(redis)
	repository := &countingPolicyRepository{platform: validReadPlatform()}
	service := &Service{policyReader: repository, policies: store}

	var wait sync.WaitGroup
	results := make(chan error, 32)
	for i := 0; i < 32; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := service.CurrentPolicy(context.Background(), "admin")
			results <- err
		}()
	}
	wait.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent CurrentPolicy error = %v", err)
		}
	}
	if repository.findCalls != 1 {
		t.Fatalf("PostgreSQL reads = %d, want 1", repository.findCalls)
	}
}

func TestCurrentPolicyUsesNewerSnapshotInstalledDuringPostgresRead(t *testing.T) {
	redis := openPolicyRedis(t)
	ctx := context.Background()
	if err := redis.Delete(ctx, PolicyKey("admin")); err != nil {
		t.Fatal(err)
	}
	store := NewPolicyStore(redis)
	newer := validReadPolicy()
	newer.PolicyVersion = 2
	newer.LoginTypes = []LoginType{LoginTypePassword}
	repository := &countingPolicyRepository{platform: validReadPlatform()}
	repository.beforeReturn = func() {
		if _, _, err := store.installReadyIfMissing(ctx, newer); err != nil {
			t.Fatal(err)
		}
	}
	service := &Service{policyReader: repository, policies: store}

	policy, err := service.CurrentPolicy(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if policy.PolicyVersion != newer.PolicyVersion || len(policy.LoginTypes) != 1 || policy.LoginTypes[0] != LoginTypePassword {
		t.Fatalf("CurrentPolicy returned stale PostgreSQL policy: %+v", policy)
	}
}

func TestCurrentPolicyIgnoresLegacySchemaKeyAfterLoginTypesUpgrade(t *testing.T) {
	redisClient := openPolicyRedis(t)
	ctx := context.Background()
	code := fmt.Sprintf("legacy_schema_%d", time.Now().UnixNano())
	legacyKey := "auth:policy:" + code
	if err := redisClient.SetString(ctx, legacyKey, `{"schemaVersion":1,"state":"ready","mutationToken":null,"policy":{"id":1,"code":"`+code+`","name":"Legacy","policyVersion":1,"accessTTL":900000000000,"refreshTTL":1209600000000000,"sessionCacheTTL":1800000000000,"accessCacheTTL":1800000000000,"bindDevice":false,"bindIP":false,"maxSessions":1,"allowRegister":false,"isEnabled":true,"isBuiltin":false,"deleted":false}}`, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = redisClient.DeleteMany(context.Background(), []string{legacyKey, PolicyKey(code)})
	})
	platform := validReadPlatform()
	platform.Code = code
	repository := &countingPolicyRepository{platform: platform}
	service := &Service{policyReader: repository, policies: NewPolicyStore(redisClient)}

	policy, err := service.CurrentPolicy(ctx, code)
	if err != nil {
		t.Fatalf("CurrentPolicy() = %+v, %v", policy, err)
	}
	if repository.findCalls != 1 || policy.Code != code {
		t.Fatalf("legacy key recovery policy=%+v findCalls=%d", policy, repository.findCalls)
	}
}

func validReadPlatform() Platform {
	return Platform{
		ID: 1, Code: "admin", Name: "Admin", LoginTypes: json.RawMessage(`["email","password"]`), PolicyVersion: 1,
		AccessTTLSeconds: 900, RefreshTTLSeconds: 1_209_600,
		SessionCacheTTLSeconds: 1800, AccessCacheTTLSeconds: 1800,
		BindDevice: 0, BindIP: 0, MaxSessions: 1, AllowRegister: 0, IsEnabled: 1, IsBuiltin: 1,
	}
}

func newReadyPolicyStore(t *testing.T) *PolicyStore {
	t.Helper()
	store := NewPolicyStore(openPolicyRedis(t))
	if _, _, err := store.installReadyIfMissing(context.Background(), validReadPolicy()); err != nil {
		t.Fatal(err)
	}
	return store
}

func validReadPolicy() Policy {
	return Policy{
		ID: 1, Code: "admin", Name: "Admin", LoginTypes: []LoginType{LoginTypeEmail, LoginTypePassword}, PolicyVersion: 1,
		AccessTTL: 15 * time.Minute, RefreshTTL: 14 * 24 * time.Hour,
		SessionCacheTTL: 30 * time.Minute, AccessCacheTTL: 30 * time.Minute,
		MaxSessions: 1, AllowRegister: false, IsEnabled: true, IsBuiltin: true,
	}
}

func openPolicyRedis(t *testing.T) *projectredis.Client {
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
	redisURL, err := url.Parse(settings.RedisURL)
	if err != nil {
		t.Fatal(err)
	}
	redisURL.Path = "/12"
	redisURL.RawPath = ""
	client, err := projectredis.Open(context.Background(), redisURL.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}
