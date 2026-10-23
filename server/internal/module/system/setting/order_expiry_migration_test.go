package setting

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gorm.io/gorm"
)

func orderExpiryMigrationFixture(t *testing.T) (*gorm.DB, context.Context, string) {
	t.Helper()
	db, ctx := openSettingDatabase(t)
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE payment_order (
 id BIGINT PRIMARY KEY, status SMALLINT NOT NULL, expires_at TIMESTAMPTZ NOT NULL,
 next_check_at TIMESTAMPTZ, check_lease_until TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL, amount_cents BIGINT NOT NULL DEFAULT 1000,
 paid_at TIMESTAMPTZ, credited_at TIMESTAMPTZ, closed_at TIMESTAMPTZ
);
INSERT INTO payment_order(id,status,expires_at,next_check_at,updated_at,created_at,check_lease_until) VALUES
 (1,1,NOW()+INTERVAL '1 minute',NOW()+INTERVAL '3 minutes',NOW()-INTERVAL '1 minute',NOW()-INTERVAL '29 minutes',NULL),
 (2,1,NOW()-INTERVAL '1 minute',NOW()+INTERVAL '3 minutes',NOW()-INTERVAL '2 minutes',NOW()-INTERVAL '31 minutes',NULL),
 (3,1,NOW()-INTERVAL '1 minute',NOW()+INTERVAL '3 minutes',NOW(),NOW()-INTERVAL '31 minutes',NULL),
 (4,2,NOW()-INTERVAL '1 minute',NOW()+INTERVAL '3 minutes',NOW(),NOW()-INTERVAL '31 minutes',NULL),
 (5,3,NOW()-INTERVAL '1 minute',NULL,NOW(),NOW()-INTERVAL '31 minutes',NULL),
 (6,1,NOW()+INTERVAL '1 minute',NOW()+INTERVAL '3 minutes',NOW()-INTERVAL '1 minute',NOW()-INTERVAL '29 minutes',NOW()+INTERVAL '10 minutes'),
 (7,1,NOW()+INTERVAL '10 minutes',NULL,NOW(),NOW(),NULL),
 (8,1,NOW()+INTERVAL '10 minutes',NOW()+INTERVAL '3 minutes',NOW(),NOW(),NULL);
INSERT INTO system_setting(setting_key,value,value_type,description,status,is_builtin,created_at,updated_at)
VALUES('custom.preserved','99',2,'Custom description',0,0,NOW(),NOW());
`).Error; err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../../docs/database/2026-10-23-payment-order-expiry.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return db, ctx, string(raw)
}

func orderExpirySnapshot(t *testing.T, db *gorm.DB, ctx context.Context, normalize bool) string {
	t.Helper()
	projection := "to_jsonb(o)"
	if normalize {
		projection += " - 'next_check_at' - 'updated_at'"
	}
	var snapshot string
	if err := db.WithContext(ctx).Raw("SELECT jsonb_agg(" + projection + " ORDER BY id)::text FROM payment_order o").Scan(&snapshot).Error; err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestOrderExpiryMigrationSeedsAndReplaysWithoutChangingExistingExpiry(t *testing.T) {
	db, ctx, sql := orderExpiryMigrationFixture(t)
	orders := orderExpirySnapshot(t, db, ctx, true)
	var unchanged string
	query := `SELECT jsonb_agg(to_jsonb(o) ORDER BY id)::text FROM payment_order o WHERE id NOT IN (1,2)`
	if err := db.WithContext(ctx).Raw(query).Scan(&unchanged).Error; err != nil {
		t.Fatal(err)
	}
	apply := func() error {
		return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(sql).Error })
	}
	if err := apply(); err != nil {
		t.Fatal(err)
	}
	var row Model
	if err := db.WithContext(ctx).Where("setting_key='payment.order.expiry_minutes'").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Value != "30" || row.ValueType != 2 || row.Status != 1 || row.IsBuiltin != 1 || row.Description != "支付订单有效期（分钟，1～1440，仅影响新订单）" {
		t.Fatalf("wrong expiry setting: %+v", row)
	}
	if got := orderExpirySnapshot(t, db, ctx, true); got != orders {
		t.Fatal("migration changed existing financial facts or expires_at")
	}
	var count int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM payment_order WHERE id IN (1,2) AND next_check_at=expires_at`).Scan(&count).Error; err != nil || count != 2 {
		t.Fatalf("late checks not repaired: count=%d err=%v", count, err)
	}
	var afterUnchanged string
	if err := db.WithContext(ctx).Raw(query).Scan(&afterUnchanged).Error; err != nil || afterUnchanged != unchanged {
		t.Fatal("migration changed retry backoff, leased, paid, closed, or ordinary orders", err)
	}
	assertSettingGeneration(t, db, ctx, "global", 2)
	assertSettingOutbox(t, db, ctx, "global", 1, 2, false)
	settings := descriptionSnapshot(t, db, ctx, false)
	allOrders := orderExpirySnapshot(t, db, ctx, false)
	if err := apply(); err != nil {
		t.Fatal(err)
	}
	if descriptionSnapshot(t, db, ctx, false) != settings || orderExpirySnapshot(t, db, ctx, false) != allOrders {
		t.Fatal("replay rewrote settings or orders")
	}
	assertSettingGeneration(t, db, ctx, "global", 2)
	assertSettingOutbox(t, db, ctx, "global", 1, 2, false)
	for _, change := range []string{
		`value='0'`, `value='1441'`, `value='+30'`, `value=' 30 '`, `value='1.5'`, `value_type=1`, `status=0`, `is_builtin=0`, `deleted_at=NOW()`,
	} {
		if err := db.WithContext(ctx).Exec(`UPDATE system_setting SET ` + change + ` WHERE setting_key='payment.order.expiry_minutes'`).Error; err == nil {
			t.Fatalf("invalid persisted setting accepted: %s", change)
		}
	}
}

func TestOrderExpiryMigrationPreservesConfiguredValue(t *testing.T) {
	db, ctx, sql := orderExpiryMigrationFixture(t)
	if err := db.WithContext(ctx).Exec(`INSERT INTO system_setting(setting_key,value,value_type,description,status,is_builtin,created_at,updated_at)
VALUES('payment.order.expiry_minutes','45',2,'Maintainer custom note',1,1,NOW(),NOW())`).Error; err != nil {
		t.Fatal(err)
	}
	before := descriptionSnapshot(t, db, ctx, false)
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(sql).Error }); err != nil {
		t.Fatal(err)
	}
	if got := descriptionSnapshot(t, db, ctx, false); got != before {
		t.Fatal("existing valid setting was overwritten")
	}
	assertSettingGeneration(t, db, ctx, "global", 1)
	assertSettingOutbox(t, db, ctx, "global", 0, 0, false)
}

func TestOrderExpiryMigrationRollsBackOnInvalidState(t *testing.T) {
	for _, scenario := range []string{"invalid setting", "missing state", "outbox conflict", "constraint drift"} {
		t.Run(scenario, func(t *testing.T) {
			db, ctx, sql := orderExpiryMigrationFixture(t)
			change := map[string]string{
				"invalid setting":  `INSERT INTO system_setting(setting_key,value,value_type,description,status,is_builtin,created_at,updated_at) VALUES('payment.order.expiry_minutes','0',2,'Invalid',1,1,NOW(),NOW())`,
				"missing state":    `DELETE FROM system_config_cache_state`,
				"outbox conflict":  `INSERT INTO system_config_cache_outbox(namespace,scope_key,generation) VALUES('system.setting','global',2)`,
				"constraint drift": `ALTER TABLE system_setting ADD CONSTRAINT ck_system_setting_payment_order_expiry CHECK (true)`,
			}[scenario]
			if err := db.WithContext(ctx).Exec(change).Error; err != nil {
				t.Fatal(err)
			}
			beforeSettings, beforeOrders := descriptionSnapshot(t, db, ctx, false), orderExpirySnapshot(t, db, ctx, false)
			if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(sql).Error }); err == nil {
				t.Fatal("invalid migration was accepted")
			}
			if descriptionSnapshot(t, db, ctx, false) != beforeSettings || orderExpirySnapshot(t, db, ctx, false) != beforeOrders {
				t.Fatal("migration failure left partial changes")
			}
		})
	}
}
