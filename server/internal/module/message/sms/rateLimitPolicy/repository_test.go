package rateLimitPolicy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func openPolicySchema(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	if err := godotenv.Load(filepath.Join("..", "..", "..", "..", "..", ".env")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_sms_rate_limit_repository")
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE permission_auth_platform(
 id BIGSERIAL PRIMARY KEY,
 code VARCHAR(64) NOT NULL,
 name VARCHAR(128) NOT NULL,
 deleted_at TIMESTAMPTZ
);
CREATE TABLE message_sms_rate_limit_policy(
 platform_id BIGINT NOT NULL,
 policy_key VARCHAR(64) NOT NULL,
 mode VARCHAR(16) NOT NULL,
 dimension VARCHAR(64) NOT NULL,
 limit_count INTEGER NOT NULL,
 window_seconds INTEGER NOT NULL,
 revision BIGINT NOT NULL DEFAULT 1,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 CONSTRAINT message_sms_rate_limit_policy_pkey PRIMARY KEY (platform_id, policy_key),
 CONSTRAINT fk_fixture_sms_policy_platform FOREIGN KEY (platform_id) REFERENCES permission_auth_platform(id)
);`).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func seedPlatforms(t *testing.T, db *gorm.DB, ctx context.Context) (int64, int64) {
	t.Helper()
	var adminID, retiredID int64
	if err := db.WithContext(ctx).Raw(`INSERT INTO permission_auth_platform(code,name) VALUES ('admin','Admin') RETURNING id`).Scan(&adminID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`INSERT INTO permission_auth_platform(code,name,deleted_at) VALUES ('retired','Retired',CURRENT_TIMESTAMP) RETURNING id`).Scan(&retiredID).Error; err != nil {
		t.Fatal(err)
	}
	if err := NewRepository(db).ProvisionDefaults(ctx, adminID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	return adminID, retiredID
}

func TestProvisionDefaultsIsIdempotentAndUsesTheFixedValues(t *testing.T) {
	db, ctx := openPolicySchema(t)
	adminID, _ := seedPlatforms(t, db, ctx)
	repository := NewRepository(db)
	if err := repository.ProvisionDefaults(ctx, adminID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	var rows []Model
	if err := db.WithContext(ctx).Where("platform_id = ?", adminID).Order("policy_key").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("policies = %d", len(rows))
	}
	if rows[0].Key != KeyTenMin || rows[0].Limit != 5 || rows[0].WindowSeconds != 600 {
		t.Fatalf("policy[0] = %+v", rows[0])
	}
	if rows[1].Key != KeyMinute || rows[1].Limit != 1 || rows[1].WindowSeconds != 60 {
		t.Fatalf("policy[1] = %+v", rows[1])
	}
	if rows[0].Mode != ModeBusiness || rows[0].Dimension != DimensionPhone || rows[0].Revision != 1 {
		t.Fatalf("policy[0] = %+v", rows[0])
	}
}

func TestListPlatformsSkipsSoftDeletedPlatforms(t *testing.T) {
	db, ctx := openPolicySchema(t)
	adminID, retiredID := seedPlatforms(t, db, ctx)
	if err := NewRepository(db).ProvisionDefaults(ctx, retiredID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	catalogs, err := NewRepository(db).ListPlatforms(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalogs) != 1 || catalogs[0].PlatformID != adminID {
		t.Fatalf("catalogs = %+v", catalogs)
	}
}

func TestFindPlatformRejectsUnknownAndSoftDeletedPlatforms(t *testing.T) {
	db, ctx := openPolicySchema(t)
	_, retiredID := seedPlatforms(t, db, ctx)
	repository := NewRepository(db)

	if _, err := repository.FindPlatform(ctx, retiredID); !errors.Is(err, ErrPlatformNotFound) {
		t.Fatalf("soft deleted platform error = %v", err)
	}
	if _, err := repository.FindPlatform(ctx, 987654321); !errors.Is(err, ErrPlatformNotFound) {
		t.Fatalf("unknown platform error = %v", err)
	}
}

func TestUpdatePolicyIncrementsRevisionAndDeleteRemovesRows(t *testing.T) {
	db, ctx := openPolicySchema(t)
	adminID, _ := seedPlatforms(t, db, ctx)
	repository := NewRepository(db)

	first, err := repository.UpdatePolicy(ctx, adminID, KeyMinute, 3, 120, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.UpdatePolicy(ctx, adminID, KeyMinute, 4, 180, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != 2 || second.Revision != 3 || second.Limit != 4 || second.WindowSeconds != 180 {
		t.Fatalf("revisions = %d / %d", first.Revision, second.Revision)
	}

	if _, err := repository.UpdatePolicy(ctx, adminID, KeyMinute, 1, 60, time.Now().UTC()); err != nil {
		t.Fatalf("third update error = %v", err)
	}
	if _, err := NewRepository(db).UpdatePolicy(ctx, 987654321, KeyMinute, 1, 60, time.Now().UTC()); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("unknown policy error = %v", err)
	}

	if err := repository.DeleteForPlatform(ctx, adminID); err != nil {
		t.Fatal(err)
	}
	catalog, err := repository.FindPlatform(ctx, adminID)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Policies) != 0 {
		t.Fatalf("policies after delete = %+v", catalog.Policies)
	}
}
