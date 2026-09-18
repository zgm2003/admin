package cosconfig

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	cachegeneration "admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRepositoryCreateWritesVersionAndGeneration(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	repository := newTestRepository(t, db)
	now := time.Now().UTC()
	endpoint := "https://cos-internal.example.com"
	domain := "https://cdn.example.com"

	model := &Model{Name: "Main", AppID: "1250000000", SecretIDCiphertext: "v1:cipher-id", SecretKeyCiphertext: "v1:cipher-key", IsEnabled: yesno.Yes, Remark: "", CreatedAt: now, UpdatedAt: now}
	version := Version{Bucket: "assets", Region: "ap-guangzhou", Endpoint: &endpoint, BucketDomain: &domain, CreatedAt: now, UpdatedAt: now}
	result, err := repository.Create(ctx, model, version)
	if err != nil {
		t.Fatal(err)
	}
	if model.ID < 1 || model.CurrentVersion != 1 || !result.Changed || result.Generation != 1 || result.OutboxID < 1 {
		t.Fatalf("created model = %+v result=%+v", model, result)
	}

	var stored struct {
		Bucket       string
		Region       string
		Endpoint     *string
		BucketDomain *string
	}
	if err := db.WithContext(ctx).Raw(`SELECT bucket, region, endpoint, bucket_domain FROM storage_cos_config_version WHERE cos_config_id = ? AND version = 1`, model.ID).Scan(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Bucket != "assets" || stored.Region != "ap-guangzhou" || stored.Endpoint == nil || *stored.Endpoint != endpoint || stored.BucketDomain == nil || *stored.BucketDomain != domain {
		t.Fatalf("version row = %+v", stored)
	}
	assertConfigGeneration(t, db, ctx, model.ID, 1)
	assertConfigOutbox(t, db, ctx, model.ID, 1, 1)

	// 重复 name 冲突仍然映射为 ErrNameConflict。
	duplicate := *model
	duplicate.ID = 0
	duplicate.Name = "main"
	if _, err := repository.Create(ctx, &duplicate, version); !errors.Is(err, ErrNameConflict) {
		t.Fatalf("duplicate create error = %v", err)
	}
}

func TestRepositoryListAndGetUseCurrentVersion(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	repository := newTestRepository(t, db)
	now := time.Now().UTC()
	if _, err := repository.Create(ctx, &Model{Name: "Main", AppID: "1", SecretIDCiphertext: "v1:id", SecretKeyCiphertext: "v1:key", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}, Version{Bucket: "assets", Region: "ap-guangzhou", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}

	rows, err := repository.List(ctx, ListQuery{Page: 1, PageSize: 20, Keyword: "assets"})
	if err != nil || len(rows) != 1 || rows[0].Bucket != "assets" || rows[0].Region != "ap-guangzhou" {
		t.Fatalf("list = %+v,%v", rows, err)
	}
	current, err := repository.FindByID(ctx, rows[0].ID)
	if err != nil || current.Bucket != "assets" || current.AppID != "1" || current.SecretIDCiphertext != "v1:id" {
		t.Fatalf("find = %+v,%v", current, err)
	}
	total, err := repository.Count(ctx, ListQuery{Keyword: "assets"})
	if err != nil || total != 1 {
		t.Fatalf("count = %d,%v", total, err)
	}

	base, err := repository.generations.Current(ctx, mustConfigScope(t, rows[0].ID))
	if err != nil {
		t.Fatal(err)
	}
	result, err := repository.Update(ctx, rows[0].ID, UpdateValues{Name: "Main", Bucket: "assets-v2", Region: "ap-guangzhou", Remark: ""}, base, now.Add(time.Minute))
	if err != nil || !result.Changed {
		t.Fatalf("physical update = %+v,%v", result, err)
	}
	updated, err := repository.FindByID(ctx, rows[0].ID)
	if err != nil || updated.Bucket != "assets-v2" || updated.CurrentVersion != 2 {
		t.Fatalf("updated = %+v,%v", updated, err)
	}
	var versionCount int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM storage_cos_config_version WHERE cos_config_id = ?`, rows[0].ID).Scan(&versionCount).Error; err != nil {
		t.Fatal(err)
	}
	if versionCount != 2 {
		t.Fatalf("version count = %d want 2", versionCount)
	}
}

func TestRepositoryUpdateAdvancesGenerationOnlyForRealChanges(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	repository := newTestRepository(t, db)
	now := time.Now().UTC()
	model := &Model{Name: "Main", AppID: "1", SecretIDCiphertext: "v1:id", SecretKeyCiphertext: "v1:key", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	if _, err := repository.Create(ctx, model, Version{Bucket: "assets", Region: "ap-guangzhou", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	outboxBefore := countConfigOutbox(t, db, ctx, model.ID)

	// 名称真实变化：推进 generation，不新增物理版本。
	result, err := repository.Update(ctx, model.ID, UpdateValues{Name: "Renamed", Bucket: "assets", Region: "ap-guangzhou", Remark: ""}, 1, now)
	if err != nil || !result.Changed || result.Generation != 2 || result.OutboxID < 1 {
		t.Fatalf("rename = %+v,%v", result, err)
	}
	assertConfigGeneration(t, db, ctx, model.ID, 2)
	assertConfigVersionCount(t, db, ctx, model.ID, 1)

	// 完全相同输入：no-op，不推进 generation、不写 outbox。
	result, err = repository.Update(ctx, model.ID, UpdateValues{Name: "Renamed", Bucket: "assets", Region: "ap-guangzhou", Remark: ""}, 2, now.Add(time.Minute))
	if err != nil || result.Changed || result.Generation != 0 {
		t.Fatalf("no-op = %+v,%v", result, err)
	}
	assertConfigGeneration(t, db, ctx, model.ID, 2)
	if after := countConfigOutbox(t, db, ctx, model.ID); after != outboxBefore+1 {
		t.Fatalf("outbox after no-op = %d want %d", after, outboxBefore+1)
	}

	// Secret 轮换属于真实变化：推进 generation，不新增物理版本。
	rotated := "v1:rotated"
	result, err = repository.Update(ctx, model.ID, UpdateValues{Name: "Renamed", Bucket: "assets", Region: "ap-guangzhou", SecretKeyCiphertext: &rotated}, 2, now.Add(2*time.Minute))
	if err != nil || !result.Changed || result.Generation != 3 {
		t.Fatalf("rotate = %+v,%v", result, err)
	}
	assertConfigVersionCount(t, db, ctx, model.ID, 1)

	// generation 基准过期：整个事务回滚，业务事实不变。
	if _, err := repository.Update(ctx, model.ID, UpdateValues{Name: "Stale", Bucket: "assets", Region: "ap-guangzhou"}, 1, now.Add(3*time.Minute)); !errors.Is(err, cachegeneration.ErrGenerationChanged) {
		t.Fatalf("stale update error = %v", err)
	}
	stale, err := repository.FindByID(ctx, model.ID)
	if err != nil || stale.Name != "Renamed" || stale.CurrentVersion != 1 {
		t.Fatalf("stale update changed facts: %+v,%v", stale, err)
	}
	assertConfigGeneration(t, db, ctx, model.ID, 3)
}

func TestRepositoryStatusDeleteAndRuleReferences(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	repository := newTestRepository(t, db)
	now := time.Now().UTC()
	model := &Model{Name: "Main", AppID: "1", SecretIDCiphertext: "v1:id", SecretKeyCiphertext: "v1:key", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	if _, err := repository.Create(ctx, model, Version{Bucket: "assets", Region: "ap-guangzhou", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}

	if result, err := repository.UpdateStatus(ctx, model.ID, yesno.No, 1, now); err != nil || !result.Changed || result.Generation != 2 {
		t.Fatalf("disable = %+v,%v", result, err)
	}
	if result, err := repository.UpdateStatus(ctx, model.ID, yesno.No, 2, now.Add(time.Minute)); err != nil || result.Changed {
		t.Fatalf("repeat disable = %+v,%v", result, err)
	}

	// 引用统计包含已禁用与已软删规则：只要曾经被引用过就不能删除。
	for _, statement := range []string{
		`INSERT INTO storage_upload_rule (cos_config_id, is_enabled, deleted_at) VALUES (?, 0, NULL)`,
		`INSERT INTO storage_upload_rule (cos_config_id, is_enabled, deleted_at) VALUES (?, 0, CURRENT_TIMESTAMP)`,
	} {
		if err := db.WithContext(ctx).Exec(statement, model.ID).Error; err != nil {
			t.Fatal(err)
		}
	}
	if references, err := repository.CountRuleReferences(ctx, model.ID); err != nil || references != 2 {
		t.Fatalf("references = %d,%v want 2", references, err)
	}

	if result, err := repository.MarkDeleted(ctx, model.ID, 2, now.Add(2*time.Minute)); err != nil || !result.Changed || result.Generation != 3 {
		t.Fatalf("delete = %+v,%v", result, err)
	}
	if _, err := repository.FindByID(ctx, model.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("find after delete error = %v", err)
	}
	var deleted struct {
		IsEnabled int16
		DeletedAt *time.Time
	}
	if err := db.WithContext(ctx).Raw(`SELECT is_enabled, deleted_at FROM storage_cos_config WHERE id = ?`, model.ID).Scan(&deleted).Error; err != nil {
		t.Fatal(err)
	}
	if deleted.IsEnabled != 0 || deleted.DeletedAt == nil {
		t.Fatalf("deleted row = %+v", deleted)
	}
}

func TestRepositoryConcurrentUpdateRollsBackOnGenerationConflict(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	repository := newTestRepository(t, db)
	second := openSecondInstance(t, db)
	secondRepository := newTestRepository(t, second)
	now := time.Now().UTC()
	model := &Model{Name: "Main", AppID: "1", SecretIDCiphertext: "v1:id", SecretKeyCiphertext: "v1:key", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	if _, err := repository.Create(ctx, model, Version{Bucket: "assets", Region: "ap-guangzhou", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}

	if _, err := secondRepository.Update(ctx, model.ID, UpdateValues{Name: "Second", Bucket: "assets", Region: "ap-guangzhou"}, 1, now); err != nil {
		t.Fatalf("first writer = %v", err)
	}
	if _, err := repository.Update(ctx, model.ID, UpdateValues{Name: "First", Bucket: "assets", Region: "ap-guangzhou"}, 1, now.Add(time.Minute)); !errors.Is(err, cachegeneration.ErrGenerationChanged) {
		t.Fatalf("conflicting writer error = %v want generation conflict", err)
	}
	current, err := repository.FindByID(ctx, model.ID)
	if err != nil || current.Name != "Second" || current.CurrentVersion != 1 {
		t.Fatalf("winner state = %+v,%v", current, err)
	}
	assertConfigGeneration(t, db, ctx, model.ID, 2)
	assertConfigVersionCount(t, db, ctx, model.ID, 1)
}

func TestRepositoryRequiresGenerationRepository(t *testing.T) {
	db, ctx := openConfigDatabase(t)
	repository := NewRepository(db)
	now := time.Now().UTC()
	if _, err := repository.Create(ctx, &Model{Name: "Main", AppID: "1", SecretIDCiphertext: "v1:id", SecretKeyCiphertext: "v1:key", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}, Version{Bucket: "assets", Region: "ap-guangzhou"}); err == nil {
		t.Fatal("create succeeded without generation repository")
	}
	if _, err := repository.Update(ctx, 1, UpdateValues{Name: "x", Bucket: "assets", Region: "ap-guangzhou"}, 1, now); err == nil {
		t.Fatal("update succeeded without generation repository")
	}
}

func newTestRepository(t *testing.T, db *gorm.DB) *Repository {
	t.Helper()
	repository := NewRepository(db)
	repository.SetGenerations(cachegeneration.NewRepository(db))
	return repository
}

func mustConfigScope(t *testing.T, id int64) cachegeneration.Scope {
	t.Helper()
	scope, err := cachegeneration.NewScope(cosConfigGenerationNamespace, decimalKey(id))
	if err != nil {
		t.Fatal(err)
	}
	return scope
}

func decimalKey(id int64) string { return strconv.FormatInt(id, 10) }

func assertConfigGeneration(t *testing.T, db *gorm.DB, ctx context.Context, id, want int64) {
	t.Helper()
	var generation int64
	if err := db.WithContext(ctx).Raw(`SELECT generation FROM system_config_cache_generation WHERE namespace = ? AND scope_key = ?`, cosConfigGenerationNamespace, decimalKey(id)).Scan(&generation).Error; err != nil {
		t.Fatal(err)
	}
	if generation != want {
		t.Fatalf("config %d generation = %d want %d", id, generation, want)
	}
}

func assertConfigOutbox(t *testing.T, db *gorm.DB, ctx context.Context, id, generation, want int64) {
	t.Helper()
	var count int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM system_config_cache_outbox WHERE namespace = ? AND scope_key = ? AND generation = ?`, cosConfigGenerationNamespace, decimalKey(id), generation).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("config %d outbox generation %d = %d want %d", id, generation, count, want)
	}
}

func countConfigOutbox(t *testing.T, db *gorm.DB, ctx context.Context, id int64) int64 {
	t.Helper()
	var count int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM system_config_cache_outbox WHERE namespace = ? AND scope_key = ?`, cosConfigGenerationNamespace, decimalKey(id)).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func assertConfigVersionCount(t *testing.T, db *gorm.DB, ctx context.Context, id, want int64) {
	t.Helper()
	var count int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM storage_cos_config_version WHERE cos_config_id = ?`, id).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("config %d version count = %d want %d", id, count, want)
	}
}

// openSecondInstance 在同一隔离 schema 上再开一个独立连接池，模拟第二个 API 实例。
func openSecondInstance(t *testing.T, db *gorm.DB) *gorm.DB {
	t.Helper()
	var schema string
	if err := db.WithContext(context.Background()).Raw(`SELECT current_schema()`).Scan(&schema).Error; err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(schema) == "" {
		t.Fatal("current schema is empty")
	}
	settings := loadCosConfigSettings(t)
	pgxConfig, err := pgx.ParseConfig(settings.PostgresDSN)
	if err != nil {
		t.Fatal(err)
	}
	pgxConfig.RuntimeParams["search_path"] = schema
	sqlDB := stdlib.OpenDB(*pgxConfig)
	if err := sqlDB.PingContext(context.Background()); err != nil {
		_ = sqlDB.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	second, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	return second
}

func loadCosConfigSettings(t *testing.T) config.Worker {
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
	return settings
}

func openConfigDatabase(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	settings := loadCosConfigSettings(t)
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_cosconfig")
	if err := db.WithContext(ctx).Exec(`
		CREATE TABLE storage_cos_config (
			id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
			name VARCHAR(128) NOT NULL,
			app_id VARCHAR(32) NOT NULL,
			secret_id_ciphertext TEXT NOT NULL,
			secret_key_ciphertext TEXT NOT NULL,
			current_version BIGINT NOT NULL,
			is_enabled SMALLINT NOT NULL DEFAULT 1 CHECK (is_enabled IN (0, 1)),
			remark VARCHAR(512) NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMPTZ
		);
		CREATE UNIQUE INDEX ux_storage_cos_config_name_active ON storage_cos_config (lower(name)) WHERE deleted_at IS NULL;
		CREATE TABLE storage_cos_config_version (
			cos_config_id BIGINT NOT NULL REFERENCES storage_cos_config(id) ON DELETE RESTRICT,
			version BIGINT NOT NULL CHECK (version >= 1),
			bucket VARCHAR(128) NOT NULL,
			region VARCHAR(64) NOT NULL,
			endpoint VARCHAR(255),
			bucket_domain VARCHAR(255),
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (cos_config_id, version)
		);
		CREATE TABLE storage_upload_rule (
			id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
			cos_config_id BIGINT NOT NULL,
			is_enabled SMALLINT NOT NULL DEFAULT 1,
			deleted_at TIMESTAMPTZ
		);
		CREATE TABLE system_config_cache_generation (
			namespace VARCHAR(128) NOT NULL,
			scope_key VARCHAR(128) NOT NULL,
			generation BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			CONSTRAINT pk_system_config_cache_generation PRIMARY KEY (namespace, scope_key)
		);
		CREATE TABLE system_config_cache_outbox (
			id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
			namespace VARCHAR(128) NOT NULL,
			scope_key VARCHAR(128) NOT NULL,
			generation BIGINT NOT NULL,
			attempts INTEGER NOT NULL DEFAULT 0,
			available_at TIMESTAMPTZ NOT NULL,
			locked_until TIMESTAMPTZ,
			lock_token VARCHAR(64),
			last_error VARCHAR(512) NOT NULL DEFAULT '',
			published_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			CONSTRAINT uq_system_config_cache_outbox_generation UNIQUE (namespace, scope_key, generation)
		);
	`).Error; err != nil {
		t.Fatal(err)
	}
	// 每个测试 schema 使用互不重叠的 config ID 段，使 Redis scope key 天然隔离。
	base := time.Now().UnixNano()%1_000_000*1000 + 1000
	if err := db.WithContext(ctx).Exec(fmt.Sprintf(`ALTER TABLE storage_cos_config ALTER COLUMN id RESTART WITH %d`, base)).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}
