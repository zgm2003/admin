package cosconfig

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	projectredis "admin/server/internal/redis"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	cachegeneration "admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/yesno"
	"admin/server/internal/storage/cos"
	"gorm.io/gorm"
)

type recordingConnectionTester struct {
	credentials cos.Credentials
	calls       int
	err         error
}

func (t *recordingConnectionTester) TestConnection(_ context.Context, credentials cos.Credentials) error {
	t.calls++
	t.credentials = credentials
	return t.err
}

func newTestService(t *testing.T, db *gorm.DB, keys *secretkey.KeyRing, tester cos.ConnectionTester) *Service {
	t.Helper()
	client := testRedisClient(t)
	generations := cachegeneration.NewRepository(db)
	states := cachegeneration.NewStore(client)
	cache := NewCache(client)
	cache.SetStateStore(states)
	repository := NewRepository(db)
	repository.SetGenerations(generations)
	service := NewService(repository, keys, tester)
	service.SetGenerations(generations, states)
	service.SetCache(cache)
	// 并发负载下 harness 放宽读预算；预算语义本身由 fake clock 用例断言。
	service.readBudget = 10 * time.Second
	t.Cleanup(func() { cleanupCosCacheKeys(t, db, client) })
	return service
}

// testRedisClient 指向独立 DB 序号，避免与运行中的 system.setting 缓存及其它包测试互相干扰。
func testRedisClient(t *testing.T) *projectredis.Client {
	t.Helper()
	settings := loadCosConfigSettings(t)
	parsed, err := url.Parse(settings.RedisURL)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/2"
	client, err := projectredis.Open(context.Background(), parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// cleanupCosCacheKeys 只删除本测试 schema 里出现过的 config ID 对应的精确 key，不使用 pattern、FLUSHDB 或 KEYS。
func cleanupCosCacheKeys(t *testing.T, db *gorm.DB, client *projectredis.Client) {
	t.Helper()
	var ids []int64
	if err := db.WithContext(context.Background()).Raw(`SELECT id FROM storage_cos_config`).Scan(&ids).Error; err != nil {
		return
	}
	keys := make([]string, 0, len(ids)*7)
	for _, id := range ids {
		base := "storage.cosconfig:" + strconv.FormatInt(id, 10)
		keys = append(keys, "config-cache:state:v1:"+base)
		for generation := int64(1); generation <= 3; generation++ {
			keys = append(keys, fmt.Sprintf("config-cache:snapshot:v1:%s:%d:runtime", base, generation))
			keys = append(keys, fmt.Sprintf("config-cache:fill:v1:%s:%d:runtime", base, generation))
		}
	}
	if len(keys) > 0 {
		_ = client.DeleteMany(context.Background(), keys)
	}
}

func TestServiceEncryptsNormalizesAndPreservesCredentials(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	keys, err := secretkey.New(strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	tester := &recordingConnectionTester{}
	service := newTestService(t, db, keys, tester)
	endpoint := " https://cos.example.com "
	domain := " https://cdn.example.com "
	id, err := service.Create(ctx, CreateInput{Name: " Main ", AppID: " 1250000000 ", SecretID: " secret-id ", SecretKey: " secret-key ", Bucket: " assets ", Region: " ap-guangzhou ", Endpoint: &endpoint, BucketDomain: &domain, IsEnabled: yesno.Yes, Remark: " primary "})
	if err != nil || id < 1 {
		t.Fatalf("create = %d,%v", id, err)
	}
	stored := readCurrentConfig(t, db, ctx, id)
	if stored.SecretIDCiphertext == "secret-id" || stored.SecretKeyCiphertext == "secret-key" || !strings.HasPrefix(stored.SecretIDCiphertext, "v1:") || !strings.HasPrefix(stored.SecretKeyCiphertext, "v1:") {
		t.Fatalf("credentials were not encrypted: %+v", stored)
	}
	if stored.Name != "Main" || stored.AppID != "1250000000" || stored.Bucket != "assets" || stored.Region != "ap-guangzhou" ||
		stored.Endpoint == nil || *stored.Endpoint != "https://cos.example.com" || stored.BucketDomain == nil || *stored.BucketDomain != "https://cdn.example.com" ||
		stored.Remark != "primary" || stored.CurrentVersion != 1 {
		t.Fatalf("stored normalization = %+v", stored)
	}
	assertConfigGeneration(t, db, ctx, id, 1)
	assertConfigOutbox(t, db, ctx, id, 1, 1)

	if _, err := service.Create(ctx, CreateInput{Name: "main", AppID: "1250000000", SecretID: "id", SecretKey: "key", Bucket: "assets", Region: "ap-guangzhou", IsEnabled: yesno.Yes}); appCode(err) != apperror.CodeConflict {
		t.Fatalf("duplicate create error = %v", err)
	}

	oldSecretID, oldSecretKey := stored.SecretIDCiphertext, stored.SecretKeyCiphertext
	if err := service.Update(ctx, id, UpdateInput{Name: "Main Updated", Bucket: "assets", Region: "ap-guangzhou", Endpoint: stringPointer("https://cos.example.com"), BucketDomain: stringPointer("https://cdn.example.com"), Remark: "updated"}); err != nil {
		t.Fatal(err)
	}
	stored = readCurrentConfig(t, db, ctx, id)
	if stored.SecretIDCiphertext != oldSecretID || stored.SecretKeyCiphertext != oldSecretKey {
		t.Fatal("omitted update secrets changed ciphertext")
	}
	if stored.Name != "Main Updated" || stored.CurrentVersion != 1 {
		t.Fatalf("metadata update = %+v", stored)
	}
	assertConfigGeneration(t, db, ctx, id, 2)

	// 相同值重复提交属于 no-op：不推进 generation，也不新增物理版本。
	if err := service.Update(ctx, id, UpdateInput{Name: "Main Updated", Bucket: "assets", Region: "ap-guangzhou", Endpoint: stringPointer("https://cos.example.com"), BucketDomain: stringPointer("https://cdn.example.com"), Remark: "updated"}); err != nil {
		t.Fatal(err)
	}
	assertConfigGeneration(t, db, ctx, id, 2)
	assertConfigVersionCount(t, db, ctx, id, 1)

	if err := service.Update(ctx, id, UpdateInput{Name: "Main Updated", Bucket: "assets", Region: "ap-guangzhou", Endpoint: stringPointer("https://cos.example.com"), BucketDomain: stringPointer("https://cdn.example.com"), SecretKey: SecretInput{Present: true, Value: "new-key"}, Remark: "updated"}); err != nil {
		t.Fatal(err)
	}
	stored = readCurrentConfig(t, db, ctx, id)
	if stored.SecretIDCiphertext != oldSecretID || stored.SecretKeyCiphertext == oldSecretKey {
		t.Fatal("secret replacement did not update exactly one ciphertext")
	}
	assertConfigVersionCount(t, db, ctx, id, 1)

	if err := service.TestConnection(ctx, id); err != nil {
		t.Fatal(err)
	}
	if tester.calls != 1 || tester.credentials.SecretID != "secret-id" || tester.credentials.SecretKey != "new-key" || tester.credentials.Endpoint != "https://cos.example.com" {
		t.Fatalf("connection credentials = %+v calls=%d", tester.credentials, tester.calls)
	}
}

func TestServiceAllowsDisableAndRequiresNoReferencesToDelete(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	keys, err := secretkey.New(strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	service := newTestService(t, db, keys, &recordingConnectionTester{})
	id, err := service.Create(ctx, CreateInput{Name: "Main", AppID: "1250000000", SecretID: "id", SecretKey: "key", Bucket: "assets", Region: "ap-guangzhou", IsEnabled: yesno.Yes})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec("INSERT INTO storage_upload_rule (cos_config_id, is_enabled) VALUES (?, 1)", id).Error; err != nil {
		t.Fatal(err)
	}

	// 被规则引用的配置仍然允许停用：这是"已引用配置只能停用"的正常路径。
	if err := service.UpdateStatus(ctx, id, yesno.No); err != nil {
		t.Fatalf("disable referenced config error = %v", err)
	}
	stored := readCurrentConfig(t, db, ctx, id)
	if stored.IsEnabled != yesno.No {
		t.Fatalf("stored isEnabled = %d want disabled", stored.IsEnabled)
	}
	if err := service.Delete(ctx, id); appCode(err) != apperror.CodeConflict {
		t.Fatalf("delete referenced config error = %v", err)
	}

	// 已软删的规则同样计入引用：配置只能停用。
	if err := db.WithContext(ctx).Exec("UPDATE storage_upload_rule SET deleted_at = CURRENT_TIMESTAMP WHERE cos_config_id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, id); appCode(err) != apperror.CodeConflict {
		t.Fatalf("delete with soft deleted rule error = %v", err)
	}
	if references, err := service.repository.CountRuleReferences(ctx, id); err != nil || references != 1 {
		t.Fatalf("references = %d,%v want 1", references, err)
	}

	// 彻底移除规则引用后才允许软删。
	if err := db.WithContext(ctx).Exec("DELETE FROM storage_upload_rule WHERE cos_config_id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Get(ctx, id); appCode(err) != apperror.CodeNotFound {
		t.Fatalf("get deleted error = %v", err)
	}
	if err := service.Update(ctx, id, UpdateInput{Name: "Deleted", Bucket: "assets", Region: "ap-guangzhou"}); appCode(err) != apperror.CodeNotFound {
		t.Fatalf("update deleted error = %v", err)
	}
	if err := service.UpdateStatus(ctx, id, yesno.Yes); appCode(err) != apperror.CodeNotFound {
		t.Fatalf("status deleted error = %v", err)
	}
}

func TestServiceRejectsInvalidInputsAndUnavailableTester(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	keys, err := secretkey.New(strings.Repeat("c", 64))
	if err != nil {
		t.Fatal(err)
	}
	service := newTestService(t, db, keys, nil)
	for _, input := range []CreateInput{
		{Name: "", AppID: "1", SecretID: "id", SecretKey: "key", Bucket: "bucket", Region: "region", IsEnabled: yesno.Yes},
		{Name: "Main", AppID: "1", SecretID: "id", SecretKey: "key", Bucket: "bucket", Region: "region", Endpoint: stringPointer("http://cos.example.com"), IsEnabled: yesno.Yes},
		{Name: "Main", AppID: "1", SecretID: "id", SecretKey: "key", Bucket: "bucket", Region: "region", Endpoint: stringPointer("https://bad_host"), IsEnabled: yesno.Yes},
		{Name: "Main", AppID: "1", SecretID: "id", SecretKey: "key", Bucket: "bucket", Region: "region", BucketDomain: stringPointer("not a url"), IsEnabled: yesno.Yes},
	} {
		if _, err := service.Create(ctx, input); appCode(err) != apperror.CodeInvalidRequest {
			t.Fatalf("invalid create error = %v input=%+v", err, input)
		}
	}
	id, err := service.Create(ctx, CreateInput{Name: "Main", AppID: "1", SecretID: "id", SecretKey: "key", Bucket: "bucket", Region: "region", IsEnabled: yesno.Yes})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.TestConnection(ctx, id); appCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("nil tester error = %v", err)
	}
	if err := service.UpdateStatus(ctx, 0, yesno.Yes); appCode(err) != apperror.CodeInvalidRequest {
		t.Fatalf("invalid id error = %v", err)
	}
}

func TestServiceRequiresGenerationRepositoryForMutations(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	keys, err := secretkey.New(strings.Repeat("d", 64))
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(NewRepository(db), keys, &recordingConnectionTester{})
	if _, err := service.Create(ctx, CreateInput{Name: "Main", AppID: "1", SecretID: "id", SecretKey: "key", Bucket: "bucket", Region: "region", IsEnabled: yesno.Yes}); appCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("create without generations error = %v", err)
	}
}

func readCurrentConfig(t *testing.T, db *gorm.DB, ctx context.Context, id int64) Current {
	t.Helper()
	var row Current
	if err := db.WithContext(ctx).Table("storage_cos_config").Select(currentVersionColumns).Joins(currentVersionJoin).
		Where("storage_cos_config.id = ?", id).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ID == 0 {
		t.Fatalf("config %d not found", id)
	}
	return row
}

func appCode(err error) int {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return 0
}

func stringPointer(value string) *string { return &value }

func TestServiceValidateDependenciesRejectsIncompleteStartupWiring(t *testing.T) {
	if err := NewService(nil, nil, nil).ValidateDependencies(); err == nil {
		t.Fatal("ValidateDependencies accepted missing COS config dependencies")
	}
}
