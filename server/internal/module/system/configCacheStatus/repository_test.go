package configcachestatus

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestRepositoryListAggregatesOutboxAndFilters(t *testing.T) {
	db, ctx := openConfigCacheDatabase(t)
	seedConfigCacheRows(t, db, ctx)
	repository := NewRepository(db)

	items, total, err := repository.List(ctx, ListQuery{Page: 1, PageSize: 20})
	if err != nil || total != 4 || len(items) != 4 {
		t.Fatalf("list = %d items total=%d err=%v want 4/4", len(items), total, err)
	}
	byScope := make(map[string]Item, len(items))
	for _, item := range items {
		byScope[item.Namespace+"/"+item.ScopeKey] = item
	}

	setting := byScope["system.setting/global"]
	if setting.Generation != 12 || setting.PendingCount != 0 || setting.LatestAttempts != 0 {
		t.Fatalf("setting row = %+v", setting)
	}
	if setting.LatestPublishedGeneration == nil || *setting.LatestPublishedGeneration != 11 || setting.LatestPublishedAt == nil {
		t.Fatalf("setting published aggregate = %+v", setting)
	}

	dictionary := byScope["system.dictionary/global"]
	if dictionary.PendingCount != 2 || dictionary.LatestAttempts != 0 || dictionary.LastError != "" || dictionary.OldestPendingAt == nil {
		t.Fatalf("dictionary row = %+v", dictionary)
	}
	if dictionary.LatestPublishedGeneration != nil {
		t.Fatalf("dictionary must not report published events: %+v", dictionary)
	}

	mail := byScope["system.mail/runtime"]
	if mail.PendingCount != 1 || mail.LatestAttempts != 4 || mail.LastError != "dependency-unavailable" {
		t.Fatalf("mail row = %+v", mail)
	}

	cos := byScope["system.cos/global"]
	if cos.PendingCount != 0 || cos.OldestPendingAt != nil || cos.LatestPublishedGeneration != nil || cos.LatestAttempts != 0 {
		t.Fatalf("generation row without outbox = %+v", cos)
	}

	keywordItems, keywordTotal, err := repository.List(ctx, ListQuery{Page: 1, PageSize: 20, Keyword: "dictionary"})
	if err != nil || keywordTotal != 1 || len(keywordItems) != 1 || keywordItems[0].Namespace != "system.dictionary" {
		t.Fatalf("namespace keyword filter = %+v total=%d err=%v", keywordItems, keywordTotal, err)
	}
	scopeItems, scopeTotal, err := repository.List(ctx, ListQuery{Page: 1, PageSize: 20, Keyword: "runtime"})
	if err != nil || scopeTotal != 1 || len(scopeItems) != 1 || scopeItems[0].ScopeKey != "runtime" {
		t.Fatalf("scope keyword filter = %+v total=%d err=%v", scopeItems, scopeTotal, err)
	}

	for _, test := range []struct {
		publishState string
		wantTotal    int64
		wantScope    string
	}{
		{PublishStateReady, 2, ""},
		{PublishStatePending, 1, "system.dictionary/global"},
		{PublishStateRetrying, 1, "system.mail/runtime"},
	} {
		stateItems, stateTotal, err := repository.List(ctx, ListQuery{Page: 1, PageSize: 20, PublishState: test.publishState})
		if err != nil || stateTotal != test.wantTotal {
			t.Fatalf("publishState %s total = %d err=%v want %d", test.publishState, stateTotal, err, test.wantTotal)
		}
		if test.wantScope != "" {
			if len(stateItems) != 1 || stateItems[0].Namespace+"/"+stateItems[0].ScopeKey != test.wantScope {
				t.Fatalf("publishState %s items = %+v want %s", test.publishState, stateItems, test.wantScope)
			}
		}
	}

	pageItems, pageTotal, err := repository.List(ctx, ListQuery{Page: 2, PageSize: 2})
	if err != nil || pageTotal != 4 || len(pageItems) != 2 {
		t.Fatalf("page 2 = %d items total=%d err=%v", len(pageItems), pageTotal, err)
	}
	if pageItems[0].Namespace != "system.mail" || pageItems[1].Namespace != "system.setting" {
		t.Fatalf("page order = %+v", pageItems)
	}
}

func TestRepositoryResolvesCOSNamesAndKeepsDeletedAndMissingScopesDistinct(t *testing.T) {
	db, ctx := openConfigCacheDatabase(t)
	if err := db.WithContext(ctx).Exec(`
INSERT INTO storage_cos_config (id, name) VALUES (1, '主对象存储');
INSERT INTO storage_cos_config (id, name, deleted_at) VALUES (2, '旧对象存储', CURRENT_TIMESTAMP);
INSERT INTO system_config_cache_state (namespace, scope_key, generation) VALUES
  ('storage.cosconfig', '1', 2), ('storage.cosconfig', '2', 3), ('storage.cosconfig', '99', 4),
  ('storage.cosconfig', 'not-a-number', 1), ('storage.cosconfig', '9223372036854775808', 1);
`).Error; err != nil {
		t.Fatal(err)
	}
	items, total, err := NewRepository(db).List(ctx, ListQuery{Page: 1, PageSize: 20, Keyword: "对象存储"})
	if err != nil || total != 2 || len(items) != 2 {
		t.Fatalf("COS name search = %+v total=%d err=%v", items, total, err)
	}
	byID := make(map[string]Item, len(items))
	for _, item := range items {
		byID[item.ScopeKey] = item
	}
	if byID["1"].ScopeName != "主对象存储" || byID["1"].ScopeDeleted || byID["1"].ScopeMissing {
		t.Fatalf("active COS scope = %+v", byID["1"])
	}
	if byID["2"].ScopeName != "旧对象存储" || !byID["2"].ScopeDeleted || byID["2"].ScopeMissing {
		t.Fatalf("deleted COS scope = %+v", byID["2"])
	}
	missingItems, missingTotal, err := NewRepository(db).List(ctx, ListQuery{Page: 1, PageSize: 20, Keyword: "99"})
	if err != nil || missingTotal != 1 || len(missingItems) != 1 || !missingItems[0].ScopeMissing {
		t.Fatalf("missing COS scope = %+v total=%d err=%v", missingItems, missingTotal, err)
	}
	for _, keyword := range []string{"not-a-number", "9223372036854775808"} {
		rows, total, err := NewRepository(db).List(ctx, ListQuery{Page: 1, PageSize: 20, Keyword: keyword})
		if err != nil || total != 1 || len(rows) != 1 || !rows[0].ScopeMissing {
			t.Fatalf("invalid COS ID %q = %+v total=%d err=%v", keyword, rows, total, err)
		}
	}
}

func openConfigCacheDatabase(t *testing.T) (*gorm.DB, context.Context) {
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
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_system_cache_generation_admin")
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE system_config_cache_state (
	namespace VARCHAR(128) NOT NULL,
	scope_key VARCHAR(128) NOT NULL,
	generation BIGINT NOT NULL DEFAULT 1,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (namespace, scope_key)
);
CREATE TABLE storage_cos_config (
	id BIGINT PRIMARY KEY,
	name VARCHAR(128) NOT NULL,
	deleted_at TIMESTAMPTZ
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
	CONSTRAINT uq_system_config_cache_outbox_generation UNIQUE (namespace, scope_key, generation)
);
`).Error; err != nil {
		t.Fatalf("prepare cache generation schema: %v", err)
	}
	return db, ctx
}

func seedConfigCacheRows(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	statements := []string{
		`INSERT INTO system_config_cache_state (namespace, scope_key, generation, created_at, updated_at) VALUES
			('system.setting', 'global', 12, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('system.dictionary', 'global', 5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('system.mail', 'runtime', 3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('system.cos', 'global', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		`INSERT INTO system_config_cache_outbox (namespace, scope_key, generation, attempts, published_at, created_at, updated_at) VALUES
			('system.setting', 'global', 11, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		`INSERT INTO system_config_cache_outbox (namespace, scope_key, generation, attempts, created_at, updated_at) VALUES
			('system.dictionary', 'global', 4, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('system.dictionary', 'global', 5, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		`INSERT INTO system_config_cache_outbox (namespace, scope_key, generation, attempts, last_error, created_at, updated_at) VALUES
			('system.mail', 'runtime', 3, 4, 'dependency-unavailable', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
	}
	for _, statement := range statements {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("seed cache generation rows: %v", err)
		}
	}
}
