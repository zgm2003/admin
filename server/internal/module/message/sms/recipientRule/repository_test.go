package recipientRule

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func openRuleSchema(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	if err := godotenv.Load(filepath.Join("..", "..", "..", "..", "..", ".env")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_sms_recipient_rule_repository")
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE message_sms_recipient_rule(
 id BIGSERIAL PRIMARY KEY,
 scope VARCHAR(16) NOT NULL,
 pattern_ciphertext TEXT NOT NULL,
 pattern_hint VARCHAR(64) NOT NULL,
 pattern_hmac VARCHAR(128) NOT NULL,
 action VARCHAR(16) NOT NULL,
 name VARCHAR(128) NOT NULL,
 remark VARCHAR(512) NOT NULL DEFAULT '',
 is_enabled SMALLINT NOT NULL DEFAULT 1,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX ux_message_sms_recipient_rule_pattern_action_active
ON message_sms_recipient_rule(scope, pattern_hmac, action) WHERE deleted_at IS NULL;`).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func realKeys(t *testing.T) *secretkey.KeyRing {
	t.Helper()
	keys, err := secretkey.New(strings.Repeat("s", 64))
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

func TestRepositoryRejectsADuplicateActivePatternAndAction(t *testing.T) {
	db, ctx := openRuleSchema(t)
	service := NewService(NewRepository(db), realKeys(t))

	first, err := service.Create(ctx, CreateInput{
		Scope: ScopePhone, Pattern: "15671628271", Action: ActionDeny, Name: "黑名单", IsEnabled: yesno.Yes,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if first.PatternHint != "156****8271" {
		t.Fatalf("hint = %q", first.PatternHint)
	}

	_, err = service.Create(ctx, CreateInput{
		Scope: ScopePhone, Pattern: "+86 156-7162-8271", Action: ActionDeny, Name: "重复", IsEnabled: yesno.Yes,
	})
	if !errors.Is(err, ErrConflict) && appErrorCode(err) != 10005 {
		t.Fatalf("duplicate create error = %v", err)
	}

	// A different action on the same pattern is a distinct rule.
	if _, err := service.Create(ctx, CreateInput{
		Scope: ScopePhone, Pattern: "15671628271", Action: ActionAllow, Name: "放行", IsEnabled: yesno.Yes,
	}); err != nil {
		t.Fatalf("Create() with another action error = %v", err)
	}
}

func TestRepositoryIgnoresSoftDeletedRules(t *testing.T) {
	db, ctx := openRuleSchema(t)
	service := NewService(NewRepository(db), realKeys(t))

	created, err := service.Create(ctx, CreateInput{
		Scope: ScopePrefix, Pattern: "+86156", Action: ActionDeny, Name: "前缀", IsEnabled: yesno.Yes,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := service.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	safes, err := service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(safes) != 0 {
		t.Fatalf("soft deleted rule is still listed: %+v", safes)
	}
	// The same pattern can be created again after the soft delete.
	if _, err := service.Create(ctx, CreateInput{
		Scope: ScopePrefix, Pattern: "+86156", Action: ActionDeny, Name: "重建", IsEnabled: yesno.Yes,
	}); err != nil {
		t.Fatalf("Create() after delete error = %v", err)
	}
}

func TestRepositoryUpdateKeepsTheStoredPatternWhenOmitted(t *testing.T) {
	db, ctx := openRuleSchema(t)
	repository := NewRepository(db)
	service := NewService(repository, realKeys(t))

	created, err := service.Create(ctx, CreateInput{
		Scope: ScopePhone, Pattern: "15671628271", Action: ActionDeny, Name: "黑名单", IsEnabled: yesno.Yes,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	updated, err := service.Update(ctx, created.ID, UpdateInput{
		Scope: ScopePhone, Action: ActionAllow, Name: "放行", IsEnabled: yesno.Yes,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.PatternHint != created.PatternHint || updated.Action != ActionAllow {
		t.Fatalf("updated = %+v", updated)
	}

	var stored Model
	if err := db.WithContext(ctx).First(&stored, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.PatternHMAC == "" || stored.PatternCiphertext[:7] != "sms:v1:" {
		t.Fatalf("stored = %+v", stored)
	}
}
