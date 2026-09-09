package ratelimitpolicy

import (
	"context"
	"os"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestRepositoryUpdateLocksCatalogAndIncrementsRevision(t *testing.T) {
	database, ctx := openRepositoryDatabase(t)
	repository := NewRepository(database)

	catalog, err := repository.Update(ctx, Input{Key: "business_email_minute", Limit: 2, WindowSeconds: 120})
	if err != nil {
		t.Fatal(err)
	}
	policy := policyByKey(t, catalog.Policies, "business_email_minute")
	if catalog.Version != 2 || policy.Limit != 2 || policy.WindowSeconds != 120 {
		t.Fatalf("catalog=%+v policy=%+v", catalog, policy)
	}

	catalog, err = repository.Update(ctx, Input{Key: "business_email_10m", Limit: 11, WindowSeconds: 60})
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Version != 3 {
		t.Fatalf("version=%d, want 3", catalog.Version)
	}
	if _, err := repository.Update(ctx, Input{Key: "unknown", Limit: 1, WindowSeconds: 60}); err == nil {
		t.Fatal("missing policy update succeeded")
	}
}

func TestBuildCatalogRejectsInvalidRows(t *testing.T) {
	now := time.Now().UTC()
	rows := FixedRateLimitPolicies()
	for index := range rows {
		rows[index].Revision = 1
		rows[index].CreatedAt = now
		rows[index].UpdatedAt = now
	}
	for _, mutate := range []func(*Model){
		func(row *Model) { row.Mode = "invalid" },
		func(row *Model) { row.Dimension = "invalid" },
		func(row *Model) { row.Limit = 0 },
		func(row *Model) { row.WindowSeconds = 0 },
		func(row *Model) { row.Revision = 0 },
		func(row *Model) { row.UpdatedAt = time.Time{} },
	} {
		candidate := append([]Model(nil), rows...)
		mutate(&candidate[0])
		if _, err := BuildCatalog(candidate); err == nil {
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
CREATE TABLE message_mail_rate_limit_policy(
 policy_key varchar(64) PRIMARY KEY, mode varchar(16) NOT NULL, dimension varchar(64) NOT NULL,
 limit_count integer NOT NULL, window_seconds integer NOT NULL, revision bigint NOT NULL,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL);
INSERT INTO message_mail_rate_limit_policy VALUES
('business_email_minute','business','platform_email',1,60,1,now(),now()),
('business_email_10m','business','platform_email',5,600,1,now(),now());`).Error; err != nil {
		t.Fatal(err)
	}
	return database, ctx
}
