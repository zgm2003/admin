package ratelimitpolicy

import (
	"context"
	"os"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	cachegeneration "admin/server/internal/shared/cacheGeneration"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestRepositoryUpdateAdvancesMailGenerationWithoutRevision(t *testing.T) {
	database, ctx := openRepositoryDatabase(t)
	repository := NewRepository(database)
	repository.SetGenerations(cachegeneration.NewRepository(database), cachegeneration.Scope{Namespace: "message.mail", ScopeKey: "global"})
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

	catalog, mutation, err := repository.Update(ctx, 1, Input{Key: "business_email_minute", Limit: 2, WindowSeconds: 120}, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	policy := policyByKey(t, catalog.Policies, "business_email_minute")
	if policy.Limit != 2 || policy.WindowSeconds != 120 || !policy.UpdatedAt.Equal(now) {
		t.Fatalf("catalog=%+v policy=%+v", catalog, policy)
	}
	if !mutation.Changed || mutation.Generation != 2 || mutation.OutboxID < 1 {
		t.Fatalf("mutation=%+v", mutation)
	}
	if generation := currentMailGeneration(t, database, ctx); generation != 2 {
		t.Fatalf("generation=%d, want 2", generation)
	}
	if _, _, err := repository.Update(ctx, 1, Input{Key: "unknown", Limit: 1, WindowSeconds: 60}, 2, now); err == nil {
		t.Fatal("missing policy update succeeded")
	}
}

func TestRepositoryNoOpDoesNotAdvanceMailGeneration(t *testing.T) {
	database, ctx := openRepositoryDatabase(t)
	repository := NewRepository(database)
	repository.SetGenerations(cachegeneration.NewRepository(database), cachegeneration.Scope{Namespace: "message.mail", ScopeKey: "global"})
	catalog, mutation, err := repository.Update(ctx, 1, Input{Key: "business_email_minute", Limit: 1, WindowSeconds: 60}, 1, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if mutation.Changed || len(catalog.Policies) != 2 || currentMailGeneration(t, database, ctx) != 1 {
		t.Fatalf("catalog=%+v mutation=%+v", catalog, mutation)
	}
}

func TestRepositoryListAllReturnsEveryPlatformCatalog(t *testing.T) {
	database, ctx := openRepositoryDatabase(t)
	repository := NewRepository(database)
	catalogs, err := repository.ListAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalogs) != 2 || catalogs[0].PlatformID != 1 || catalogs[1].PlatformID != 2 {
		t.Fatalf("catalogs=%+v, want platform 1 and 2", catalogs)
	}
	for _, catalog := range catalogs {
		if len(catalog.Policies) != 2 {
			t.Fatalf("platform %d policies=%d, want 2", catalog.PlatformID, len(catalog.Policies))
		}
	}
}

func TestRepositoryProvisionDefaultsIsIdempotentAndDeletesByPlatform(t *testing.T) {
	database, ctx := openRepositoryDatabase(t)
	repository := NewRepository(database)
	if err := database.WithContext(ctx).Exec("INSERT INTO permission_auth_platform(id, code, name) VALUES (3, 'mobile', 'Mobile')").Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.ProvisionDefaults(ctx, 3); err != nil {
		t.Fatal(err)
	}
	if err := repository.ProvisionDefaults(ctx, 3); err != nil {
		t.Fatal(err)
	}
	catalog, err := repository.List(ctx, 3)
	if err != nil || len(catalog.Policies) != 2 {
		t.Fatalf("provisioned catalog=%+v err=%v", catalog, err)
	}
	minute := policyByKey(t, catalog.Policies, "business_email_minute")
	tenMinutes := policyByKey(t, catalog.Policies, "business_email_10m")
	if minute.Limit != 1 || minute.WindowSeconds != 60 || tenMinutes.Limit != 5 || tenMinutes.WindowSeconds != 600 {
		t.Fatalf("default policies minute=%+v tenMinutes=%+v", minute, tenMinutes)
	}
	if err := repository.DeleteForPlatform(ctx, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.List(ctx, 3); err == nil {
		t.Fatal("deleted platform policies still form a catalog")
	}
}

func TestBuildCatalogRejectsInvalidRows(t *testing.T) {
	now := time.Now().UTC()
	rows := FixedRateLimitPolicies()
	for index := range rows {
		rows[index].PlatformID = 1
		rows[index].CreatedAt = now
		rows[index].UpdatedAt = now
	}
	for _, mutate := range []func(*Model){
		func(row *Model) { row.Mode = "invalid" },
		func(row *Model) { row.Dimension = "invalid" },
		func(row *Model) { row.Limit = 0 },
		func(row *Model) { row.WindowSeconds = 0 },
		func(row *Model) { row.UpdatedAt = time.Time{} },
	} {
		candidate := append([]Model(nil), rows...)
		mutate(&candidate[0])
		if _, err := BuildCatalog(1, candidate); err == nil {
			t.Fatalf("invalid row accepted: %+v", candidate[0])
		}
	}
}

func policyByKey(t *testing.T, policies []Model, key string) Model {
	t.Helper()
	for _, policy := range policies {
		if policy.Key == key {
			return policy
		}
	}
	t.Fatalf("policy %q not found", key)
	return Model{}
}

func openRepositoryDatabase(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	_ = godotenv.Load("../../../../../.env")
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	database, ctx := testschema.Open(t, settings.PostgresDSN, "test_mail_rate_limit_policy")
	if err := database.WithContext(ctx).Exec(`
CREATE TABLE permission_auth_platform(id bigint PRIMARY KEY, code varchar(49) NOT NULL, name varchar(64) NOT NULL, deleted_at timestamptz);
CREATE TABLE message_mail_rate_limit_policy(
 platform_id bigint NOT NULL, policy_key varchar(64) NOT NULL, mode varchar(16) NOT NULL, dimension varchar(64) NOT NULL,
 limit_count integer NOT NULL, window_seconds integer NOT NULL,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL, PRIMARY KEY(platform_id, policy_key));
INSERT INTO message_mail_rate_limit_policy VALUES
(1,'business_email_minute','business','platform_email',1,60,now(),now()),
(1,'business_email_10m','business','platform_email',5,600,now(),now()),
(2,'business_email_minute','business','platform_email',1,60,now(),now()),
(2,'business_email_10m','business','platform_email',5,600,now(),now());
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
	if err := database.WithContext(ctx).Exec(`INSERT INTO permission_auth_platform(id, code, name) VALUES (1, 'admin', 'Admin'), (2, 'canvas', 'Canvas')`).Error; err != nil {
		t.Fatal(err)
	}
	return database, ctx
}

func currentMailGeneration(t *testing.T, database *gorm.DB, ctx context.Context) int64 {
	t.Helper()
	generation, err := cachegeneration.NewRepository(database).Current(ctx, cachegeneration.Scope{Namespace: "message.mail", ScopeKey: "global"})
	if err != nil {
		t.Fatal(err)
	}
	return generation
}
