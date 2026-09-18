package uploadrule

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestServiceValidatesTargetsAndAutoDisablesOtherEnabledRules(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	platformID := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	disabledPlatformID := insertPlatform(t, db, ctx, "disabled", yesno.No)
	configID := insertConfig(t, db, ctx, "main", yesno.Yes, "https://cdn.example.com")
	privateConfigID := insertConfig(t, db, ctx, "private", yesno.Yes, "")
	disabledConfigID := insertConfig(t, db, ctx, "disabled", yesno.No, "")
	service := NewService(NewRepository(db), nil, nil, nil, nil)

	firstID, err := service.Create(ctx, validCreate(platformID, configID, []string{"avatar", "article-cover"}, yesno.Yes))
	if err != nil {
		t.Fatal(err)
	}
	assertEnabledRule(t, db, ctx, platformID, firstID)

	// 同平台新建 enabled 规则：旧规则必须被自动停用，终态只有一条活动规则。
	secondID, err := service.Create(ctx, validCreate(platformID, configID, []string{"attachment"}, yesno.Yes))
	if err != nil {
		t.Fatal(err)
	}
	assertEnabledRule(t, db, ctx, platformID, secondID)
	listed, err := service.List(ctx, ListQuery{Page: 1, PageSize: 20})
	if err != nil || len(listed.List) != 2 {
		t.Fatalf("list result = %+v,%v", listed, err)
	}

	// 跨规则复用相同 code 是允许的数据。
	if _, err := service.Create(ctx, validCreate(platformID, configID, []string{"avatar"}, yesno.No)); err != nil {
		t.Fatalf("cross rule duplicate code error = %v", err)
	}
	// 同一规则内重复输入被规范化去重并保持首次顺序。
	deduped, err := service.Create(ctx, validCreate(platformID, configID, []string{"dup", "DUP ", "dup"}, yesno.No))
	if err != nil {
		t.Fatal(err)
	}
	if row, err := service.Get(ctx, deduped); err != nil || !reflect.DeepEqual(row.Codes, []string{"dup"}) {
		t.Fatalf("deduped codes = %+v,%v", row, err)
	}
	// 保留段与非法 code 一律 400。
	for _, codes := range [][]string{{"evil/.admin-storage/thing"}, {"a//b"}, {"trailing/"}, {"a..b"}} {
		if _, err := service.Create(ctx, validCreate(platformID, configID, codes, yesno.No)); appCode(err) != apperror.CodeInvalidRequest {
			t.Fatalf("invalid code %v error = %v", codes, err)
		}
	}
	if _, err := service.Create(ctx, validCreate(disabledPlatformID, configID, []string{"disabled-platform"}, yesno.No)); appCode(err) != apperror.CodeConflict {
		t.Fatalf("disabled platform error = %v", err)
	}
	if _, err := service.Create(ctx, validCreate(platformID, disabledConfigID, []string{"disabled-config"}, yesno.No)); appCode(err) != apperror.CodeConflict {
		t.Fatalf("disabled config error = %v", err)
	}
	public := validCreate(platformID, privateConfigID, []string{"public"}, yesno.No)
	public.AccessMode = "public"
	if _, err := service.Create(ctx, public); appCode(err) != apperror.CodeConflict {
		t.Fatalf("public config error = %v", err)
	}
	if err := service.Delete(ctx, secondID); appCode(err) != apperror.CodeConflict {
		t.Fatalf("delete enabled error = %v", err)
	}
	if err := service.UpdateStatus(ctx, secondID, yesno.No); err != nil {
		t.Fatal(err)
	}
	assertEnabledRule(t, db, ctx, platformID)
}

func TestServiceUpdatesCodesAndAllowsCrossRuleDuplicates(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	platformID := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	configID := insertConfig(t, db, ctx, "main", yesno.Yes, "")
	service := NewService(NewRepository(db), nil, nil, nil, nil)
	firstID, err := service.Create(ctx, validCreate(platformID, configID, []string{"avatar", "article-cover"}, yesno.Yes))
	if err != nil {
		t.Fatal(err)
	}

	update := UpdateInput{
		Codes:             []string{" Avatar-V2 ", "profile-photo", "avatar-v2"},
		Name:              "Updated avatar",
		MaxFileSizeBytes:  2048,
		AllowedExtensions: []string{"png"},
		AllowedMimeTypes:  []string{"image/png"},
	}
	if err = service.Update(ctx, firstID, update); err != nil {
		t.Fatal(err)
	}
	updated, err := service.Get(ctx, firstID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(updated.Codes, []string{"avatar-v2", "profile-photo"}) || updated.Name != "Updated avatar" || updated.MaxFileSizeBytes != 2048 {
		t.Fatalf("updated=%+v", updated)
	}
	if _, err = NewRepository(db).FindUploadTarget(ctx, platformID, "avatar"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("old code error=%v", err)
	}
	if target, targetErr := NewRepository(db).FindUploadTarget(ctx, platformID, "avatar-v2"); targetErr != nil || target.RuleID != firstID {
		t.Fatalf("new target=%+v error=%v", target, targetErr)
	}

	// 跨规则复用同一 code 是允许的：另一条（未启用）规则可以保存 avatar-v2。
	if _, err = service.Create(ctx, validCreate(platformID, configID, []string{"avatar-v2"}, yesno.No)); err != nil {
		t.Fatalf("cross rule duplicate after update error=%v", err)
	}

	// 非法 code 属于请求校验失败：不产生任何写入。
	invalid := update
	invalid.Codes = []string{"evil/.admin-storage/thing"}
	invalid.Name = "Must not persist"
	if err = service.Update(ctx, firstID, invalid); appCode(err) != apperror.CodeInvalidRequest {
		t.Fatalf("invalid update error=%v", err)
	}
	unchanged, err := service.Get(ctx, firstID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(unchanged.Codes, []string{"avatar-v2", "profile-photo"}) || unchanged.Name != "Updated avatar" {
		t.Fatalf("unchanged=%+v", unchanged)
	}
}

func TestConcurrentEnableProducesExactlyOneEnabledRule(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	platformID := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	configID := insertConfig(t, db, ctx, "main", yesno.Yes, "")
	service := NewService(NewRepository(db), nil, nil, nil, nil)
	firstID, err := service.Create(ctx, validCreate(platformID, configID, []string{"one"}, yesno.No))
	if err != nil {
		t.Fatal(err)
	}
	secondID, err := service.Create(ctx, validCreate(platformID, configID, []string{"two"}, yesno.No))
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	errorsByCall := make([]error, 2)
	var wait sync.WaitGroup
	for index, id := range []int64{firstID, secondID} {
		wait.Add(1)
		go func(index int, id int64) {
			defer wait.Done()
			<-start
			errorsByCall[index] = service.UpdateStatus(context.Background(), id, yesno.Yes)
		}(index, id)
	}
	close(start)
	wait.Wait()
	for _, err := range errorsByCall {
		if err != nil {
			t.Fatalf("concurrent enable error = %v", err)
		}
	}
	var enabled int64
	if err := db.WithContext(ctx).Model(&Model{}).Where("platform_id = ? AND is_enabled = 1 AND deleted_at IS NULL", platformID).Count(&enabled).Error; err != nil {
		t.Fatal(err)
	}
	if enabled != 1 {
		t.Fatalf("enabled rule count = %d want exactly 1", enabled)
	}
}

func validCreate(platformID, configID int64, codes []string, enabled yesno.Value) CreateInput {
	return CreateInput{PlatformID: platformID, Codes: codes, Name: strings.ToUpper(codes[0]), CosConfigID: configID, MaxFileSizeBytes: 1024, AllowedExtensions: []string{"png"}, AllowedMimeTypes: []string{"image/png"}, AccessMode: "private", IsEnabled: enabled}
}

func openRuleDatabase(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration test")
	}
	if err := godotenv.Load("../../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_uploadrule")
	if err := db.WithContext(ctx).Exec(`
		CREATE TABLE permission_auth_platform (id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY, code VARCHAR(49) NOT NULL, name VARCHAR(64) NOT NULL, is_enabled SMALLINT NOT NULL, deleted_at TIMESTAMPTZ);
		CREATE TABLE storage_cos_config (id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY, name VARCHAR(128) NOT NULL, app_id VARCHAR(32) NOT NULL DEFAULT '', secret_id_ciphertext TEXT NOT NULL DEFAULT '', secret_key_ciphertext TEXT NOT NULL DEFAULT '', current_version BIGINT NOT NULL DEFAULT 1, is_enabled SMALLINT NOT NULL, remark VARCHAR(512) NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, deleted_at TIMESTAMPTZ);
		CREATE TABLE storage_cos_config_version (cos_config_id BIGINT NOT NULL REFERENCES storage_cos_config(id) ON DELETE RESTRICT, version BIGINT NOT NULL, bucket VARCHAR(128) NOT NULL, region VARCHAR(64) NOT NULL, endpoint VARCHAR(255), bucket_domain VARCHAR(255), created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (cos_config_id, version));
		CREATE TABLE storage_upload_rule (id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY, platform_id BIGINT NOT NULL, name VARCHAR(128) NOT NULL, cos_config_id BIGINT NOT NULL, max_file_size_bytes BIGINT NOT NULL, allowed_extensions TEXT[] NOT NULL, allowed_mime_types TEXT[] NOT NULL, access_mode VARCHAR(16) NOT NULL, is_enabled SMALLINT NOT NULL, remark VARCHAR(512) NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, deleted_at TIMESTAMPTZ);
		CREATE TABLE storage_upload_rule_code (id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY, rule_id BIGINT NOT NULL, code VARCHAR(64) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, deleted_at TIMESTAMPTZ);
		CREATE UNIQUE INDEX ux_storage_upload_rule_code_rule_code ON storage_upload_rule_code(rule_id, code) WHERE deleted_at IS NULL;
		CREATE INDEX ix_storage_upload_rule_code_rule ON storage_upload_rule_code(rule_id, id) WHERE deleted_at IS NULL;
		CREATE UNIQUE INDEX ux_storage_upload_rule_platform_enabled ON storage_upload_rule(platform_id) WHERE is_enabled = 1 AND deleted_at IS NULL;
		CREATE TABLE system_config_cache_generation (namespace VARCHAR(128) NOT NULL, scope_key VARCHAR(128) NOT NULL, generation BIGINT NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL, CONSTRAINT pk_system_config_cache_generation PRIMARY KEY (namespace, scope_key));
		CREATE TABLE system_config_cache_outbox (id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY, namespace VARCHAR(128) NOT NULL, scope_key VARCHAR(128) NOT NULL, generation BIGINT NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, available_at TIMESTAMPTZ NOT NULL, locked_until TIMESTAMPTZ, lock_token VARCHAR(64), last_error VARCHAR(512) NOT NULL DEFAULT '', published_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL, CONSTRAINT uq_system_config_cache_outbox_generation UNIQUE (namespace, scope_key, generation));
	`).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func insertPlatform(t *testing.T, db *gorm.DB, ctx context.Context, code string, enabled yesno.Value) int64 {
	t.Helper()
	var id int64
	if err := db.WithContext(ctx).Raw("INSERT INTO permission_auth_platform(code,name,is_enabled) VALUES(?,?,?) RETURNING id", code, code, enabled).Scan(&id).Error; err != nil {
		t.Fatal(err)
	}
	return id
}
func insertConfig(t *testing.T, db *gorm.DB, ctx context.Context, name string, enabled yesno.Value, domain string) int64 {
	t.Helper()
	var id int64
	var value any
	if domain != "" {
		value = domain
	}
	if err := db.WithContext(ctx).Raw("INSERT INTO storage_cos_config(name,is_enabled,current_version) VALUES(?,?,1) RETURNING id", name, enabled).Scan(&id).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec("INSERT INTO storage_cos_config_version(cos_config_id,version,bucket,region,bucket_domain) VALUES(?,1,'assets','ap-guangzhou',?)", id, value).Error; err != nil {
		t.Fatal(err)
	}
	return id
}
func assertEnabledRule(t *testing.T, db *gorm.DB, ctx context.Context, platformID int64, wantIDs ...int64) {
	t.Helper()
	var rows []Model
	if err := db.WithContext(ctx).Where("platform_id=? AND is_enabled=1 AND deleted_at IS NULL", platformID).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(wantIDs) {
		t.Fatalf("enabled=%+v want=%v", rows, wantIDs)
	}
	for _, wantID := range wantIDs {
		found := false
		for _, row := range rows {
			if row.ID == wantID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("enabled=%+v want=%v", rows, wantIDs)
		}
	}
}
func appCode(err error) int {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return 0
}

var _ = time.Now

func TestServiceValidateDependenciesRejectsIncompleteStartupWiring(t *testing.T) {
	if err := NewService(nil, nil, nil, nil, nil).ValidateDependencies(); err == nil {
		t.Fatal("ValidateDependencies accepted missing upload rule dependencies")
	}
}
