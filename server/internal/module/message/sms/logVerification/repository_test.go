package logVerification

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func openVerificationSchema(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	if err := godotenv.Load(filepath.Join("..", "..", "..", "..", "..", ".env")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_sms_log_verification_repository")
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE message_sms_log(
 id BIGINT NOT NULL, platform_id BIGINT NOT NULL, PRIMARY KEY(id), UNIQUE(id, platform_id));
CREATE TABLE message_sms_log_verification(
 id BIGSERIAL PRIMARY KEY, platform_id BIGINT NOT NULL, sms_log_id BIGINT NOT NULL UNIQUE,
 key_version VARCHAR(16) NOT NULL, code_ciphertext TEXT NOT NULL, expires_at TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
 CONSTRAINT fk_message_sms_log_verification_log FOREIGN KEY(sms_log_id, platform_id)
 REFERENCES message_sms_log(id, platform_id) ON DELETE RESTRICT);`).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func TestRepositoryCreatesOneVerificationPerLogAndScopesReadsByPlatform(t *testing.T) {
	db, ctx := openVerificationSchema(t)
	if err := db.WithContext(ctx).Exec(`INSERT INTO message_sms_log(id, platform_id) VALUES (7, 2), (8, 3)`).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	row := &Model{PlatformID: 2, SMSLogID: 7, KeyVersion: "v1", CodeCiphertext: "cipher", ExpiresAt: now.Add(5 * time.Minute), CreatedAt: now, UpdatedAt: now}
	if err := repository.Create(ctx, row); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.FindBySMSLog(ctx, 3, 7); err == nil {
		t.Fatal("cross-platform verification read succeeded")
	}
	got, err := repository.FindBySMSLog(ctx, 2, 7)
	if err != nil || got.ID != row.ID || got.CodeCiphertext != "cipher" {
		t.Fatalf("row=%+v err=%v", got, err)
	}
	duplicate := *row
	duplicate.ID = 0
	if err := repository.Create(ctx, &duplicate); err == nil {
		t.Fatal("duplicate verification was accepted")
	}
	foreign := *row
	foreign.ID = 0
	foreign.SMSLogID = 8
	if err := repository.Create(ctx, &foreign); err == nil {
		t.Fatal("cross-platform verification was accepted")
	}
}
