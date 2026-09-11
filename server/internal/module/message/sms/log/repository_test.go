package log

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"admin/server/internal/module/message/sms/logVerification"
	"admin/server/internal/secretkey"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func openSMSLogSchema(t *testing.T) (*gorm.DB, context.Context) {
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
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_sms_log_repository")
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE permission_auth_platform(
 id BIGINT PRIMARY KEY, code VARCHAR(49) NOT NULL, name VARCHAR(64) NOT NULL, deleted_at TIMESTAMPTZ);
CREATE TABLE user_account(
 id BIGINT PRIMARY KEY, username VARCHAR(64) NOT NULL, email VARCHAR(254) NOT NULL,
 phone VARCHAR(32), password_hash VARCHAR(255) NOT NULL, is_enabled SMALLINT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL, deleted_at TIMESTAMPTZ);
CREATE TABLE message_sms_log(
 id BIGSERIAL PRIMARY KEY, platform_id BIGINT NOT NULL, challenge_id VARCHAR(128), user_id BIGINT,
 scene VARCHAR(32) NOT NULL, template_id BIGINT NOT NULL, to_phone_ciphertext TEXT NOT NULL,
 to_phone_hint VARCHAR(32) NOT NULL, to_phone_hmac VARCHAR(128) NOT NULL, status VARCHAR(16) NOT NULL,
 request_id VARCHAR(128) NOT NULL DEFAULT '', serial_no VARCHAR(128) NOT NULL DEFAULT '', fee INTEGER NOT NULL DEFAULT 0,
 error_code VARCHAR(128) NOT NULL DEFAULT '', error_summary VARCHAR(512) NOT NULL DEFAULT '', latency_ms BIGINT NOT NULL DEFAULT 0,
 sent_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL);
CREATE UNIQUE INDEX ux_message_sms_log_platform_challenge_active
 ON message_sms_log(platform_id, challenge_id) WHERE challenge_id IS NOT NULL AND status = 'pending';
CREATE TABLE message_sms_log_verification(
 id BIGSERIAL PRIMARY KEY, platform_id BIGINT NOT NULL, sms_log_id BIGINT NOT NULL UNIQUE,
 key_version VARCHAR(16) NOT NULL, code_ciphertext TEXT NOT NULL, expires_at TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL);`).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func TestRepositoryListsAcrossPlatformsWithExactPhoneAndStablePagination(t *testing.T) {
	db, ctx := openSMSLogSchema(t)
	repository := NewRepository(db)
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	if err := db.WithContext(ctx).Exec(`INSERT INTO permission_auth_platform(id,code,name) VALUES
 (1,'admin','Admin'),(2,'canvas','Canvas')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO user_account(id,username,email,password_hash,is_enabled,created_at,updated_at)
 VALUES (7,'alice','alice@example.com','',1,?,?)`, now, now).Error; err != nil {
		t.Fatal(err)
	}
	rows := []*Model{
		{PlatformID: 1, UserID: pointer[int64](7), Scene: "login", TemplateID: 1, ToPhoneCiphertext: "cipher-a", ToPhoneHint: "156****8271", ToPhoneHMAC: "hmac-a", Status: StatusSent, CreatedAt: now, UpdatedAt: now},
		{PlatformID: 2, Scene: "forget", TemplateID: 2, ToPhoneCiphertext: "cipher-b", ToPhoneHint: "138****0000", ToPhoneHMAC: "hmac-b", Status: StatusFailed, CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
		{PlatformID: 1, Scene: "login", TemplateID: 1, ToPhoneCiphertext: "cipher-a", ToPhoneHint: "156****8271", ToPhoneHMAC: "hmac-a", Status: StatusSent, CreatedAt: now.Add(-48 * time.Hour), UpdatedAt: now},
	}
	for _, row := range rows {
		if err := repository.CreatePending(ctx, row); err != nil {
			t.Fatal(err)
		}
	}
	listed, total, err := repository.List(ctx, Query{Page: 1, PageSize: 1, Platform: "ad", PhoneToHMAC: pointer("hmac-a"), Scene: "login", Status: StatusSent})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(listed) != 1 || listed[0].ID != rows[0].ID || listed[0].Platform != "admin" || listed[0].Username != "alice" {
		t.Fatalf("total=%d rows=%+v", total, listed)
	}
	if err := db.WithContext(ctx).Model(&struct{ ID int64 }{}).Table("permission_auth_platform").Where("id = 1").Update("deleted_at", now).Error; err != nil {
		t.Fatal(err)
	}
	listed, _, err = repository.List(ctx, Query{Page: 1, PageSize: 20, Platform: "ad"})
	if err != nil || len(listed) != 2 || listed[0].Platform != "admin" {
		t.Fatalf("soft-deleted platform rows=%+v err=%v", listed, err)
	}
}

func TestServiceDecryptsSensitiveDetailByLogID(t *testing.T) {
	db, ctx := openSMSLogSchema(t)
	keys, err := secretkey.New(strings.Repeat("s", 64))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	if err := db.WithContext(ctx).Exec(`INSERT INTO permission_auth_platform(id,code,name) VALUES (2,'canvas','Canvas')`).Error; err != nil {
		t.Fatal(err)
	}
	phoneCiphertext, _, err := secretkey.EncryptSMSValue(keys.SMSEncryptionKey(), "+8615671628271")
	if err != nil {
		t.Fatal(err)
	}
	row := &Model{PlatformID: 2, Scene: "login", TemplateID: 1, ToPhoneCiphertext: phoneCiphertext, ToPhoneHint: "156****8271", ToPhoneHMAC: "hmac", Status: StatusSent, CreatedAt: now, UpdatedAt: now}
	if err := NewRepository(db).CreatePending(ctx, row); err != nil {
		t.Fatal(err)
	}
	codeCiphertext, version, err := secretkey.EncryptSMSValue(keys.SMSEncryptionKey(), "123456")
	if err != nil {
		t.Fatal(err)
	}
	expires := now.Add(5 * time.Minute)
	verificationRepository := logVerification.NewRepository(db)
	if err := verificationRepository.Create(ctx, &logVerification.Model{PlatformID: 2, SMSLogID: row.ID, KeyVersion: version, CodeCiphertext: codeCiphertext, ExpiresAt: expires, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	detail, err := NewService(NewRepository(db), verificationRepository, keys).Detail(ctx, row.ID)
	if err != nil || detail.ToPhone != "+8615671628271" || detail.VerificationCode != "123456" || detail.VerificationExpiresAt == nil {
		t.Fatalf("detail=%+v err=%v", detail, err)
	}
}

func pointer[T any](value T) *T { return &value }
