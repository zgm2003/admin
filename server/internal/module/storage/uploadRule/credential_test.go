package uploadrule

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"admin/server/internal/database/testquery"
	"admin/server/internal/module/auth/login"
	"admin/server/internal/module/storage/cosConfig"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	cachegeneration "admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/yesno"
	storagecos "admin/server/internal/storage/cos"
	"gorm.io/gorm"
)

// newConfigService 按 API 装配方式同时给 cosConfig 的 Repository 与 Service 注入 generation。
func newConfigService(t *testing.T, db *gorm.DB, keys *secretkey.KeyRing) *cosconfig.Service {
	t.Helper()
	repository := cosconfig.NewRepository(db)
	repository.SetGenerations(cachegeneration.NewRepository(db))
	service := cosconfig.NewService(repository, keys, nil)
	// 该 helper 只用于构造 COS 配置事实；Create 在 states 为 nil 时跳过同步发布，由 relay/repair 恢复。
	service.SetGenerations(cachegeneration.NewRepository(db), nil)
	return service
}

type recordingSigner struct {
	credentials storagecos.Credentials
	requests    []storagecos.PutRequest
	getRequests []storagecos.GetRequest
	getResult   storagecos.GetResult
	putErr      error
	getErr      error
}

func (s *recordingSigner) PresignPut(_ context.Context, credentials storagecos.Credentials, request storagecos.PutRequest) (storagecos.PutResult, error) {
	s.credentials = credentials
	s.requests = append(s.requests, request)
	if s.putErr != nil {
		return storagecos.PutResult{}, s.putErr
	}
	return storagecos.PutResult{URL: "https://upload.example.com/signed", Headers: map[string]string{"Content-Type": request.ContentType}}, nil
}

func (s *recordingSigner) PresignGet(_ context.Context, credentials storagecos.Credentials, request storagecos.GetRequest) (storagecos.GetResult, error) {
	s.credentials = credentials
	s.getRequests = append(s.getRequests, request)
	if s.getErr != nil {
		return storagecos.GetResult{}, s.getErr
	}
	if s.getResult.URL == "" {
		return storagecos.GetResult{URL: "https://download.example.com/signed", ExpiresAt: time.Now().UTC().Add(storagecos.GetPresignValidity)}, nil
	}
	return s.getResult, nil
}

type fixedRuntimeSource struct {
	config cosconfig.RuntimeConfig
	err    error
	calls  int
}

type acceptingConnectionTester struct{}

func (acceptingConnectionTester) TestConnection(context.Context, storagecos.Credentials) error {
	return nil
}

func (s *fixedRuntimeSource) Runtime(_ context.Context, _ int64) (cosconfig.RuntimeConfig, error) {
	s.calls++
	return s.config, s.err
}

func TestIssueCredentialsValidatesAndSignsPlatformRule(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	platformID := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	otherPlatformID := insertPlatform(t, db, ctx, "canvas", yesno.Yes)
	keys, err := secretkey.New(strings.Repeat("k", 64))
	if err != nil {
		t.Fatal(err)
	}
	configService := newConfigService(t, db, keys)
	domain := "https://cdn.example.com"
	configID, err := configService.Create(ctx, cosconfig.CreateInput{Name: "Main", AppID: "1250000000", SecretID: "secret-id", SecretKey: "secret-key", Bucket: "assets", Region: "ap-guangzhou", BucketDomain: &domain, IsEnabled: yesno.Yes})
	if err != nil {
		t.Fatal(err)
	}
	signer := &recordingSigner{}
	runtime, err := cosconfig.NewRepository(db).RuntimeFacts(ctx, configID)
	if err != nil {
		t.Fatal(err)
	}
	runtimeSource := &fixedRuntimeSource{config: runtime}
	service := NewService(NewRepository(db), keys, signer, runtimeSource, nil)
	rule := validCreate(platformID, configID, []string{"avatar", "article-cover"}, yesno.Yes)
	rule.AccessMode = "public"
	if _, err := service.Create(ctx, rule); err != nil {
		t.Fatal(err)
	}
	before := time.Now().UTC()
	result, err := service.IssueCredentials(ctx, auth.Identity{PlatformID: platformID}, CredentialInput{RuleCode: "article-cover", Files: []FileInput{{FileName: "photo.PNG", ContentType: "image/png", FileSizeBytes: 100}, {FileName: "cover.PNG", ContentType: "image/png", FileSizeBytes: 100}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 || len(signer.requests) != 2 {
		t.Fatalf("result=%+v requests=%+v", result, signer.requests)
	}
	item := result.Items[0]
	if item.Method != "PUT" || !strings.HasPrefix(item.ObjectKey, "article-cover/.admin-storage/v2/") || strings.Contains(item.ObjectKey, "photo") || !strings.HasSuffix(item.ObjectKey, ".png") || item.PublicURL == nil || !strings.HasPrefix(*item.PublicURL, "https://cdn.example.com/article-cover/") {
		t.Fatalf("item=%+v", item)
	}
	coordinates, err := parseV2ObjectKey(item.ObjectKey)
	if err != nil || coordinates.PlatformID != platformID || coordinates.CosConfigID != configID || coordinates.Version != runtime.CurrentVersion {
		t.Fatalf("coordinates=%+v error=%v", coordinates, err)
	}
	if item.ExpiresAt.Before(before.Add(9*time.Minute+50*time.Second)) || item.ExpiresAt.After(time.Now().UTC().Add(10*time.Minute+time.Second)) {
		t.Fatalf("expiresAt=%v", item.ExpiresAt)
	}
	if signer.credentials.SecretID != "secret-id" || signer.credentials.SecretKey != "secret-key" || signer.requests[0].ContentLength != 100 || !signer.requests[0].PublicRead {
		t.Fatalf("credentials=%+v request=%+v", signer.credentials, signer.requests[0])
	}
	if runtimeSource.calls != 1 {
		t.Fatalf("runtime calls=%d want 1", runtimeSource.calls)
	}
	if _, err := service.IssueCredentials(ctx, auth.Identity{PlatformID: otherPlatformID}, CredentialInput{RuleCode: "avatar", Files: []FileInput{{FileName: "photo.png", ContentType: "image/png", FileSizeBytes: 100}}}); appCode(err) != apperror.CodeNotFound {
		t.Fatalf("cross-platform error=%v", err)
	}
}

func TestIssueCredentialsRejectsRuleViolations(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	platformID := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	keys, _ := secretkey.New(strings.Repeat("z", 64))
	configService := newConfigService(t, db, keys)
	configID, err := configService.Create(ctx, cosconfig.CreateInput{Name: "Main", AppID: "1", SecretID: "id", SecretKey: "key", Bucket: "assets", Region: "ap-guangzhou", IsEnabled: yesno.Yes})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := cosconfig.NewRepository(db).RuntimeFacts(ctx, configID)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(NewRepository(db), keys, &recordingSigner{}, &fixedRuntimeSource{config: runtime}, nil)
	rule := validCreate(platformID, configID, []string{"avatar"}, yesno.Yes)
	rule.MaxFileSizeBytes = 100
	if _, err := service.Create(ctx, rule); err != nil {
		t.Fatal(err)
	}
	invalid := []CredentialInput{{RuleCode: "avatar", Files: nil}, {RuleCode: "avatar", Files: []FileInput{{FileName: "../photo.png", ContentType: "image/png", FileSizeBytes: 1}}}, {RuleCode: "avatar", Files: []FileInput{{FileName: "photo.jpg", ContentType: "image/png", FileSizeBytes: 1}}}, {RuleCode: "avatar", Files: []FileInput{{FileName: "photo.png", ContentType: "text/plain", FileSizeBytes: 1}}}, {RuleCode: "avatar", Files: []FileInput{{FileName: "photo.png", ContentType: "image/png", FileSizeBytes: 101}}}}
	for _, input := range invalid {
		if _, err := service.IssueCredentials(ctx, auth.Identity{PlatformID: platformID}, input); appCode(err) != apperror.CodeInvalidRequest {
			t.Fatalf("input=%+v error=%v", input, err)
		}
	}
}

func TestObjectURLUsesImmutableRouteAndHistoricalPublicVersion(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	platformID := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	otherPlatformID := insertPlatform(t, db, ctx, "canvas", yesno.Yes)
	configID := insertConfig(t, db, ctx, "main", yesno.Yes, "https://current.example.com")
	repository := NewRepository(db)
	rule := validCreate(platformID, configID, []string{"avatar"}, yesno.Yes)
	rule.AccessMode = "public"
	ruleID, err := NewService(repository, nil, nil, nil, nil).Create(ctx, rule)
	if err != nil {
		t.Fatal(err)
	}
	oldDomain := "https://old.example.com/base"
	currentDomain := "https://current.example.com"
	runtime := &fixedRuntimeSource{config: cosconfig.RuntimeConfig{
		ID: configID, AppID: "1250000000", CurrentVersion: 2, IsEnabled: yesno.No,
		Versions: []cosconfig.RuntimeVersion{
			{Version: 1, Bucket: "old", Region: "ap-guangzhou", BucketDomain: &oldDomain},
			{Version: 2, Bucket: "current", Region: "ap-shanghai", BucketDomain: &currentDomain},
		},
	}}
	cache := NewRouteCache(testRouteRedisClient(t))
	cleanupRouteCacheKey(t, cache.client, ruleID)
	service := NewService(repository, nil, nil, runtime, cache)
	key, err := generateObjectKey("avatar", ObjectCoordinates{PlatformID: platformID, RuleID: ruleID, CosConfigID: configID, Version: 1}, "png", time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.ObjectURL(ctx, auth.Identity{PlatformID: platformID}, key)
	if err != nil || result.URL != oldDomain+"/"+key || result.ExpiresAt != nil {
		t.Fatalf("public result=%+v error=%v", result, err)
	}
	if err := db.WithContext(ctx).Model(&Model{}).Where("id = ?", ruleID).Updates(map[string]any{"is_enabled": yesno.No, "deleted_at": time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	routeKey, _ := routeCacheKey(ruleID)
	if err := cache.client.Delete(ctx, routeKey); err != nil {
		t.Fatal(err)
	}
	result, err = service.ObjectURL(ctx, auth.Identity{PlatformID: platformID}, key)
	if err != nil || result.URL != oldDomain+"/"+key {
		t.Fatalf("soft-deleted route result=%+v error=%v", result, err)
	}
	if _, err := service.ObjectURL(ctx, auth.Identity{PlatformID: otherPlatformID}, key); appCode(err) != apperror.CodeForbidden {
		t.Fatalf("cross-platform error=%v", err)
	}
	tampered := strings.Replace(key, fmt.Sprintf("/c%d/", configID), fmt.Sprintf("/c%d/", configID+1), 1)
	if _, err := service.ObjectURL(ctx, auth.Identity{PlatformID: platformID}, tampered); appCode(err) != apperror.CodeInvalidRequest {
		t.Fatalf("tampered coordinates error=%v", err)
	}
	if _, err := service.ObjectURL(ctx, auth.Identity{PlatformID: platformID}, "avatar/2026/09/17/legacy.png"); appCode(err) != apperror.CodeInvalidRequest {
		t.Fatalf("legacy key error=%v", err)
	}
}

func TestObjectURLReadyRouteAndRuntimeUseZeroPostgresSelects(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	counter := testquery.New(db.Logger, "storage_cos_config", "storage_cos_config_version", "storage_upload_rule")
	db = db.Session(&gorm.Session{Logger: counter})
	client := testRouteRedisClient(t)
	keys, err := secretkey.New(strings.Repeat("h", 64))
	if err != nil {
		t.Fatal(err)
	}
	generations := cachegeneration.NewRepository(db)
	states := cachegeneration.NewStore(client)
	configRepository := cosconfig.NewRepository(db)
	configRepository.SetGenerations(generations)
	configCache := cosconfig.NewCache(client)
	configCache.SetStateStore(states)
	configService := cosconfig.NewService(configRepository, keys, acceptingConnectionTester{})
	configService.SetGenerations(generations, states)
	configService.SetCache(configCache)
	domain := "https://cdn.example.com"
	configID, err := configService.Create(ctx, cosconfig.CreateInput{
		Name: "Hot", AppID: "1250000000", SecretID: "secret-id", SecretKey: "secret-key",
		Bucket: "assets", Region: "ap-guangzhou", BucketDomain: &domain, IsEnabled: yesno.Yes,
	})
	if err != nil {
		t.Fatal(err)
	}
	configScope, err := cachegeneration.NewScope("storage.cosconfig", fmt.Sprintf("%d", configID))
	if err != nil {
		t.Fatal(err)
	}
	runtimeKey, err := cachegeneration.SnapshotKey(configScope, 1, "runtime")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = client.DeleteMany(context.Background(), []string{cachegeneration.StateKey(configScope), runtimeKey})
	})

	platformID := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	repository := NewRepository(db)
	rule := validCreate(platformID, configID, []string{"avatar"}, yesno.Yes)
	rule.AccessMode = "public"
	ruleID, err := NewService(repository, nil, nil, nil, nil).Create(ctx, rule)
	if err != nil {
		t.Fatal(err)
	}
	routeCache := NewRouteCache(client)
	cleanupRouteCacheKey(t, client, ruleID)
	service := NewService(repository, keys, nil, configService, routeCache)
	key, err := generateObjectKey("avatar", ObjectCoordinates{PlatformID: platformID, RuleID: ruleID, CosConfigID: configID, Version: 1}, "png", time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ObjectURL(ctx, auth.Identity{PlatformID: platformID}, key); err != nil {
		t.Fatalf("warm object URL: %v", err)
	}

	counter.Reset()
	result, err := service.ObjectURL(ctx, auth.Identity{PlatformID: platformID}, key)
	if err != nil || result.URL != domain+"/"+key {
		t.Fatalf("hot object URL = %+v, %v", result, err)
	}
	if got := counter.Total(); got != 0 {
		t.Fatalf("hot object URL SELECT budget = %d, want 0", got)
	}
}

func TestObjectURLPresignsPrivateHistoricalVersionAndFailsClosed(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	platformID := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	keys, err := secretkey.New(strings.Repeat("p", 64))
	if err != nil {
		t.Fatal(err)
	}
	configService := newConfigService(t, db, keys)
	configID, err := configService.Create(ctx, cosconfig.CreateInput{Name: "Private", AppID: "1250000000", SecretID: "secret-id", SecretKey: "secret-key", Bucket: "old", Region: "ap-guangzhou", IsEnabled: yesno.Yes})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := cosconfig.NewRepository(db).RuntimeFacts(ctx, configID)
	if err != nil {
		t.Fatal(err)
	}
	runtime.CurrentVersion = 2
	runtime.IsEnabled = yesno.No
	runtime.Versions = append(runtime.Versions, cosconfig.RuntimeVersion{Version: 2, Bucket: "current", Region: "ap-shanghai"})
	repository := NewRepository(db)
	ruleID, err := NewService(repository, nil, nil, nil, nil).Create(ctx, validCreate(platformID, configID, []string{"private-file"}, yesno.Yes))
	if err != nil {
		t.Fatal(err)
	}
	cache := NewRouteCache(testRouteRedisClient(t))
	cleanupRouteCacheKey(t, cache.client, ruleID)
	expiresAt := time.Now().UTC().Add(9 * time.Minute)
	signer := &recordingSigner{getResult: storagecos.GetResult{URL: "https://download.example.com/private", ExpiresAt: expiresAt}}
	service := NewService(repository, keys, signer, &fixedRuntimeSource{config: runtime}, cache)
	key, err := generateObjectKey("private-file", ObjectCoordinates{PlatformID: platformID, RuleID: ruleID, CosConfigID: configID, Version: 1}, "png", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.ObjectURL(ctx, auth.Identity{PlatformID: platformID}, key)
	if err != nil || result.URL != signer.getResult.URL || result.ExpiresAt == nil || !result.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("private result=%+v error=%v", result, err)
	}
	if len(signer.getRequests) != 1 || signer.getRequests[0].ObjectKey != key || signer.credentials.Bucket != "old" || signer.credentials.Region != "ap-guangzhou" || signer.credentials.SecretID != "secret-id" {
		t.Fatalf("credentials=%+v requests=%+v", signer.credentials, signer.getRequests)
	}

	missingVersion := strings.Replace(key, "/v1/", "/v9/", 1)
	if _, err := service.ObjectURL(ctx, auth.Identity{PlatformID: platformID}, missingVersion); appCode(err) != apperror.CodeNotFound {
		t.Fatalf("missing version error=%v", err)
	}
	signer.getErr = errors.New("sign failed")
	if _, err := service.ObjectURL(ctx, auth.Identity{PlatformID: platformID}, key); appCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("sign failure error=%v", err)
	}
}
