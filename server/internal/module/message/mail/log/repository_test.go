package log

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	logverification "admin/server/internal/module/message/mail/logVerification"
	"admin/server/internal/secretkey"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestRepositoryKeepsOneActiveChallengePerSourcePlatform(t *testing.T) {
	database, ctx := openRepositoryDatabase(t)
	repository := NewRepository(database)
	challenge := "challenge-1"
	now := time.Now().UTC().Truncate(time.Microsecond)
	type result struct {
		value Model
		err   error
	}
	results := make(chan result, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			value, err := repository.CreatePending(ctx, &Model{
				PlatformID: 1, ChallengeID: &challenge, Scene: "login", TemplateID: 47941,
				ToEmail: "user@example.com", Subject: "subject", Status: "pending", CreatedAt: now, UpdatedAt: now,
			})
			results <- result{value: value, err: err}
		}()
	}
	wait.Wait()
	close(results)
	var created Model
	var uniqueErrors int
	for result := range results {
		if result.err == nil {
			created = result.value
			continue
		}
		var postgresError *pgconn.PgError
		if !errors.As(result.err, &postgresError) || postgresError.Code != "23505" {
			t.Fatalf("create error=%v", result.err)
		}
		uniqueErrors++
	}
	if created.ID < 1 || uniqueErrors != 1 {
		t.Fatalf("created=%+v uniqueErrors=%d", created, uniqueErrors)
	}
	if _, err := repository.CreatePending(ctx, &Model{
		PlatformID: 2, ChallengeID: &challenge, Scene: "login", TemplateID: 47941,
		ToEmail: "user@example.com", Subject: "subject", Status: "pending", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("challenge leaked across source platforms: %v", err)
	}
}

func TestRepositoryListsAllPlatformsForAdminConsole(t *testing.T) {
	database, ctx := openRepositoryDatabase(t)
	repository := NewRepository(database)
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := database.WithContext(ctx).Exec(`
INSERT INTO permission_auth_platform(id, code, name, login_types, allow_register, policy_version, menu_version,
 access_ttl_seconds, refresh_ttl_seconds, session_cache_ttl_seconds, access_cache_ttl_seconds,
 bind_device, bind_ip, max_sessions, is_enabled, is_builtin, created_at, updated_at)
VALUES (1, 'admin', 'Admin', '["email","password"]', 1, 3, 2, 3600, 86400, 900, 300, 0, 0, 5, 1, 1, ?, ?),
 (2, 'canvas', 'Canvas', '["email","password"]', 0, 1, 1, 3600, 86400, 900, 300, 0, 0, 5, 1, 0, ?, ?);`, now, now, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.WithContext(ctx).Exec(`
INSERT INTO user_account(id, username, email, password_hash, is_enabled, created_at, updated_at)
VALUES (7, 'tester', 'user@example.com', '', 1, ?, ?);`, now, now).Error; err != nil {
		t.Fatal(err)
	}
	adminLog, err := repository.CreatePending(ctx, &Model{
		PlatformID: 1, UserID: ptr[int64](7), Scene: "login", TemplateID: 47941, ToEmail: "user@example.com",
		Subject: "subject", Status: "sent", CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	canvasLog, err := repository.CreatePending(ctx, &Model{
		PlatformID: 2, Scene: "forget", TemplateID: 47941, ToEmail: "other@example.com",
		Subject: "subject", Status: "failed", CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	older, err := repository.CreatePending(ctx, &Model{
		PlatformID: 1, Scene: "login", TemplateID: 47941, ToEmail: "old@example.com",
		Subject: "subject", Status: "sent", CreatedAt: now.Add(-48 * time.Hour), UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}

	rows, total, err := repository.List(ctx, ListQuery{}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(rows) != 3 {
		t.Fatalf("total=%d rows=%d, want 3 admin-console rows across platforms", total, len(rows))
	}
	platformByLogID := map[int64]string{adminLog.ID: "admin", canvasLog.ID: "canvas", older.ID: "admin"}
	usernameByLogID := map[int64]string{adminLog.ID: "tester", canvasLog.ID: "", older.ID: ""}
	for _, row := range rows {
		if row.Platform != platformByLogID[row.ID] {
			t.Fatalf("log id=%d platform=%q, want %q", row.ID, row.Platform, platformByLogID[row.ID])
		}
		if row.Username != usernameByLogID[row.ID] {
			t.Fatalf("log id=%d username=%q, want %q", row.ID, row.Username, usernameByLogID[row.ID])
		}
	}

	platformRows, total, err := repository.List(ctx, ListQuery{Platform: "ad"}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(platformRows) != 2 {
		t.Fatalf("platform filter total=%d rows=%d, want 2", total, len(platformRows))
	}

	emailRows, total, err := repository.List(ctx, ListQuery{ToEmail: "user@"}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(emailRows) != 1 || emailRows[0].ID != adminLog.ID {
		t.Fatalf("email filter total=%d, want the admin log only", total)
	}

	sceneRows, total, err := repository.List(ctx, ListQuery{Scene: "forget"}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(sceneRows) != 1 || sceneRows[0].ID != canvasLog.ID {
		t.Fatalf("scene filter total=%d, want the canvas log only", total)
	}

	statusRows, total, err := repository.List(ctx, ListQuery{Status: "failed"}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(statusRows) != 1 || statusRows[0].ID != canvasLog.ID {
		t.Fatalf("status filter total=%d, want the canvas log only", total)
	}

	from := now.Add(-24 * time.Hour)
	timeRows, total, err := repository.List(ctx, ListQuery{From: &from}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(timeRows) != 2 {
		t.Fatalf("time filter total=%d rows=%d, want 2 recent logs", total, len(timeRows))
	}
	for _, row := range timeRows {
		if row.ID == older.ID {
			t.Fatalf("time filter returned the 48h-old log id=%d", row.ID)
		}
	}
}

func ptr[T any](value T) *T { return &value }

func TestRepositoryFindDeleteCrossPlatform(t *testing.T) {
	database, ctx := openRepositoryDatabase(t)
	repository := NewRepository(database)
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := database.WithContext(ctx).Exec(`
INSERT INTO permission_auth_platform(id, code, name, login_types, allow_register, policy_version, menu_version,
 access_ttl_seconds, refresh_ttl_seconds, session_cache_ttl_seconds, access_cache_ttl_seconds,
 bind_device, bind_ip, max_sessions, is_enabled, is_builtin, created_at, updated_at)
VALUES (2, 'canvas', 'Canvas', '["email","password"]', 0, 1, 1, 3600, 86400, 900, 300, 0, 0, 5, 1, 0, ?, ?);`, now, now).Error; err != nil {
		t.Fatal(err)
	}
	created, err := repository.CreatePending(ctx, &Model{
		PlatformID: 2, Scene: "login", TemplateID: 47941, ToEmail: "user@example.com",
		Subject: "subject", Status: "sent", CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	row, err := repository.Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("find cross-platform log: %v", err)
	}
	if row.PlatformID != 2 || row.Platform != "canvas" {
		t.Fatalf("row platformID=%d platform=%q, want platform 2 canvas", row.PlatformID, row.Platform)
	}
	if err := repository.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete cross-platform log: %v", err)
	}
	if _, err := repository.Find(ctx, created.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("deleted log is still visible: %v", err)
	}
}

func TestServiceDecryptsVerificationDetailFromSubmodule(t *testing.T) {
	database, ctx := openRepositoryDatabase(t)
	keys, err := secretkey.New(strings.Repeat("s", 64))
	if err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(database)
	verificationRepository := logverification.NewRepository(database)
	now := time.Now().UTC().Truncate(time.Microsecond)
	created, err := repository.CreatePending(ctx, &Model{
		PlatformID: 1, Scene: "login", TemplateID: 47941, ToEmail: "user@example.com",
		Subject: "subject", Status: "sent", CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, version, err := secretkey.EncryptMailValue(keys.MailEncryptionKey(), "123456")
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := now.Add(5 * time.Minute)
	if err := verificationRepository.Create(ctx, &logverification.Model{
		PlatformID: 1, MailLogID: created.ID, KeyVersion: version,
		CodeCiphertext: ciphertext, ExpiresAt: expiresAt, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	detail, err := NewService(repository, verificationRepository, keys).Get(ctx, created.ID)
	if err != nil || detail.VerificationCode != "123456" || detail.VerificationExpiresAt == nil || !detail.VerificationExpiresAt.Equal(expiresAt) {
		t.Fatalf("detail=%+v err=%v", detail, err)
	}
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
	database, ctx := testschema.Open(t, settings.PostgresDSN, "test_mail_log")
	if err := database.WithContext(ctx).Exec(`
CREATE TABLE message_mail_log(
 id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY, platform_id bigint NOT NULL,
 challenge_id varchar(128), user_id bigint, scene varchar(32) NOT NULL, template_id integer NOT NULL,
 to_email varchar(254) NOT NULL, subject varchar(255) NOT NULL, status varchar(16) NOT NULL,
 request_id varchar(128) NOT NULL DEFAULT '', message_id varchar(128) NOT NULL DEFAULT '',
 error_code varchar(128) NOT NULL DEFAULT '', error_summary varchar(512) NOT NULL DEFAULT '',
 latency_ms bigint NOT NULL DEFAULT 0, sent_at timestamptz, created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL, deleted_at timestamptz);
CREATE UNIQUE INDEX ux_message_mail_log_platform_challenge_active
 ON message_mail_log(platform_id,challenge_id) WHERE deleted_at IS NULL AND challenge_id IS NOT NULL;
CREATE TABLE message_mail_log_verification(
 id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,platform_id bigint NOT NULL,
 mail_log_id bigint NOT NULL,key_version varchar(16) NOT NULL,code_ciphertext text NOT NULL,
 expires_at timestamptz NOT NULL,created_at timestamptz NOT NULL,deleted_at timestamptz);
CREATE TABLE permission_auth_platform(
 id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY, code varchar(49) NOT NULL,
 name varchar(64) NOT NULL, policy_version bigint NOT NULL DEFAULT 1, access_ttl_seconds integer NOT NULL,
 refresh_ttl_seconds integer NOT NULL, session_cache_ttl_seconds integer NOT NULL,
 access_cache_ttl_seconds integer NOT NULL, bind_device smallint NOT NULL, bind_ip smallint NOT NULL,
 max_sessions smallint NOT NULL, allow_register smallint NOT NULL, is_enabled smallint NOT NULL,
 is_builtin smallint NOT NULL, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 deleted_at timestamptz, login_types jsonb NOT NULL DEFAULT '["email","password"]',
 menu_version bigint NOT NULL DEFAULT 1);
CREATE TABLE user_account(
 id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY, username varchar(64) NOT NULL,
 email varchar(254) NOT NULL, phone varchar(32), password_hash varchar(255) NOT NULL,
 is_enabled smallint NOT NULL DEFAULT 1, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 deleted_at timestamptz);`).Error; err != nil {
		t.Fatal(err)
	}
	return database, ctx
}
