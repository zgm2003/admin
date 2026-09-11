package template

import (
	"context"
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

func openTemplateSchema(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	if err := godotenv.Load(filepath.Join("..", "..", "..", "..", "..", ".env")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_sms_template_repository")
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE message_sms_template(
 id BIGSERIAL PRIMARY KEY,
 scene VARCHAR(32) NOT NULL,
 name VARCHAR(128) NOT NULL,
 tencent_template_id VARCHAR(64) NOT NULL DEFAULT '',
 parameter_keys JSONB NOT NULL,
 example_variables JSONB NOT NULL,
 is_enabled SMALLINT NOT NULL DEFAULT 0,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 CONSTRAINT ck_fixture_sms_template_scene CHECK (scene IN ('login','forget','bind_phone','change_password'))
);
CREATE UNIQUE INDEX ux_message_sms_template_scene ON message_sms_template(scene);`).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func seedTemplates(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	for index, fixed := range FixedCatalog() {
		if err := db.WithContext(ctx).Exec(`
INSERT INTO message_sms_template(scene,name,tencent_template_id,parameter_keys,example_variables,is_enabled,created_at,updated_at)
VALUES (?,?,'',?::jsonb,?::jsonb,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
			fixed.Scene, fixed.Name, `["code","ttl_minutes"]`, `{"code":"123456","ttl_minutes":"5"}`, yesno.No).
			Error; err != nil {
			t.Fatalf("seed template %d: %v", index, err)
		}
	}
}

func TestRepositoryListsTemplatesInCatalogOrder(t *testing.T) {
	db, ctx := openTemplateSchema(t)
	seedTemplates(t, db, ctx)
	repository := NewRepository(db)

	rows, err := repository.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{SceneLogin, SceneForget, SceneBindPhone, SceneChangePassword}
	if len(rows) != len(want) {
		t.Fatalf("rows = %d", len(rows))
	}
	for index, scene := range want {
		if rows[index].Scene != scene {
			t.Fatalf("row[%d].Scene = %q, want %q", index, rows[index].Scene, scene)
		}
	}
}

func TestRepositoryUpdatesContentAndStatusOnly(t *testing.T) {
	db, ctx := openTemplateSchema(t)
	seedTemplates(t, db, ctx)
	repository := NewRepository(db)
	service := NewService(repository)

	rows, err := repository.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	target := rows[0]

	updated, err := service.Update(ctx, target.ID, UpdateInput{
		Scene:             target.Scene,
		Name:              "  登录短信验证码  ",
		TencentTemplateID: "1234567",
		ParameterKeys:     []string{"code", "ttl_minutes"},
		ExampleVariables:  map[string]string{"code": "123456", "ttl_minutes": "5"},
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != "登录短信验证码" || updated.TencentTemplateID != "1234567" || updated.Scene != target.Scene {
		t.Fatalf("updated = %+v", updated)
	}

	if err := service.UpdateStatus(ctx, target.ID, yesno.Yes); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	var stored Model
	if err := db.WithContext(ctx).First(&stored, target.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.IsEnabled != yesno.Yes || stored.Scene != target.Scene || stored.Name != "登录短信验证码" {
		t.Fatalf("stored = %+v", stored)
	}
	if !stored.UpdatedAt.After(time.Time{}) {
		t.Fatal("updated_at was not maintained")
	}
}

func TestRepositoryRejectsADuplicateScene(t *testing.T) {
	db, ctx := openTemplateSchema(t)
	seedTemplates(t, db, ctx)
	err := db.WithContext(ctx).Exec(`
INSERT INTO message_sms_template(scene,name,tencent_template_id,parameter_keys,example_variables,is_enabled)
VALUES ('login','重复','','["code","ttl_minutes"]'::jsonb,'{}'::jsonb,0)`).Error
	if err == nil {
		t.Fatal("a duplicate scene was accepted")
	}
}
