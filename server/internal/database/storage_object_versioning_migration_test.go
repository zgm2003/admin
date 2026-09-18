package database_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"admin/server/internal/database/testschema"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// storageObjectVersioningNamespace 必须同时满足 Go cacheGeneration namespacePattern 与
// PostgreSQL ck_system_config_cache_generation_namespace（同一正则，禁止大写）。
const storageObjectVersioningNamespace = "storage.cosconfig"

const storageObjectVersioningKeyA = "avatar/2026/08/31/6e7e53334a6da066b130a89cc3f69535.png"
const storageObjectVersioningKeyB = "file/2026/09/01/0f1e2d3c4b5a69788796a5b4c3d2e1f0.png"

func TestStorageObjectVersioningMigration(t *testing.T) {
	db, ctx := openStorageObjectVersioningSchema(t)
	seedStorageObjectVersioningFixture(t, db, ctx)
	script := readStorageObjectVersioningMigration(t)

	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("execute migration: %v", err)
	}

	assertStorageObjectVersioningStructure(t, db, ctx)
	assertStorageObjectVersioningBackfill(t, db, ctx)
	assertStorageObjectVersioningClearedReferences(t, db, ctx)
	assertStorageObjectVersioningConstraintBehaviour(t, db, ctx)

	adminVersion, canvasVersion := readCacheGenerationMenuVersions(t, db, ctx)
	if adminVersion != 7 || canvasVersion != 3 {
		t.Fatalf("menu versions admin=%d canvas=%d want 7/3", adminVersion, canvasVersion)
	}

	// 幂等：第二次执行不得新增版本/outbox，也不得再次推进 generation 或清空引用。
	codeRowsBefore := readStorageTableCount(t, db, ctx, "storage_upload_rule_code")
	outboxRowsBefore := readStorageTableCount(t, db, ctx, "system_config_cache_outbox")
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("re-execute migration: %v", err)
	}
	assertCount(t, db, ctx, "storage_cos_config_version", 3)
	if after := readStorageTableCount(t, db, ctx, "storage_upload_rule_code"); after != codeRowsBefore {
		t.Fatalf("code rows changed on re-run: %d -> %d", codeRowsBefore, after)
	}
	if after := readStorageTableCount(t, db, ctx, "system_config_cache_outbox"); after != outboxRowsBefore {
		t.Fatalf("outbox rows changed on re-run: %d -> %d", outboxRowsBefore, after)
	}
	assertStorageObjectVersioningGenerations(t, db, ctx, 1)
	assertSystemSettingGeneration(t, db, ctx, 2, 1)
	assertStorageObjectVersioningClearedReferences(t, db, ctx)
	adminVersion, canvasVersion = readCacheGenerationMenuVersions(t, db, ctx)
	if adminVersion != 7 || canvasVersion != 3 {
		t.Fatalf("menu versions after re-run admin=%d canvas=%d want 7/3", adminVersion, canvasVersion)
	}
}

func TestStorageObjectVersioningMigrationRejectsMultipleEnabledRules(t *testing.T) {
	db, ctx := openStorageObjectVersioningSchema(t)
	seedStorageObjectVersioningFixture(t, db, ctx)
	if err := db.WithContext(ctx).Exec(`INSERT INTO storage_upload_rule (id, platform_id, name, cos_config_id, max_file_size_bytes, allowed_extensions, allowed_mime_types, access_mode, is_enabled, remark, created_at, updated_at)
		VALUES (23, 1, '第二条启用规则', 10, 1024, '{png}'::text[], '{image/png}'::text[], 'private', 1, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error; err != nil {
		t.Fatalf("seed second enabled rule: %v", err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO storage_upload_rule_code (id, rule_id, platform_id, code, created_at) VALUES (34, 23, 1, 'extra', CURRENT_TIMESTAMP)`).Error; err != nil {
		t.Fatalf("seed second rule code: %v", err)
	}

	err := db.WithContext(ctx).Exec(readStorageObjectVersioningMigration(t)).Error
	if err == nil {
		t.Fatal("migration accepted two enabled rules on one platform")
	}
	if !strings.Contains(err.Error(), "platform 1") {
		t.Fatalf("error = %v, want platform id in message", err)
	}
	assertStorageObjectVersioningRolledBack(t, db, ctx)
}

func TestStorageObjectVersioningMigrationRejectsReservedCode(t *testing.T) {
	db, ctx := openStorageObjectVersioningSchema(t)
	seedStorageObjectVersioningFixture(t, db, ctx)
	if err := db.WithContext(ctx).Exec(`INSERT INTO storage_upload_rule_code (id, rule_id, platform_id, code, created_at) VALUES (34, 21, 1, 'evil/.admin-storage/thing', CURRENT_TIMESTAMP)`).Error; err != nil {
		t.Fatalf("seed reserved code: %v", err)
	}

	err := db.WithContext(ctx).Exec(readStorageObjectVersioningMigration(t)).Error
	if err == nil {
		t.Fatal("migration accepted a code that occupies the internal marker")
	}
	if !strings.Contains(err.Error(), ".admin-storage") {
		t.Fatalf("error = %v, want marker in message", err)
	}
	assertStorageObjectVersioningRolledBack(t, db, ctx)
}

func assertStorageObjectVersioningStructure(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	assertTableExists(t, db, ctx, "storage_cos_config_version")
	for _, column := range []string{"bucket", "region", "endpoint", "bucket_domain"} {
		assertColumnMissing(t, db, ctx, "storage_cos_config", column)
	}
	assertColumnExists(t, db, ctx, "storage_cos_config", "current_version")
	assertColumnMissing(t, db, ctx, "storage_cos_config", "revision")
	assertColumnMissing(t, db, ctx, "storage_upload_rule", "revision")
	assertColumnMissing(t, db, ctx, "storage_upload_rule_code", "platform_id")
	assertColumnExists(t, db, ctx, "storage_upload_rule_code", "updated_at")
	assertTableMissing(t, db, ctx, "storage_object_legacy_route")
	assertForeignKey(t, db, ctx, "fk_storage_cos_config_version_config")
	assertForeignKey(t, db, ctx, "fk_storage_cos_config_current_version")
	assertStorageConstraintDefinition(t, db, ctx, "pk_storage_cos_config_version", "PRIMARY KEY (cos_config_id, version)")
	assertStorageConstraintMissing(t, db, ctx, "fk_storage_upload_rule_code_platform")
	assertStorageIndexMissing(t, db, ctx, "ux_storage_upload_rule_code_platform_code")
	assertStorageIndexDefinition(t, db, ctx, "ux_storage_upload_rule_code_rule_code", "WHERE (deleted_at IS NULL)")
	assertStorageIndexDefinition(t, db, ctx, "ux_storage_upload_rule_platform_enabled", "(platform_id)")
	assertStorageIndexDefinition(t, db, ctx, "ux_storage_upload_rule_platform_enabled", "is_enabled = 1")
	assertStorageIndexDefinition(t, db, ctx, "ux_storage_upload_rule_platform_enabled", "deleted_at IS NULL")
}

func assertStorageObjectVersioningBackfill(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	cases := []struct {
		configID     int64
		bucket       string
		region       string
		endpoint     *string
		bucketDomain *string
	}{
		{10, "bucket-a", "ap-guangzhou", strPtr("https://cos-internal.example.com"), strPtr("https://cdn-a.example.com")},
		{11, "bucket-b", "ap-shanghai", nil, strPtr("https://cdn-b.example.com")},
		{12, "bucket-c", "ap-nanjing", nil, nil},
	}
	for _, test := range cases {
		var row struct {
			Bucket       string
			Region       string
			Endpoint     *string
			BucketDomain *string
		}
		if err := db.WithContext(ctx).Raw(`SELECT bucket, region, endpoint, bucket_domain FROM storage_cos_config_version WHERE cos_config_id=? AND version=1`, test.configID).Scan(&row).Error; err != nil {
			t.Fatal(err)
		}
		if row.Bucket != test.bucket || row.Region != test.region ||
			!sameOptionalString(row.Endpoint, test.endpoint) || !sameOptionalString(row.BucketDomain, test.bucketDomain) {
			t.Fatalf("config %d version row = %+v", test.configID, row)
		}
		var currentVersion int64
		if err := db.WithContext(ctx).Raw(`SELECT current_version FROM storage_cos_config WHERE id=?`, test.configID).Scan(&currentVersion).Error; err != nil {
			t.Fatal(err)
		}
		if currentVersion != 1 {
			t.Fatalf("config %d current_version=%d want=1", test.configID, currentVersion)
		}
	}
	assertCount(t, db, ctx, "storage_cos_config_version", 3)

	var backfilled int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM storage_upload_rule_code WHERE id=30 AND updated_at=created_at`).Scan(&backfilled).Error; err != nil {
		t.Fatal(err)
	}
	if backfilled != 1 {
		t.Fatal("code row updated_at was not backfilled from created_at")
	}

	assertStorageObjectVersioningGenerations(t, db, ctx, 1)
	assertSystemSettingGeneration(t, db, ctx, 2, 1)
}

func assertStorageObjectVersioningGenerations(t *testing.T, db *gorm.DB, ctx context.Context, wantStoredGeneration int64) {
	t.Helper()
	for _, scopeKey := range []string{"10", "11", "12"} {
		var generation int64
		if err := db.WithContext(ctx).Raw(`SELECT generation FROM system_config_cache_generation WHERE namespace=? AND scope_key=?`, storageObjectVersioningNamespace, scopeKey).Scan(&generation).Error; err != nil {
			t.Fatal(err)
		}
		if generation != wantStoredGeneration {
			t.Fatalf("storage.cosconfig/%s generation=%d want=%d", scopeKey, generation, wantStoredGeneration)
		}
		var outbox int64
		if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM system_config_cache_outbox WHERE namespace=? AND scope_key=? AND generation=?`, storageObjectVersioningNamespace, scopeKey, wantStoredGeneration).Scan(&outbox).Error; err != nil {
			t.Fatal(err)
		}
		if outbox != 1 {
			t.Fatalf("storage.cosconfig/%s outbox generation=%d count=%d want=1", scopeKey, wantStoredGeneration, outbox)
		}
	}
	var total int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM system_config_cache_generation WHERE namespace=?`, storageObjectVersioningNamespace).Scan(&total).Error; err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("storage.cosconfig generation rows=%d want=3", total)
	}
}

func assertSystemSettingGeneration(t *testing.T, db *gorm.DB, ctx context.Context, wantGeneration int64, wantOutbox int64) {
	t.Helper()
	var generation int64
	if err := db.WithContext(ctx).Raw(`SELECT generation FROM system_config_cache_generation WHERE namespace='system.setting' AND scope_key='global'`).Scan(&generation).Error; err != nil {
		t.Fatal(err)
	}
	if generation != wantGeneration {
		t.Fatalf("system.setting generation=%d want=%d", generation, wantGeneration)
	}
	var outbox int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM system_config_cache_outbox WHERE namespace='system.setting' AND scope_key='global' AND generation=?`, wantGeneration).Scan(&outbox).Error; err != nil {
		t.Fatal(err)
	}
	if outbox != wantOutbox {
		t.Fatalf("system.setting generation=%d outbox count=%d want=%d", wantGeneration, outbox, wantOutbox)
	}
}

func assertStorageObjectVersioningClearedReferences(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	var nonEmpty int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM user_profile WHERE avatar <> ''`).Scan(&nonEmpty).Error; err != nil {
		t.Fatal(err)
	}
	if nonEmpty != 0 {
		t.Fatalf("non empty user avatars=%d want=0", nonEmpty)
	}
	var brand string
	if err := db.WithContext(ctx).Raw(`SELECT value FROM system_setting WHERE setting_key='app.brand.default_avatar' AND deleted_at IS NULL`).Scan(&brand).Error; err != nil {
		t.Fatal(err)
	}
	if brand != "" {
		t.Fatalf("brand default avatar=%q want empty", brand)
	}
	var untouched int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM system_setting WHERE setting_key='app.brand.title_zh_cn' AND value='智澜'`).Scan(&untouched).Error; err != nil {
		t.Fatal(err)
	}
	if untouched != 1 {
		t.Fatal("unrelated system setting was modified")
	}
}

func assertStorageObjectVersioningRolledBack(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	assertTableMissing(t, db, ctx, "storage_cos_config_version")
	assertColumnMissing(t, db, ctx, "storage_cos_config", "current_version")
	assertColumnExists(t, db, ctx, "storage_cos_config", "bucket")
	assertColumnExists(t, db, ctx, "storage_upload_rule_code", "platform_id")
	var generationRows int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM system_config_cache_generation WHERE namespace=?`, storageObjectVersioningNamespace).Scan(&generationRows).Error; err != nil {
		t.Fatal(err)
	}
	if generationRows != 0 {
		t.Fatalf("storage.cosconfig generation rows=%d want=0 after rollback", generationRows)
	}
	assertSystemSettingGeneration(t, db, ctx, 1, 0)
	var nonEmpty int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM user_profile WHERE avatar <> ''`).Scan(&nonEmpty).Error; err != nil {
		t.Fatal(err)
	}
	if nonEmpty != 1 {
		t.Fatalf("non empty user avatars=%d want=1 after rollback", nonEmpty)
	}
	var brand string
	if err := db.WithContext(ctx).Raw(`SELECT value FROM system_setting WHERE setting_key='app.brand.default_avatar' AND deleted_at IS NULL`).Scan(&brand).Error; err != nil {
		t.Fatal(err)
	}
	if brand != storageObjectVersioningKeyB {
		t.Fatalf("brand default avatar=%q want original value", brand)
	}
}

func assertStorageObjectVersioningConstraintBehaviour(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	expectStorageStatementFailure(t, db, ctx,
		`INSERT INTO storage_upload_rule (id, platform_id, name, cos_config_id, max_file_size_bytes, allowed_extensions, allowed_mime_types, access_mode, is_enabled, remark, created_at, updated_at)
			VALUES (90, 1, '第二条启用', 10, 1024, '{png}'::text[], '{image/png}'::text[], 'private', 1, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"23505", "ux_storage_upload_rule_platform_enabled")
	expectStorageStatementFailure(t, db, ctx,
		`INSERT INTO storage_upload_rule_code (id, rule_id, code, created_at, updated_at) VALUES (90, 20, 'avatar', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"23505", "ux_storage_upload_rule_code_rule_code")
	if err := db.WithContext(ctx).Exec(`INSERT INTO storage_upload_rule_code (id, rule_id, code, created_at, updated_at) VALUES (91, 21, 'file', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error; err != nil {
		t.Fatalf("cross rule duplicate code must be accepted: %v", err)
	}
	expectStorageStatementFailure(t, db, ctx,
		`INSERT INTO storage_upload_rule_code (id, rule_id, code, created_at, updated_at) VALUES (92, 20, 'BadCode', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"23514", "ck_storage_upload_rule_code_value")
	expectStorageStatementFailure(t, db, ctx,
		`INSERT INTO storage_upload_rule_code (id, rule_id, code, created_at, updated_at) VALUES (93, 20, 'evil/.admin-storage/thing', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"23514", "ck_storage_upload_rule_code_value")
	expectStorageStatementFailure(t, db, ctx,
		`UPDATE storage_cos_config SET current_version = 99 WHERE id = 10`,
		"23503", "fk_storage_cos_config_current_version")
	expectStorageStatementFailure(t, db, ctx,
		`DELETE FROM storage_cos_config_version WHERE cos_config_id = 10 AND version = 1`,
		"23503", "fk_storage_cos_config_current_version")
	expectStorageStatementFailure(t, db, ctx,
		`DELETE FROM storage_cos_config WHERE id = 10`,
		"23001", "")
	expectStorageStatementFailure(t, db, ctx,
		`DELETE FROM permission_auth_platform WHERE id = 1`,
		"23001", "")
	expectStorageStatementFailure(t, db, ctx,
		`INSERT INTO storage_cos_config_version (cos_config_id, version, bucket, region) VALUES (10, 0, 'bucket', 'region')`,
		"23514", "ck_storage_cos_config_version_value")
	expectStorageStatementFailure(t, db, ctx,
		`UPDATE storage_cos_config SET current_version = 0 WHERE id = 10`,
		"23514", "ck_storage_cos_config_current_version")
	expectStorageStatementFailure(t, db, ctx,
		`UPDATE storage_cos_config SET is_enabled = 7 WHERE id = 10`,
		"23514", "ck_storage_cos_config_is_enabled")
	expectStorageStatementFailure(t, db, ctx,
		`UPDATE storage_upload_rule SET access_mode = 'weird' WHERE id = 20`,
		"23514", "ck_storage_upload_rule_access_mode")
}

func expectStorageStatementFailure(t *testing.T, db *gorm.DB, ctx context.Context, statement, code, constraint string) {
	t.Helper()
	err := db.WithContext(ctx).Exec(statement).Error
	if err == nil {
		t.Fatalf("statement was accepted, want sqlstate %s: %s", code, statement)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("error %v is not a PgError", err)
	}
	if pgErr.Code != code {
		t.Fatalf("sqlstate=%s want=%s: %v", pgErr.Code, code, err)
	}
	if constraint != "" && pgErr.ConstraintName != constraint {
		t.Fatalf("constraint=%s want=%s: %v", pgErr.ConstraintName, constraint, err)
	}
}

func assertStorageConstraintDefinition(t *testing.T, db *gorm.DB, ctx context.Context, name, wantSubstring string) {
	t.Helper()
	var definition string
	if err := db.WithContext(ctx).Raw(`SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname=? AND connamespace=current_schema()::regnamespace`, name).Scan(&definition).Error; err != nil {
		t.Fatal(err)
	}
	if definition == "" || !strings.Contains(definition, wantSubstring) {
		t.Fatalf("constraint %s definition=%q want contains %q", name, definition, wantSubstring)
	}
}

func assertStorageConstraintMissing(t *testing.T, db *gorm.DB, ctx context.Context, name string) {
	t.Helper()
	var exists bool
	if err := db.WithContext(ctx).Raw(`SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname=? AND connamespace=current_schema()::regnamespace)`, name).Scan(&exists).Error; err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatalf("constraint %s exists but must be dropped", name)
	}
}

func assertStorageIndexDefinition(t *testing.T, db *gorm.DB, ctx context.Context, name, wantSubstring string) {
	t.Helper()
	var definition string
	if err := db.WithContext(ctx).Raw(`SELECT indexdef FROM pg_indexes WHERE schemaname=current_schema() AND indexname=?`, name).Scan(&definition).Error; err != nil {
		t.Fatal(err)
	}
	if definition == "" || !strings.Contains(definition, wantSubstring) {
		t.Fatalf("index %s definition=%q want contains %q", name, definition, wantSubstring)
	}
}

func assertStorageIndexMissing(t *testing.T, db *gorm.DB, ctx context.Context, name string) {
	t.Helper()
	var exists bool
	if err := db.WithContext(ctx).Raw(`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname=current_schema() AND indexname=?)`, name).Scan(&exists).Error; err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatalf("index %s exists but must be dropped", name)
	}
}

func readStorageObjectVersioningMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-09-17-storage-object-versioning.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) == "" {
		t.Fatal("storage object versioning migration is empty")
	}
	return string(data)
}

func openStorageObjectVersioningSchema(t *testing.T) (*gorm.DB, context.Context) {
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_storage_object_versioning")
	for _, statement := range []string{
		`CREATE TABLE permission_auth_platform (
			id BIGINT PRIMARY KEY,
			code VARCHAR(49) NOT NULL,
			is_enabled SMALLINT NOT NULL DEFAULT 1,
			menu_version BIGINT NOT NULL DEFAULT 1,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMPTZ NULL)`,
		`CREATE TABLE storage_cos_config (
			id BIGINT PRIMARY KEY,
			name VARCHAR(128) NOT NULL,
			app_id VARCHAR(32) NOT NULL,
			secret_id_ciphertext TEXT NOT NULL,
			secret_key_ciphertext TEXT NOT NULL,
			bucket VARCHAR(128) NOT NULL,
			region VARCHAR(64) NOT NULL,
			endpoint VARCHAR(255) NULL,
			bucket_domain VARCHAR(255) NULL,
			is_enabled SMALLINT NOT NULL DEFAULT 1,
			remark VARCHAR(512) NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMPTZ NULL,
			CONSTRAINT ck_storage_cos_config_is_enabled CHECK (is_enabled IN (0, 1)))`,
		`CREATE UNIQUE INDEX ux_storage_cos_config_name_active ON storage_cos_config (lower(name)) WHERE deleted_at IS NULL`,
		`CREATE TABLE storage_upload_rule (
			id BIGINT PRIMARY KEY,
			platform_id BIGINT NOT NULL,
			name VARCHAR(128) NOT NULL,
			cos_config_id BIGINT NOT NULL,
			max_file_size_bytes BIGINT NOT NULL,
			allowed_extensions TEXT[] NOT NULL,
			allowed_mime_types TEXT[] NOT NULL,
			access_mode VARCHAR(16) NOT NULL,
			is_enabled SMALLINT NOT NULL DEFAULT 1,
			remark VARCHAR(512) NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMPTZ NULL,
			CONSTRAINT fk_storage_upload_rule_platform FOREIGN KEY (platform_id) REFERENCES permission_auth_platform(id) ON DELETE RESTRICT,
			CONSTRAINT fk_storage_upload_rule_cos_config FOREIGN KEY (cos_config_id) REFERENCES storage_cos_config(id) ON DELETE RESTRICT,
			CONSTRAINT ck_storage_upload_rule_access_mode CHECK (access_mode IN ('private', 'public')),
			CONSTRAINT ck_storage_upload_rule_is_enabled CHECK (is_enabled IN (0, 1)))`,
		`CREATE INDEX ix_storage_upload_rule_config_enabled_created_at ON storage_upload_rule (cos_config_id, is_enabled, created_at DESC) WHERE deleted_at IS NULL`,
		`CREATE TABLE storage_upload_rule_code (
			id BIGINT PRIMARY KEY,
			rule_id BIGINT NOT NULL,
			platform_id BIGINT NOT NULL,
			code VARCHAR(64) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMPTZ NULL,
			CONSTRAINT fk_storage_upload_rule_code_rule FOREIGN KEY (rule_id) REFERENCES storage_upload_rule(id) ON DELETE CASCADE,
			CONSTRAINT fk_storage_upload_rule_code_platform FOREIGN KEY (platform_id) REFERENCES permission_auth_platform(id) ON DELETE RESTRICT,
			CONSTRAINT ck_storage_upload_rule_code_value CHECK (length(btrim(code)) > 0))`,
		`CREATE INDEX ix_storage_upload_rule_code_rule ON storage_upload_rule_code (rule_id, id) WHERE deleted_at IS NULL`,
		`CREATE TABLE system_setting (
			id BIGINT PRIMARY KEY,
			setting_key VARCHAR(128) NOT NULL,
			value TEXT NOT NULL,
			value_type SMALLINT NOT NULL DEFAULT 1,
			description VARCHAR(255) NOT NULL DEFAULT '',
			is_enabled SMALLINT NOT NULL DEFAULT 1,
			is_builtin SMALLINT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMPTZ NULL)`,
		`CREATE TABLE user_profile (
			user_id BIGINT PRIMARY KEY,
			birthday DATE NULL,
			gender SMALLINT NOT NULL DEFAULT 0,
			avatar VARCHAR(512) NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE system_config_cache_generation (
			namespace VARCHAR(128) NOT NULL,
			scope_key VARCHAR(128) NOT NULL,
			generation BIGINT NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT pk_system_config_cache_generation PRIMARY KEY (namespace, scope_key),
			CONSTRAINT ck_system_config_cache_generation_namespace CHECK (namespace ~ '^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$'),
			CONSTRAINT ck_system_config_cache_generation_scope_key CHECK (scope_key ~ '^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,127}$'),
			CONSTRAINT ck_system_config_cache_generation_value CHECK (generation >= 1))`,
		`CREATE TABLE system_config_cache_outbox (
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
			CONSTRAINT fk_system_config_cache_outbox_generation FOREIGN KEY (namespace, scope_key) REFERENCES system_config_cache_generation(namespace, scope_key),
			CONSTRAINT ck_system_config_cache_outbox_namespace CHECK (namespace ~ '^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$'),
			CONSTRAINT ck_system_config_cache_outbox_scope_key CHECK (scope_key ~ '^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,127}$'),
			CONSTRAINT ck_system_config_cache_outbox_generation_value CHECK (generation >= 1),
			CONSTRAINT ck_system_config_cache_outbox_attempts CHECK (attempts >= 0),
			CONSTRAINT ck_system_config_cache_outbox_lock CHECK ((locked_until IS NULL AND lock_token IS NULL) OR (locked_until IS NOT NULL AND lock_token IS NOT NULL)),
			CONSTRAINT ck_system_config_cache_outbox_published_lock CHECK (published_at IS NULL OR (locked_until IS NULL AND lock_token IS NULL)))`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("prepare storage object versioning schema: %v", err)
		}
	}
	return db, ctx
}

func seedStorageObjectVersioningFixture(t *testing.T, db *gorm.DB, ctx context.Context) {
	for _, statement := range []string{
		`INSERT INTO permission_auth_platform (id, code, is_enabled, menu_version) VALUES (1, 'admin', 1, 7), (2, 'canvas', 1, 3)`,
		`INSERT INTO storage_cos_config (id, name, app_id, secret_id_ciphertext, secret_key_ciphertext, bucket, region, endpoint, bucket_domain, is_enabled, remark, created_at, updated_at) VALUES
			(10, '主配置', '1250000000', 'v1:cipher-id-10', 'v1:cipher-key-10', 'bucket-a', 'ap-guangzhou', 'https://cos-internal.example.com', 'https://cdn-a.example.com', 1, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			(11, '第二配置', '1250000001', 'v1:cipher-id-11', 'v1:cipher-key-11', 'bucket-b', 'ap-shanghai', NULL, 'https://cdn-b.example.com', 1, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			(12, '第三配置', '1250000002', 'v1:cipher-id-12', 'v1:cipher-key-12', 'bucket-c', 'ap-nanjing', NULL, NULL, 0, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		// 平台 1 故意复用 avatar：rule 20（public/enabled/config 10）与 rule 21（private/disabled/config 12）。
		`INSERT INTO storage_upload_rule (id, platform_id, name, cos_config_id, max_file_size_bytes, allowed_extensions, allowed_mime_types, access_mode, is_enabled, remark, created_at, updated_at) VALUES
			(20, 1, '默认100MB', 10, 104857600, '{png,jpg}'::text[], '{image/png}'::text[], 'public', 1, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			(21, 1, '备用规则', 12, 1048576, '{png}'::text[], '{image/png}'::text[], 'private', 0, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			(22, 2, 'Canvas 规则', 11, 5242880, '{png}'::text[], '{image/png}'::text[], 'private', 1, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		`INSERT INTO storage_upload_rule_code (id, rule_id, platform_id, code, created_at) VALUES
			(30, 20, 1, 'avatar', CURRENT_TIMESTAMP),
			(31, 20, 1, 'file', CURRENT_TIMESTAMP),
			(32, 21, 1, 'avatar', CURRENT_TIMESTAMP),
			(33, 22, 2, 'avatar', CURRENT_TIMESTAMP)`,
		`INSERT INTO system_setting (id, setting_key, value, value_type, description, is_enabled, is_builtin, created_at, updated_at) VALUES
			(1, 'app.brand.default_avatar', '` + storageObjectVersioningKeyB + `', 1, 'Default avatar COS object key using the avatar upload rule', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			(2, 'app.brand.title_zh_cn', '智澜', 1, 'Admin 中文品牌标题', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		`INSERT INTO user_profile (user_id, avatar, created_at, updated_at) VALUES
			(169, '` + storageObjectVersioningKeyA + `', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			(718, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		`INSERT INTO system_config_cache_generation (namespace, scope_key, generation) VALUES ('system.setting', 'global', 1)`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("seed storage object versioning fixture: %v", err)
		}
	}
}

func readStorageTableCount(t *testing.T, db *gorm.DB, ctx context.Context, table string) int64 {
	t.Helper()
	var count int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM ` + table).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func strPtr(value string) *string { return &value }

func sameOptionalString(left, right *string) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return *left == *right
	}
}
