package mail

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestIsUniqueViolationRecognizesPostgresConstraint(t *testing.T) {
	err := &pgconn.PgError{Code: "23505"}
	if !isUniqueViolation(err) {
		t.Fatal("expected PostgreSQL unique violation to be recognized")
	}
	if isUniqueViolation(errors.New("duplicate")) {
		t.Fatal("plain errors must not be treated as unique violations")
	}
}

func TestWrapRepoReturnsNilWithoutError(t *testing.T) {
	if err := wrapRepo(nil); err != nil {
		t.Fatalf("wrapRepo(nil) = %v, want nil", err)
	}
}

func TestUpdateRateLimitPolicyLocksAllRowsAndIncrementsRevision(t *testing.T) {
	db, ctx := openMailRepositoryDatabase(t)
	repository := NewRepository(db)

	catalog, err := repository.UpdateRateLimitPolicy(ctx, RateLimitPolicyInput{
		Key: "business_email_minute", Limit: 2, WindowSeconds: 120,
	})
	if err != nil {
		t.Fatal(err)
	}
	row := findPolicy(t, catalog.Policies, "business_email_minute")
	if catalog.Version != 2 || row.Limit != 2 || row.WindowSeconds != 120 {
		t.Fatalf("catalog=%+v row=%+v", catalog, row)
	}

	catalogAgain, err := repository.UpdateRateLimitPolicy(ctx, RateLimitPolicyInput{
		Key: "business_ip_minute", Limit: 11, WindowSeconds: 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	if catalogAgain.Version != 3 {
		t.Fatalf("version after second update = %d, want 3", catalogAgain.Version)
	}
	if updated := findPolicy(t, catalogAgain.Policies, "business_ip_minute"); updated.Limit != 11 || updated.WindowSeconds != 60 {
		t.Fatalf("second update row = %+v", updated)
	}

	if _, err := repository.UpdateRateLimitPolicy(ctx, RateLimitPolicyInput{
		Key: "unknown", Limit: 1, WindowSeconds: 60,
	}); err == nil {
		t.Fatal("expected missing policy key to fail")
	}
}

func TestBuildRateLimitCatalogRejectsInvalidRowShape(t *testing.T) {
	rows := make([]RateLimitPolicy, 0, len(fixedRateLimitPolicyKeys))
	now := time.Now().UTC()
	for _, policy := range FixedRateLimitPolicies() {
		policy.Revision = 1
		policy.CreatedAt = now
		policy.UpdatedAt = now
		rows = append(rows, policy)
	}

	for _, mutate := range []func(*RateLimitPolicy){
		func(row *RateLimitPolicy) { row.Mode = "invalid" },
		func(row *RateLimitPolicy) { row.Dimension = "invalid" },
		func(row *RateLimitPolicy) { row.Limit = 0 },
		func(row *RateLimitPolicy) { row.WindowSeconds = 0 },
		func(row *RateLimitPolicy) { row.Revision = 0 },
		func(row *RateLimitPolicy) { row.UpdatedAt = time.Time{} },
	} {
		candidate := append([]RateLimitPolicy(nil), rows...)
		mutate(&candidate[0])
		if _, err := buildRateLimitCatalog(candidate); err == nil {
			t.Fatalf("accepted invalid policy row: %+v", candidate[0])
		}
	}
}

func findPolicy(t *testing.T, policies []RateLimitPolicy, key string) RateLimitPolicy {
	t.Helper()
	for _, policy := range policies {
		if policy.Key == key {
			return policy
		}
	}
	t.Fatalf("policy %q not found", key)
	return RateLimitPolicy{}
}

func TestFindActiveChallengeReturnsPendingAndSentLogs(t *testing.T) {
	db, ctx := openMailRepositoryDatabase(t)
	repository := NewRepository(db)
	now := time.Now().UTC()
	challenge := "challenge-1"
	type createResult struct {
		row Log
		err error
	}
	var wg sync.WaitGroup
	results := make(chan createResult, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			row, err := repository.CreatePendingLog(ctx, &Log{PlatformID: 1, ChallengeID: &challenge, Scene: SceneLogin, TemplateID: 47941, ToEmail: "user@example.com", Subject: "subject", Status: StatusPending, CreatedAt: now, UpdatedAt: now})
			results <- createResult{row: row, err: err}
		}()
	}
	wg.Wait()
	close(results)
	var first Log
	var uniqueErrors int
	for result := range results {
		if result.err == nil {
			first = result.row
		} else if isUniqueViolation(result.err) {
			uniqueErrors++
		} else {
			t.Fatalf("concurrent create error = %v", result.err)
		}
	}
	if first.ID == 0 || uniqueErrors != 1 {
		t.Fatalf("concurrent creates = row=%+v uniqueErrors=%d", first, uniqueErrors)
	}
	found, err := repository.FindActiveChallenge(ctx, 1, challenge)
	if err != nil || found.ID != first.ID || found.Status != StatusPending {
		t.Fatalf("pending active log = %+v,%v", found, err)
	}
	if err := db.WithContext(ctx).Model(&Log{}).Where("id = ?", first.ID).Update("status", StatusSent).Error; err != nil {
		t.Fatal(err)
	}
	found, err = repository.FindActiveChallenge(ctx, 1, challenge)
	if err != nil || found.ID != first.ID || found.Status != StatusSent {
		t.Fatalf("sent active log = %+v,%v", found, err)
	}
}

func openMailRepositoryDatabase(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration test")
	}
	_ = godotenv.Load("../../../../.env")
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_mail_repository")
	if err := db.WithContext(ctx).Exec(`
		CREATE TABLE message_mail_log (
			id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
			platform_id BIGINT NOT NULL,
			challenge_id VARCHAR(128),
			user_id BIGINT,
			scene VARCHAR(32) NOT NULL,
			template_id INTEGER NOT NULL,
			to_email VARCHAR(254) NOT NULL,
			subject VARCHAR(255) NOT NULL,
			status VARCHAR(16) NOT NULL,
			request_id VARCHAR(128) NOT NULL DEFAULT '',
			message_id VARCHAR(128) NOT NULL DEFAULT '',
			error_code VARCHAR(128) NOT NULL DEFAULT '',
			error_summary VARCHAR(512) NOT NULL DEFAULT '',
			latency_ms BIGINT NOT NULL DEFAULT 0,
			sent_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			deleted_at TIMESTAMPTZ
		);
		CREATE UNIQUE INDEX ux_message_mail_log_platform_challenge_active ON message_mail_log(platform_id, challenge_id) WHERE deleted_at IS NULL AND challenge_id IS NOT NULL;
	`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`
		CREATE TABLE message_mail_rate_limit_policy (
			policy_key VARCHAR(64) PRIMARY KEY,
			mode VARCHAR(16) NOT NULL,
			dimension VARCHAR(64) NOT NULL,
			limit_count INTEGER NOT NULL,
			window_seconds INTEGER NOT NULL,
			revision BIGINT NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO message_mail_rate_limit_policy (policy_key, mode, dimension, limit_count, window_seconds, revision) VALUES
			('business_email_minute', 'business', 'platform_scene_email', 1, 60, 1),
			('business_email_10m', 'business', 'platform_scene_email', 5, 600, 1),
			('business_ip_minute', 'business', 'platform_ip', 10, 60, 1),
			('business_scene_minute', 'business', 'platform_scene', 30, 60, 1),
			('admin_test_user_10m', 'admin_test', 'admin_user', 5, 600, 1),
			('admin_test_ip_minute', 'admin_test', 'ip', 10, 60, 1),
			('admin_test_email_10m', 'admin_test', 'email', 3, 600, 1);
	`).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}
