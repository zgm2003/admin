package setting

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestRepositoryMutationAdvancesGenerationWithOutbox(t *testing.T) {
	db, ctx := openSettingDatabase(t)
	seedSettingRow(t, db, ctx, numericSettingRow("auth.captcha.ttl_minutes", "2", yesno.No))
	repository := NewRepository(db)
	repository.SetGenerations(cachegeneration.NewRepository(db))
	now := time.Now().UTC()

	result, err := repository.Update(ctx, "auth.captcha.ttl_minutes", Record{
		Key: "auth.captcha.ttl_minutes", Value: "5", ValueType: ValueTypeNumber, Description: "", IsEnabled: yesno.Yes, IsBuiltin: yesno.No, UpdatedAt: now,
	}, 1)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !result.Changed || result.Generation != 2 || result.OutboxID < 1 {
		t.Fatalf("mutation result = %+v", result)
	}
	assertSettingGeneration(t, db, ctx, "global", 2)
	assertSettingOutbox(t, db, ctx, "global", 1, 2, false)

	var row Model
	if err := db.WithContext(ctx).Where("setting_key = ?", "auth.captcha.ttl_minutes").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Value != "5" {
		t.Fatalf("setting value = %q want 5", row.Value)
	}
}

func TestRepositoryMutationNoOpKeepsGenerationAndOutbox(t *testing.T) {
	db, ctx := openSettingDatabase(t)
	seedSettingRow(t, db, ctx, numericSettingRow("auth.captcha.ttl_minutes", "2", yesno.No))
	repository := NewRepository(db)
	repository.SetGenerations(cachegeneration.NewRepository(db))

	result, err := repository.UpdateStatus(ctx, "auth.captcha.ttl_minutes", yesno.Yes, 1, time.Now().UTC())
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if result.Changed {
		t.Fatalf("no-op mutation result = %+v want unchanged", result)
	}
	assertSettingGeneration(t, db, ctx, "global", 1)
	assertSettingOutbox(t, db, ctx, "global", 0, 0, false)
}

func TestRepositoryMutationRollsBackGenerationOnConflict(t *testing.T) {
	db, ctx := openSettingDatabase(t)
	seedSettingRow(t, db, ctx, numericSettingRow("auth.captcha.ttl_minutes", "2", yesno.No))
	repository := NewRepository(db)
	repository.SetGenerations(cachegeneration.NewRepository(db))
	now := time.Now().UTC()

	_, err := repository.Create(ctx, &Record{
		Key: "auth.captcha.ttl_minutes", Value: "5", ValueType: ValueTypeNumber, Description: "", IsEnabled: yesno.Yes, IsBuiltin: yesno.No, CreatedAt: now, UpdatedAt: now,
	}, 1)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Create() error = %v want ErrConflict", err)
	}
	assertSettingGeneration(t, db, ctx, "global", 1)
	assertSettingOutbox(t, db, ctx, "global", 0, 0, false)
}

func TestRepositoryMutationRejectsStaleLeaseBaseGeneration(t *testing.T) {
	db, ctx := openSettingDatabase(t)
	seedSettingRow(t, db, ctx, numericSettingRow("auth.captcha.ttl_minutes", "2", yesno.No))
	repository := NewRepository(db)
	repository.SetGenerations(cachegeneration.NewRepository(db))

	_, err := repository.Update(ctx, "auth.captcha.ttl_minutes", Record{
		Key: "auth.captcha.ttl_minutes", Value: "5", ValueType: ValueTypeNumber, Description: "", IsEnabled: yesno.Yes, IsBuiltin: yesno.No, UpdatedAt: time.Now().UTC(),
	}, 2)
	if !errors.Is(err, cachegeneration.ErrGenerationChanged) {
		t.Fatalf("Update() error = %v want ErrGenerationChanged", err)
	}
	assertSettingGeneration(t, db, ctx, "global", 1)
	assertSettingOutbox(t, db, ctx, "global", 0, 0, false)
}

func TestRepositoryBrandMutationAdvancesGenerationOnce(t *testing.T) {
	db, ctx := openSettingDatabase(t)
	now := time.Now().UTC()
	for _, row := range []Model{
		{Key: BrandTitleZhCNKey, Value: "智澜", ValueType: ValueTypeString, Description: "", IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes, CreatedAt: now, UpdatedAt: now},
		{Key: BrandTitleEnUSKey, Value: "ZHILAN", ValueType: ValueTypeString, Description: "", IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes, CreatedAt: now, UpdatedAt: now},
		{Key: BrandDefaultAvatarKey, Value: "", ValueType: ValueTypeString, Description: "", IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes, CreatedAt: now, UpdatedAt: now},
	} {
		seedSettingRow(t, db, ctx, row)
	}
	repository := NewRepository(db)
	repository.SetGenerations(cachegeneration.NewRepository(db))

	result, err := repository.UpdateBrand(ctx, BrandSettings{TitleZhCN: "新标题", TitleEnUS: "NEW", DefaultAvatar: ""}, 1, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("UpdateBrand() error = %v", err)
	}
	if !result.Changed || result.Generation != 2 || result.OutboxID < 1 {
		t.Fatalf("brand mutation result = %+v", result)
	}
	assertSettingGeneration(t, db, ctx, "global", 2)
	assertSettingOutbox(t, db, ctx, "global", 1, 2, false)
}

type settingGenerationHarness struct {
	ctx         context.Context
	db          *gorm.DB
	client      *projectredis.Client
	scope       cachegeneration.Scope
	store       *cachegeneration.Store
	generations *cachegeneration.Repository
	cache       *Cache
}

func openSettingGenerationHarness(t *testing.T) settingGenerationHarness {
	t.Helper()
	scopeKey := "test-" + time.Now().UTC().Format("20060102150405.000000000")
	db, ctx := openSettingSchema(t, "test_system_setting_generation", scopeKey)
	scope, err := cachegeneration.NewScope("system.setting", scopeKey)
	if err != nil {
		t.Fatal(err)
	}
	settings := loadSettingTestSettings(t)
	client, err := projectredis.Open(ctx, settings.RedisURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = client.Delete(context.Background(), cachegeneration.StateKey(scope))
		_ = client.Close()
	})
	store := cachegeneration.NewStore(client)
	if _, err := store.Reconcile(ctx, scope, 1); err != nil {
		t.Fatal(err)
	}
	cache := NewCache(client)
	cache.scope = scope
	cache.SetStateStore(store)
	return settingGenerationHarness{
		ctx: ctx, db: db, client: client, scope: scope, store: store,
		generations: cachegeneration.NewRepository(db),
		cache:       cache,
	}
}

func (h settingGenerationHarness) service(settingsRepository repository) *Service {
	service := NewService(settingsRepository)
	service.scope = h.scope
	service.renewInterval = 10 * time.Millisecond
	service.readBudget = settingReadBudget
	service.SetGenerations(h.generations, h.store)
	service.SetCache(h.cache)
	return service
}

func loadSettingTestSettings(t *testing.T) config.Worker {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test")
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", ".."))
	if err := godotenv.Load(filepath.Join(repoRoot, "server", ".env")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	return settings
}

func openSettingDatabase(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	return openSettingSchema(t, "test_system_setting_repository", "global")
}

func openSettingSchema(t *testing.T, prefix string, scopeKey string) (*gorm.DB, context.Context) {
	t.Helper()
	settings := loadSettingTestSettings(t)
	db, ctx := testschema.Open(t, settings.PostgresDSN, prefix)
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE system_setting (
	id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
	setting_key VARCHAR(128) NOT NULL,
	value TEXT NOT NULL,
	value_type SMALLINT NOT NULL,
	description VARCHAR(512) NOT NULL,
	is_enabled SMALLINT NOT NULL,
	is_builtin SMALLINT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX ux_system_setting_key_active ON system_setting(setting_key) WHERE deleted_at IS NULL;
CREATE TABLE system_config_cache_generation (
	namespace VARCHAR(128) NOT NULL,
	scope_key VARCHAR(128) NOT NULL,
	generation BIGINT NOT NULL DEFAULT 1,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (namespace, scope_key),
	CONSTRAINT ck_system_config_cache_generation_value CHECK (generation >= 1)
);
CREATE TABLE system_config_cache_outbox (
	id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
	namespace VARCHAR(128) NOT NULL,
	scope_key VARCHAR(128) NOT NULL,
	generation BIGINT NOT NULL,
	attempts INTEGER NOT NULL DEFAULT 0,
	available_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	locked_until TIMESTAMPTZ,
	lock_token VARCHAR(64),
	last_error VARCHAR(512) NOT NULL DEFAULT '',
	published_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_system_config_cache_outbox_generation UNIQUE (namespace, scope_key, generation),
	CONSTRAINT fk_system_config_cache_outbox_generation
		FOREIGN KEY (namespace, scope_key) REFERENCES system_config_cache_generation(namespace, scope_key),
	CONSTRAINT ck_system_config_cache_outbox_published_lock
		CHECK (published_at IS NULL OR (locked_until IS NULL AND lock_token IS NULL))
);
`).Error; err != nil {
		t.Fatalf("prepare setting schema: %v", err)
	}
	if err := db.WithContext(ctx).Exec(
		`INSERT INTO system_config_cache_generation (namespace, scope_key, generation) VALUES ('system.setting', ?, 1)`,
		scopeKey).Error; err != nil {
		t.Fatalf("seed setting generation: %v", err)
	}
	return db, ctx
}

func seedSettingRow(t *testing.T, db *gorm.DB, ctx context.Context, row Model) {
	t.Helper()
	if err := db.WithContext(ctx).Create(&row).Error; err != nil {
		t.Fatalf("seed setting row: %v", err)
	}
}

func numericSettingRow(key, value string, builtin yesno.Value) Model {
	now := time.Now().UTC()
	return Model{
		Key: key, Value: value, ValueType: ValueTypeNumber, Description: "", IsEnabled: yesno.Yes, IsBuiltin: builtin, CreatedAt: now, UpdatedAt: now,
	}
}

func assertSettingGeneration(t *testing.T, db *gorm.DB, ctx context.Context, scopeKey string, want int64) {
	t.Helper()
	var generation int64
	if err := db.WithContext(ctx).Raw(
		`SELECT generation FROM system_config_cache_generation WHERE namespace = 'system.setting' AND scope_key = ?`,
		scopeKey).Scan(&generation).Error; err != nil {
		t.Fatal(err)
	}
	if generation != want {
		t.Fatalf("generation = %d want %d", generation, want)
	}
}

func assertSettingOutbox(t *testing.T, db *gorm.DB, ctx context.Context, scopeKey string, wantCount int, wantGeneration int64, wantPublished bool) {
	t.Helper()
	var count int64
	if err := db.WithContext(ctx).Raw(
		`SELECT count(*) FROM system_config_cache_outbox WHERE namespace = 'system.setting' AND scope_key = ?`,
		scopeKey).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != int64(wantCount) {
		t.Fatalf("outbox count = %d want %d", count, wantCount)
	}
	if wantCount == 0 {
		return
	}
	var row struct {
		Generation  int64
		PublishedAt *time.Time
	}
	if err := db.WithContext(ctx).Raw(
		`SELECT generation, published_at FROM system_config_cache_outbox WHERE namespace = 'system.setting' AND scope_key = ? ORDER BY id LIMIT 1`,
		scopeKey).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Generation != wantGeneration {
		t.Fatalf("outbox generation = %d want %d", row.Generation, wantGeneration)
	}
	if (row.PublishedAt != nil) != wantPublished {
		t.Fatalf("outbox published = %v want %v", row.PublishedAt != nil, wantPublished)
	}
}

func assertSettingOutboxGenerations(t *testing.T, db *gorm.DB, ctx context.Context, scopeKey string, want []int64) {
	t.Helper()
	var got []int64
	if err := db.WithContext(ctx).Raw(
		`SELECT generation FROM system_config_cache_outbox WHERE namespace = 'system.setting' AND scope_key = ? ORDER BY generation`,
		scopeKey).Scan(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("outbox generations = %v want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("outbox generations = %v want %v", got, want)
		}
	}
}
