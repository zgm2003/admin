package phone

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"admin/server/internal/module/user/account"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func openPhoneSchema(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	if err := godotenv.Load(filepath.Join("..", "..", "..", "..", ".env")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_user_phone_repository")
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE permission_auth_platform(id BIGSERIAL PRIMARY KEY, code VARCHAR(32) NOT NULL UNIQUE);
CREATE TABLE user_account(
 id BIGSERIAL PRIMARY KEY, username VARCHAR(64) NOT NULL, email VARCHAR(254) NOT NULL DEFAULT '', phone VARCHAR(32),
 password_hash VARCHAR(255) NOT NULL DEFAULT '', is_enabled SMALLINT NOT NULL DEFAULT 1,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX ux_user_account_phone_active ON user_account(phone) WHERE phone IS NOT NULL AND deleted_at IS NULL;
CREATE TABLE user_phone_change_log(
 id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL REFERENCES user_account(id) ON DELETE RESTRICT,
 platform_id BIGINT NOT NULL REFERENCES permission_auth_platform(id) ON DELETE RESTRICT,
 action VARCHAR(16) NOT NULL CHECK(action IN ('bind','change')),
 old_phone_hint VARCHAR(32) NOT NULL, old_phone_hmac VARCHAR(128) NOT NULL,
 new_phone_hint VARCHAR(32) NOT NULL, new_phone_hmac VARCHAR(128) NOT NULL,
 created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL
);`).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func TestRepositoryChangePhoneLocksCurrentValueAndAppendsAudit(t *testing.T) {
	db, ctx := openPhoneSchema(t)
	platform := int64(1)
	if err := db.WithContext(ctx).Exec(`INSERT INTO permission_auth_platform(id,code) VALUES (1,'admin')`).Error; err != nil {
		t.Fatal(err)
	}
	user := account.User{Username: "alice", Email: "alice@example.com", PasswordHash: "hash", IsEnabled: yesno.Yes}
	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	now := time.Date(2026, 9, 11, 1, 2, 3, 0, time.UTC)
	input := ChangeInput{UserID: user.ID, PlatformID: platform, Action: ActionBind, NewPhone: "+8615671628271", NewHint: "156****8271", NewHMAC: "new-hmac", Now: now}
	if err := repository.Change(ctx, input); err != nil {
		t.Fatal(err)
	}
	current, err := repository.Current(ctx, user.ID)
	if err != nil || current.Phone == nil || *current.Phone != input.NewPhone {
		t.Fatalf("current=%+v error=%v", current, err)
	}
	var log ChangeLog
	if err := db.WithContext(ctx).Take(&log).Error; err != nil {
		t.Fatal(err)
	}
	if log.Action != ActionBind || log.NewPhoneHint != input.NewHint || log.NewPhoneHMAC != input.NewHMAC || !log.CreatedAt.Equal(now) {
		t.Fatalf("log=%+v", log)
	}

	stale := input
	stale.Action = ActionChange
	stale.OldPhone = "+8613800000000"
	stale.NewPhone = "+8613900000000"
	if err := repository.Change(ctx, stale); !errors.Is(err, ErrCurrentPhoneChanged) {
		t.Fatalf("stale error=%v", err)
	}
}

func TestRepositoryChangePhoneRollsBackWhenAuditInsertFails(t *testing.T) {
	db, ctx := openPhoneSchema(t)
	if err := db.WithContext(ctx).Exec(`INSERT INTO permission_auth_platform(id,code) VALUES (1,'admin')`).Error; err != nil {
		t.Fatal(err)
	}
	user := account.User{Username: "bob", Email: "bob@example.com", PasswordHash: "hash", IsEnabled: yesno.Yes}
	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`DROP TABLE user_phone_change_log`).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	err := repository.Change(ctx, ChangeInput{UserID: user.ID, PlatformID: 1, Action: ActionBind, NewPhone: "+8615671628271", NewHint: "156****8271", NewHMAC: "hmac", Now: time.Now()})
	if err == nil {
		t.Fatal("missing audit table was accepted")
	}
	current, findErr := repository.Current(ctx, user.ID)
	if findErr != nil || current.Phone != nil {
		t.Fatalf("transaction was not rolled back: current=%+v error=%v", current, findErr)
	}
}

func TestRepositoryMapsActivePhoneUniqueConflict(t *testing.T) {
	db, ctx := openPhoneSchema(t)
	if err := db.WithContext(ctx).Exec(`INSERT INTO permission_auth_platform(id,code) VALUES (1,'admin')`).Error; err != nil {
		t.Fatal(err)
	}
	phone := "+8615671628271"
	first := account.User{Username: "first", Email: "first@example.com", Phone: &phone, PasswordHash: "hash", IsEnabled: yesno.Yes}
	second := account.User{Username: "second", Email: "second@example.com", PasswordHash: "hash", IsEnabled: yesno.Yes}
	if err := db.WithContext(ctx).Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	err := NewRepository(db).Change(ctx, ChangeInput{UserID: second.ID, PlatformID: 1, Action: ActionBind, NewPhone: phone, NewHint: "156****8271", NewHMAC: "hmac", Now: time.Now()})
	if !errors.Is(err, ErrPhoneConflict) {
		t.Fatalf("unique conflict error=%v", err)
	}
}
