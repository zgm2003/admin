package architecture_test

import (
	"context"
	"os"
	"testing"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

const mailRatePolicyPlatformMigrationPath = "../../../docs/database/2026-09-09-mail-rate-policy-platform.sql"

func TestMailRatePolicyPlatformMigrationCreatesTwoPoliciesPerPlatformAndIsIdempotent(t *testing.T) {
	db, ctx := openMailRatePolicyPlatformSchema(t, false)
	script, err := os.ReadFile(mailRatePolicyPlatformMigrationPath)
	if err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 2; run++ {
		if err := db.WithContext(ctx).Exec(string(script)).Error; err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		var result struct {
			Policies, AdminPolicies, CanvasPolicies, AdminMinute, CanvasMinute int64
		}
		if err := db.WithContext(ctx).Raw(`SELECT
(SELECT count(*) FROM message_mail_rate_limit_policy) policies,
(SELECT count(*) FROM message_mail_rate_limit_policy WHERE platform_id=1) admin_policies,
(SELECT count(*) FROM message_mail_rate_limit_policy WHERE platform_id=2) canvas_policies,
(SELECT limit_count FROM message_mail_rate_limit_policy WHERE platform_id=1 AND policy_key='business_email_minute') admin_minute,
(SELECT limit_count FROM message_mail_rate_limit_policy WHERE platform_id=2 AND policy_key='business_email_minute') canvas_minute`).Scan(&result).Error; err != nil {
			t.Fatal(err)
		}
		if result.Policies != 4 || result.AdminPolicies != 2 || result.CanvasPolicies != 2 || result.AdminMinute != 1 || result.CanvasMinute != 1 {
			t.Fatalf("run %d result=%+v", run, result)
		}
	}
	var constraints int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM pg_constraint WHERE conrelid='message_mail_rate_limit_policy'::regclass AND conname IN ('message_mail_rate_limit_policy_platform_id_fkey','ck_message_mail_rate_limit_policy_platform','ck_message_mail_rate_limit_policy_shape')`).Scan(&constraints).Error; err != nil {
		t.Fatal(err)
	}
	if constraints != 3 {
		t.Fatalf("constraints=%d, want 3", constraints)
	}
}

func TestMailRatePolicyPlatformMigrationRejectsUnexpectedPolicyAndRollsBack(t *testing.T) {
	db, ctx := openMailRatePolicyPlatformSchema(t, true)
	script, err := os.ReadFile(mailRatePolicyPlatformMigrationPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(string(script)).Error; err == nil {
		t.Fatal("migration accepted unexpected policy")
	}
	var result struct{ PlatformColumn, Policies, Unexpected int64 }
	if err := db.WithContext(ctx).Raw(`SELECT
(SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='message_mail_rate_limit_policy' AND column_name='platform_id') platform_column,
(SELECT count(*) FROM message_mail_rate_limit_policy) policies,
(SELECT count(*) FROM message_mail_rate_limit_policy WHERE policy_key='unexpected') unexpected`).Scan(&result).Error; err != nil {
		t.Fatal(err)
	}
	if result.PlatformColumn != 0 || result.Policies != 3 || result.Unexpected != 1 {
		t.Fatalf("failed migration changed state: %+v", result)
	}
}

func openMailRatePolicyPlatformSchema(t *testing.T, unexpected bool) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	_ = godotenv.Load("../../.env")
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_mail_rate_policy_platform")
	fixture := `
CREATE TABLE permission_auth_platform (id bigint PRIMARY KEY, code text NOT NULL, is_enabled smallint NOT NULL, deleted_at timestamptz);
CREATE TABLE message_mail_rate_limit_policy (policy_key varchar(64) PRIMARY KEY, mode varchar(16) NOT NULL, dimension varchar(64) NOT NULL, limit_count integer NOT NULL, window_seconds integer NOT NULL, revision bigint NOT NULL, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL);
INSERT INTO permission_auth_platform VALUES (1,'admin',1,NULL),(2,'canvas',1,NULL);
INSERT INTO message_mail_rate_limit_policy VALUES
('business_email_minute','business','platform_email',1,60,2,now(),now()),
('business_email_10m','business','platform_email',5,600,2,now(),now());`
	if unexpected {
		fixture += `INSERT INTO message_mail_rate_limit_policy VALUES ('unexpected','business','platform_email',1,60,2,now(),now());`
	}
	if err := db.WithContext(ctx).Exec(fixture).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}
