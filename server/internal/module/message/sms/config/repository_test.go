package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func openConfigRepositorySchema(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	if err := godotenv.Load(filepath.Join("..", "..", "..", "..", "..", ".env")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_sms_config_repository")
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE message_sms_config(
 id BIGSERIAL PRIMARY KEY,
 secret_id_ciphertext TEXT NOT NULL,
 secret_key_ciphertext TEXT NOT NULL,
 secret_id_hint VARCHAR(32) NOT NULL DEFAULT '',
 secret_key_hint VARCHAR(32) NOT NULL DEFAULT '',
 sms_sdk_app_id VARCHAR(64) NOT NULL,
 sign_name VARCHAR(128) NOT NULL,
 region VARCHAR(64) NOT NULL,
 endpoint VARCHAR(255),
 ttl_minutes SMALLINT NOT NULL,
 is_enabled SMALLINT NOT NULL DEFAULT 0,
 last_test_at TIMESTAMPTZ,
 last_test_error VARCHAR(512) NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX ux_message_sms_config_active_singleton ON message_sms_config((true)) WHERE deleted_at IS NULL;`).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func activeConfig() Model {
	return Model{
		SecretIDCiphertext: "sms:v1:id", SecretKeyCiphertext: "sms:v1:key",
		SecretIDHint: "AK***Id", SecretKeyHint: "ex***ue", SDKAppID: "1400006666",
		SignName: "签名", Region: "ap-guangzhou", TTLMinutes: 5, IsEnabled: yesno.Yes,
	}
}

func TestRepositoryKeepsASingleActiveConfiguration(t *testing.T) {
	db, ctx := openConfigRepositorySchema(t)
	repository := NewRepository(db)

	first := activeConfig()
	if err := repository.Create(ctx, &first); err != nil {
		t.Fatalf("create first config: %v", err)
	}
	second := activeConfig()
	if err := repository.Create(ctx, &second); err == nil {
		t.Fatal("a second active configuration was accepted")
	}

	stored, err := repository.FindActive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != first.ID || stored.SDKAppID != "1400006666" || stored.IsEnabled != yesno.Yes {
		t.Fatalf("stored = %+v", stored)
	}
}

func TestRepositorySoftDeletesAndAllowsAReplacement(t *testing.T) {
	db, ctx := openConfigRepositorySchema(t)
	repository := NewRepository(db)

	first := activeConfig()
	if err := repository.Create(ctx, &first); err != nil {
		t.Fatal(err)
	}
	if err := repository.Delete(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.FindActive(ctx); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("FindActive() after delete = %v", err)
	}

	replacement := activeConfig()
	replacement.SignName = "新签名"
	if err := repository.Create(ctx, &replacement); err != nil {
		t.Fatalf("create replacement config: %v", err)
	}
	stored, err := repository.FindActive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stored.SignName != "新签名" || stored.ID == first.ID {
		t.Fatalf("stored replacement = %+v", stored)
	}
}

func TestRepositoryUpdatesEditableFieldsAndTestResult(t *testing.T) {
	db, ctx := openConfigRepositorySchema(t)
	repository := NewRepository(db)

	value := activeConfig()
	if err := repository.Create(ctx, &value); err != nil {
		t.Fatal(err)
	}
	endpoint := "sms.tencentcloudapi.com"
	value.Endpoint = &endpoint
	value.TTLMinutes = 30
	value.IsEnabled = yesno.No
	updatedAt := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	if err := repository.Update(ctx, &value, updatedAt); err != nil {
		t.Fatal(err)
	}
	testedAt := time.Date(2026, 9, 10, 11, 0, 0, 0, time.UTC)
	if err := repository.UpdateTestResult(ctx, value.ID, testedAt, "provider rejected the signature"); err != nil {
		t.Fatal(err)
	}

	stored, err := repository.FindActive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stored.TTLMinutes != 30 || stored.IsEnabled != yesno.No ||
		stored.Endpoint == nil || *stored.Endpoint != endpoint {
		t.Fatalf("stored = %+v", stored)
	}
	if stored.LastTestAt == nil || !stored.LastTestAt.UTC().Equal(testedAt) ||
		stored.LastTestError != "provider rejected the signature" {
		t.Fatalf("test result = %v / %q", stored.LastTestAt, stored.LastTestError)
	}
	if !stored.UpdatedAt.UTC().Equal(testedAt) {
		t.Fatalf("updated_at = %v", stored.UpdatedAt)
	}
}
