package config

import (
	"context"
	"os"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func platformRoutesSQL(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../../../../../docs/database/2026-10-18-platform-api-routes.sql")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
func runPlatformRoutesSQL(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(platformRoutesSQL(t)).Error }); err != nil {
		t.Fatal(err)
	}
}
func platformMigrationFacts(t *testing.T, db *gorm.DB, ctx context.Context) (string, int64, int64) {
	t.Helper()
	var facts struct {
		Digest             string
		Generation, Events int64
	}
	if err := db.WithContext(ctx).Raw(`SELECT
 (SELECT md5(coalesce(jsonb_agg(to_jsonb(v) ORDER BY config_id,version)::text,'')) FROM payment_config_version v) digest,
 (SELECT generation FROM system_config_cache_state WHERE namespace='payment.config' AND scope_key='global') generation,
 (SELECT count(*) FROM system_config_cache_outbox) events`).Scan(&facts).Error; err != nil {
		t.Fatal(err)
	}
	return facts.Digest, facts.Generation, facts.Events
}
func TestPlatformCallbackMigrationPreservesVersionsAndReplays(t *testing.T) {
	db, ctx := configFixture(t, false)
	runConfigSQL(t, db, ctx)
	seedLegacyConfig(t, db, ctx)
	runRemoveCodeSQL(t, db, ctx)
	var protected string
	query := `SELECT md5((to_jsonb(v)-'notify_url')::text) FROM payment_config_version v WHERE config_id=100`
	if err := db.WithContext(ctx).Raw(query).Scan(&protected).Error; err != nil {
		t.Fatal(err)
	}
	_, beforeGen, beforeEvents := platformMigrationFacts(t, db, ctx)
	runPlatformRoutesSQL(t, db, ctx)
	var url, afterProtected string
	if err := db.WithContext(ctx).Raw(`SELECT notify_url FROM payment_config_version WHERE config_id=100`).Scan(&url).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(query).Scan(&afterProtected).Error; err != nil {
		t.Fatal(err)
	}
	if url != "https://pay.example.test/api/admin/v1/payment/callback/alipay/100/1" || afterProtected != protected {
		t.Fatal("callback migration changed protected credential facts or coordinates")
	}
	credential, err := NewRepository(db).CredentialFacts(ctx, 100, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateSnapshot(snapshot{SchemaVersion: 1, Generation: beforeGen + 1, Variant: credentialVariant(100, 1), Credential: &credential}); err != nil {
		t.Fatalf("migrated credential cannot refill runtime cache: %v", err)
	}
	digest, generation, events := platformMigrationFacts(t, db, ctx)
	if generation != beforeGen+1 || events != beforeEvents+1 {
		t.Fatalf("generation/events=%d/%d before=%d/%d", generation, events, beforeGen, beforeEvents)
	}
	runPlatformRoutesSQL(t, db, ctx)
	afterDigest, afterGen, afterEvents := platformMigrationFacts(t, db, ctx)
	if afterDigest != digest || afterGen != generation || afterEvents != events {
		t.Fatal("replay changed payment facts or cache generation")
	}
	if err := db.WithContext(ctx).Exec(`UPDATE payment_config_version SET app_cert_pem='changed' WHERE config_id=100`).Error; err == nil {
		t.Fatal("immutable trigger disabled after migration")
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO payment_config_version(config_id,version,private_key_ciphertext,app_cert_pem,alipay_cert_pem,root_cert_pem,notify_url,pay_methods) VALUES(100,2,'x','a','b','c','https://pay.example.test/api/v1/payment/callback/alipay/100/2',ARRAY[1]::smallint[])`).Error; err == nil {
		t.Fatal("legacy callback accepted")
	}
}
func TestPlatformCallbackMigrationRollsBackOnOutboxConflict(t *testing.T) {
	db, ctx := configFixture(t, false)
	runConfigSQL(t, db, ctx)
	seedLegacyConfig(t, db, ctx)
	runRemoveCodeSQL(t, db, ctx)
	if err := db.WithContext(ctx).Exec(`INSERT INTO system_config_cache_outbox(namespace,scope_key,generation,available_at,created_at,updated_at) VALUES('payment.config','global',3,now(),now(),now())`).Error; err != nil {
		t.Fatal(err)
	}
	digest, generation, events := platformMigrationFacts(t, db, ctx)
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(platformRoutesSQL(t)).Error }); err == nil {
		t.Fatal("outbox conflict ignored")
	}
	afterDigest, afterGen, afterEvents := platformMigrationFacts(t, db, ctx)
	if digest != afterDigest || generation != afterGen || events != afterEvents {
		t.Fatal("partial migration committed")
	}
	if err := db.WithContext(ctx).Exec(`UPDATE payment_config_version SET app_cert_pem='changed' WHERE config_id=100`).Error; err == nil {
		t.Fatal("rollback lost immutable trigger")
	}
}
func TestPlatformCallbackMigrationWithNoVersionsDoesNotAdvanceGeneration(t *testing.T) {
	db, ctx := configFixture(t, false)
	runConfigSQL(t, db, ctx)
	runRemoveCodeSQL(t, db, ctx)
	digest, generation, events := platformMigrationFacts(t, db, ctx)
	runPlatformRoutesSQL(t, db, ctx)
	afterDigest, afterGen, afterEvents := platformMigrationFacts(t, db, ctx)
	if digest != afterDigest || generation != afterGen || events != afterEvents {
		t.Fatal("empty migration changed data/generation")
	}
}

func TestPlatformCallbackMigrationRejectsPendingLegacyCallbackOrders(t *testing.T) {
	db, ctx := configFixture(t, false)
	runConfigSQL(t, db, ctx)
	seedLegacyConfig(t, db, ctx)
	runRemoveCodeSQL(t, db, ctx)
	if err := db.WithContext(ctx).Exec(`INSERT INTO payment_order(config_id,config_version,status) VALUES(100,1,1)`).Error; err != nil {
		t.Fatal(err)
	}
	beforeDigest, beforeGeneration, beforeEvents := platformMigrationFacts(t, db, ctx)
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Exec(platformRoutesSQL(t)).Error
	})
	if err == nil || !strings.Contains(err.Error(), "pending payment orders still target the legacy callback URL") {
		t.Fatalf("migration error = %v, want explicit pending legacy callback rejection", err)
	}
	afterDigest, afterGeneration, afterEvents := platformMigrationFacts(t, db, ctx)
	if beforeDigest != afterDigest || beforeGeneration != afterGeneration || beforeEvents != afterEvents {
		t.Fatal("rejected callback migration changed credential or cache facts")
	}
	var notifyURL string
	if err := db.WithContext(ctx).Raw(`SELECT notify_url FROM payment_config_version WHERE config_id=100 AND version=1`).Scan(&notifyURL).Error; err != nil {
		t.Fatal(err)
	}
	if notifyURL != "https://pay.example.test/api/v1/payment/callback/alipay/100/1" {
		t.Fatalf("rejected migration changed callback URL: %s", notifyURL)
	}
}

func TestPlatformCallbackMigrationAllowsTerminalOrders(t *testing.T) {
	for _, status := range []int16{2, 3} {
		t.Run(map[int16]string{2: "paid", 3: "closed"}[status], func(t *testing.T) {
			db, ctx := configFixture(t, false)
			runConfigSQL(t, db, ctx)
			seedLegacyConfig(t, db, ctx)
			runRemoveCodeSQL(t, db, ctx)
			if err := db.WithContext(ctx).Exec(`INSERT INTO payment_order(config_id,config_version,status) VALUES(100,1,?)`, status).Error; err != nil {
				t.Fatal(err)
			}
			runPlatformRoutesSQL(t, db, ctx)
			var after int16
			if err := db.WithContext(ctx).Raw(`SELECT status FROM payment_order WHERE config_id=100 AND config_version=1`).Scan(&after).Error; err != nil {
				t.Fatal(err)
			}
			if after != status {
				t.Fatal("callback migration changed a terminal order")
			}
		})
	}
}
