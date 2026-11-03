package ai

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"admin/server/internal/database/testschema"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func aiSchemaFixture(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	if err := godotenv.Load("../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("POSTGRES_DSN is required for AI migration validation")
	}
	db, ctx := testschema.Open(t, dsn, "test_ai_core")
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE user_account(id bigint PRIMARY KEY);
CREATE TABLE permission_auth_platform(id bigint PRIMARY KEY);
CREATE TABLE storage_file(id bigint PRIMARY KEY);
INSERT INTO user_account VALUES(1),(2);
INSERT INTO permission_auth_platform VALUES(1),(2);
INSERT INTO storage_file VALUES(1);`).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func aiCoreSQL(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("../../../../docs/database/2026-10-28-ai-core.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func runAICore(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(aiCoreSQL(t)).Error }); err != nil {
		t.Fatal(err)
	}
}

func TestAICoreSchemaCreatesTwentyOneTablesAndReplays(t *testing.T) {
	db, ctx := aiSchemaFixture(t)
	runAICore(t, db, ctx)
	want := []string{"ai_agent", "ai_agent_model_binding", "ai_agent_tool", "ai_conversation", "ai_media_file", "ai_media_task", "ai_message", "ai_official_model", "ai_provider", "ai_provider_model", "ai_provider_model_price_rate", "ai_provider_model_price_version", "ai_provider_request", "ai_provider_version", "ai_run", "ai_run_event", "ai_tool", "ai_tool_call", "ai_tool_version", "ai_usage_charge", "ai_usage_charge_item"}
	var got []string
	if err := db.WithContext(ctx).Raw(`SELECT tablename FROM pg_tables WHERE schemaname=current_schema() AND tablename LIKE 'ai\_%' ESCAPE '\' ORDER BY tablename`).Scan(&got).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("tables=%v", got)
	}
	var missing int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name LIKE 'ai\_%' ESCAPE '\' AND column_name IN('created_at','updated_at') AND (is_nullable<>'NO' OR data_type<>'timestamp with time zone')`).Scan(&missing).Error; err != nil || missing != 0 {
		t.Fatalf("invalid timestamps %d: %v", missing, err)
	}
	var before string
	if err := db.WithContext(ctx).Raw(`SELECT ai_core_schema_digest()`).Scan(&before).Error; err != nil {
		t.Fatal(err)
	}
	runAICore(t, db, ctx)
	var after string
	if err := db.WithContext(ctx).Raw(`SELECT ai_core_schema_digest()`).Scan(&after).Error; err != nil || before != after {
		t.Fatalf("schema changed on replay: %s != %s: %v", before, after, err)
	}
}

func TestAICoreSchemaRejectsDriftWithoutRepairingIt(t *testing.T) {
	db, ctx := aiSchemaFixture(t)
	runAICore(t, db, ctx)
	if err := db.WithContext(ctx).Exec(`ALTER TABLE ai_run ADD COLUMN unrelated integer`).Error; err != nil {
		t.Fatal(err)
	}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(aiCoreSQL(t)).Error })
	if err == nil {
		t.Fatal("schema drift accepted")
	}
	var found int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='ai_run' AND column_name='unrelated'`).Scan(&found).Error; err != nil || found != 1 {
		t.Fatalf("drift unexpectedly repaired: %d, %v", found, err)
	}
}

func TestAICoreSchemaProtectsVersionsAndPublishedPrices(t *testing.T) {
	db, ctx := aiSchemaFixture(t)
	runAICore(t, db, ctx)
	seedAIPrice(t, db, ctx)
	assertAIPriceProtection(t, db, ctx)
}

func seedAIPrice(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	seed := `
INSERT INTO ai_official_model(id,vendor_code,model_key,name,operations,source_urls,status,created_by,updated_by) VALUES(1,'openai','fixture','Fixture',ARRAY['text.generate'],'[]',1,1,1);
INSERT INTO ai_provider(id,code,name,status,current_version,created_by,updated_by) VALUES(1,'fixture','Fixture',1,1,1,1);
INSERT INTO ai_provider_version(id,provider_id,version,base_url,api_key_ciphertext,encryption_key_id,media_host_allowlist,request_timeout_ms,created_by) VALUES(1,1,1,'https://api.example.test/v1','cipher','key-1','[]',30000,1);
INSERT INTO ai_provider_model(id,provider_id,upstream_model_id,name,official_model_id,operations,limits,status) VALUES(1,1,'fixture','Fixture',1,ARRAY['text.generate'],'{}',1);
INSERT INTO ai_provider_model_price_version(id,provider_model_id,version,status,currency,source_currency,source_digest,snapshot_digest,source_payload,synced_at,created_by) VALUES(1,1,1,0,'CNY','USD',repeat('a',64),repeat('b',64),'{}',now(),1);
INSERT INTO ai_provider_model_price_rate(id,price_version_id,operation,usage_key,selector,selector_hash,unit,unit_quantity,source_unit_price,billing_unit_price_units) VALUES(1,1,'text.generate','input','{}',repeat('c',64),'token',1000000,5,500000000);`
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(seed).Error }); err != nil {
		t.Fatal(err)
	}
}

func assertAIPriceProtection(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	for _, tc := range []struct{ name, sql, code string }{
		{"direct published insertion", `INSERT INTO ai_provider_model_price_version(id,provider_model_id,version,status,effective_at,currency,source_currency,source_digest,snapshot_digest,source_payload,synced_at,created_by,published_by,published_at) VALUES(2,1,2,1,now(),'CNY','USD',repeat('d',64),repeat('e',64),'{}',now(),1,1,now())`, "P0001"},
		{"NaN source price", `INSERT INTO ai_provider_model_price_rate(id,price_version_id,operation,usage_key,selector,selector_hash,unit,unit_quantity,source_unit_price,billing_unit_price_units) VALUES(2,1,'text.generate','output','{}',repeat('d',64),'token',1000000,'NaN',1)`, "23514"},
		{"NaN quantity", `INSERT INTO ai_provider_model_price_rate(id,price_version_id,operation,usage_key,selector,selector_hash,unit,unit_quantity,source_unit_price,billing_unit_price_units) VALUES(3,1,'audio.speech','audio','{}',repeat('e',64),'second','NaN',1,1)`, "23514"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(tc.sql).Error })
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != tc.code {
				t.Fatalf("invalid price must fail with %s, got %v", tc.code, err)
			}
		})
	}
	for _, sql := range []string{
		`UPDATE ai_provider_version SET base_url='https://other.example.test' WHERE id=1`,
		`UPDATE ai_provider_model_price_rate SET source_unit_price=1 WHERE id=1`,
		`UPDATE ai_provider_model_price_version SET source_payload='{"forged":true}' WHERE id=1`,
	} {
		if err := db.WithContext(ctx).Exec(sql).Error; err == nil {
			t.Fatalf("immutable source modified: %s", sql)
		}
	}
	if err := db.WithContext(ctx).Exec(`UPDATE ai_provider_model_price_rate SET billing_unit_price_units=800000000 WHERE id=1`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`UPDATE ai_provider_model_price_version SET status=1,effective_at=now(),published_at=now(),published_by=1 WHERE id=1`).Error; err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`UPDATE ai_provider_model_price_rate SET billing_unit_price_units=1 WHERE id=1`,
		`DELETE FROM ai_provider_model_price_rate WHERE id=1`,
		`TRUNCATE ai_provider_version CASCADE`,
	} {
		if err := db.WithContext(ctx).Exec(sql).Error; err == nil {
			t.Fatalf("published version modified: %s", sql)
		}
	}
}

func TestAICoreSchemaFreezesRunAndChecksCurrentBinding(t *testing.T) {
	db, ctx := aiSchemaFixture(t)
	runAICore(t, db, ctx)
	seedAIPrice(t, db, ctx)
	seed := `
UPDATE ai_provider_model_price_version SET status=1,effective_at=now(),published_at=now(),published_by=1 WHERE id=1;
INSERT INTO ai_agent(id,code,name,capabilities,system_prompt,multiplier_ppm,generation_params,status,created_by,updated_by) VALUES(1,'fixture','Fixture',ARRAY['text'],'prompt',1000000,'{}',0,1,1);
INSERT INTO ai_agent_model_binding(id,agent_id,provider_model_id,text_protocol,parameters,status) VALUES(1,1,1,1,'{}',1);
UPDATE ai_agent SET status=1,active_binding_id=1 WHERE id=1;
INSERT INTO ai_run(id,user_id,platform_id,request_key,request_fingerprint,agent_id,binding_id,provider_model_id,provider_version_id,official_model_id,price_version_id,operation,execution_status,usage_status,billing_status,transfer_status,input_snapshot,config_snapshot,policy_snapshot,multiplier_ppm,estimated_units,reservation_status,state_version,accepted_at,deadline_at)
VALUES(1,1,1,'00000000-0000-4000-8000-000000000001',repeat('a',64),1,1,1,1,1,1,'text.generate',0,0,0,0,'{}','{}','{}',1000000,1,1,1,now(),now()+interval '3 minutes');`
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(seed).Error }); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`UPDATE ai_run SET config_snapshot='{"changed":true}' WHERE id=1`,
		`UPDATE ai_run SET multiplier_ppm=2000000 WHERE id=1`,
		`UPDATE ai_run SET user_id=2 WHERE id=1`,
		`DELETE FROM ai_run WHERE id=1`,
		`UPDATE ai_agent_model_binding SET status=0 WHERE id=1`,
	} {
		if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(sql).Error }); err == nil {
			t.Fatalf("run/binding protection bypassed: %s", sql)
		}
	}
	if err := db.WithContext(ctx).Exec(`UPDATE ai_run SET execution_status=1,state_version=2 WHERE id=1`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`UPDATE ai_run SET execution_status=3,state_version=3,finished_at=now() WHERE id=1`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`UPDATE ai_run SET execution_status=1,state_version=4 WHERE id=1`).Error; err == nil {
		t.Fatal("terminal run restarted")
	}
}
