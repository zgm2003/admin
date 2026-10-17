package database_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"admin/server/internal/database/testschema"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPaymentOrderWalletMigrationIsIdempotentAndEnforcesFacts(t *testing.T) {
	db, ctx := openPaymentOrderWalletSchema(t, "test_payment_order_wallet")
	applyPaymentOrderWalletMigration(t, db, ctx)
	applyPaymentOrderWalletMigration(t, db, ctx)

	for _, table := range []string{"payment_recharge_package", "payment_wallet", "payment_order", "payment_transaction", "payment_callback_event"} {
		var count int64
		if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name=?`, table).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("table %s count=%d", table, count)
		}
	}

	if err := db.WithContext(ctx).Exec(`
INSERT INTO payment_recharge_package(name,amount_cents,is_enabled) VALUES('Credits',100,1);
INSERT INTO payment_wallet(user_id,balance_units) VALUES(7,-1000000000);
INSERT INTO payment_order(order_no,user_id,platform_id,package_id,package_name,amount_cents,config_id,config_version,pay_method,request_id,request_fingerprint,return_url,status,expires_at)
VALUES('PO20261016000000000000000001',7,1,1,'Credits',100,1,1,1,'00000000-0000-4000-8000-000000000001',decode(repeat('01',32),'hex'),'https://app.example.test/return',1,CURRENT_TIMESTAMP + interval '1 hour');
`).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.WithContext(ctx).Exec(`UPDATE payment_order SET status=3,closed_at=CURRENT_TIMESTAMP WHERE id=1`).Error; err != nil {
		t.Fatal(err)
	}
	// A late trusted payment may recover a closed order while retaining the
	// original close timestamp for the audit trail.
	if err := db.WithContext(ctx).Exec(`UPDATE payment_order SET status=2,paid_at=CURRENT_TIMESTAMP,credited_at=CURRENT_TIMESTAMP WHERE id=1`).Error; err != nil {
		t.Fatalf("late payment recovery rejected: %v", err)
	}

	if err := db.WithContext(ctx).Exec(`INSERT INTO payment_transaction(transaction_no,user_id,platform_id,amount_units,balance_after_units,order_id) VALUES('PT-1',7,1,100000000,-900000000,1)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO payment_callback_event(config_id,config_version,dedupe_key,payload,process_status) VALUES(1,1,decode(repeat('02',32),'hex'),'[]'::jsonb,1)`).Error; err == nil {
		t.Fatal("non-object callback payload was accepted")
	}

	for _, statement := range []string{
		`INSERT INTO payment_transaction(transaction_no,user_id,platform_id,amount_units,balance_after_units,order_id) VALUES('PT-zero',7,1,0,-900000000,1)`,
		`INSERT INTO payment_transaction(transaction_no,user_id,platform_id,amount_units,balance_after_units) VALUES('PT-no-source',7,1,100000000,-800000000)`,
		`INSERT INTO payment_transaction(transaction_no,user_id,platform_id,amount_units,balance_after_units,order_id,redeem_code_id) VALUES('PT-two-sources',7,1,100000000,-800000000,1,1)`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err == nil {
			t.Fatalf("invalid transaction accepted: %s", statement)
		}
	}

	if err := db.WithContext(ctx).Exec(`DELETE FROM payment_wallet WHERE user_id=7`).Error; err == nil {
		t.Fatal("financial wallet deletion accepted")
	}
	if err := db.WithContext(ctx).Exec(`TRUNCATE payment_transaction`).Error; err == nil {
		t.Fatal("financial transaction truncate accepted")
	}
}

func TestPaymentOrderWalletMigrationRollsBackIncompatibleExistingTable(t *testing.T) {
	db, ctx := openPaymentOrderWalletSchema(t, "test_payment_order_wallet_conflict")
	if err := db.WithContext(ctx).Exec(`CREATE TABLE payment_order(id bigint PRIMARY KEY)`).Error; err != nil {
		t.Fatal(err)
	}
	sql := readPaymentOrderWalletMigration(t)
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(sql).Error })
	if err == nil {
		t.Fatal("incompatible payment_order table accepted")
	}
	for _, table := range []string{"payment_recharge_package", "payment_wallet", "payment_transaction", "payment_callback_event"} {
		var exists bool
		if err := db.WithContext(ctx).Raw(`SELECT to_regclass(current_schema()||'.'||?) IS NOT NULL`, table).Scan(&exists).Error; err != nil {
			t.Fatal(err)
		}
		if exists {
			t.Fatalf("failed migration left table %s", table)
		}
	}
}

func TestPaymentOrderWalletMigrationRejectsSchemaDriftWithoutRepairingIt(t *testing.T) {
	for name, change := range map[string]string{
		"column type":         `ALTER TABLE payment_order ALTER COLUMN package_name TYPE text`,
		"column nullability":  `ALTER TABLE payment_wallet ALTER COLUMN balance_units DROP NOT NULL`,
		"column default":      `ALTER TABLE payment_wallet ALTER COLUMN balance_units SET DEFAULT 100`,
		"column identity":     `ALTER TABLE payment_order ALTER COLUMN id SET GENERATED ALWAYS`,
		"extra balance check": `ALTER TABLE payment_wallet ADD CONSTRAINT injected_positive_balance CHECK(balance_units >= 0)`,
		"missing check":       `ALTER TABLE payment_order DROP CONSTRAINT ck_payment_order_amount`,
		"weakened check":      `ALTER TABLE payment_order DROP CONSTRAINT ck_payment_order_amount; ALTER TABLE payment_order ADD CONSTRAINT ck_payment_order_amount CHECK(amount_cents>=0)`,
		"unvalidated check":   `ALTER TABLE payment_order DROP CONSTRAINT ck_payment_order_amount; ALTER TABLE payment_order ADD CONSTRAINT ck_payment_order_amount CHECK(amount_cents>0) NOT VALID`,
		"foreign key":         `ALTER TABLE payment_order DROP CONSTRAINT fk_payment_order_user`,
		"foreign key action":  `ALTER TABLE payment_order DROP CONSTRAINT fk_payment_order_user; ALTER TABLE payment_order ADD CONSTRAINT fk_payment_order_user FOREIGN KEY(user_id) REFERENCES user_account(id) ON DELETE CASCADE`,
		"missing index":       `DROP INDEX ux_payment_order_request`,
		"index uniqueness":    `DROP INDEX ux_payment_order_request; CREATE INDEX ux_payment_order_request ON payment_order(user_id,platform_id,request_id)`,
		"index direction":     `DROP INDEX ix_payment_order_user_created; CREATE INDEX ix_payment_order_user_created ON payment_order(user_id,created_at,id)`,
		"index predicate":     `DROP INDEX ix_payment_order_reconcile_check; CREATE INDEX ix_payment_order_reconcile_check ON payment_order(next_check_at,id) WHERE status=2 AND next_check_at IS NOT NULL`,
		"disabled guard":      `ALTER TABLE payment_wallet DISABLE TRIGGER payment_wallet_no_delete`,
		"guard body":          `CREATE OR REPLACE FUNCTION payment_fact_no_delete_guard() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END $$`,
	} {
		t.Run(name, func(t *testing.T) {
			db, ctx := openPaymentOrderWalletSchema(t, "test_payment_order_wallet_drift")
			applyPaymentOrderWalletMigration(t, db, ctx)
			if err := db.WithContext(ctx).Exec(change).Error; err != nil {
				t.Fatal(err)
			}
			before := paymentSchemaDigest(t, db, ctx)
			err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				return tx.Exec(readPaymentOrderWalletMigration(t)).Error
			})
			if err == nil {
				t.Error("schema drift accepted instead of rejected")
			}
			if after := paymentSchemaDigest(t, db, ctx); after != before {
				t.Error("schema conflict was silently repaired or left partial DDL")
			}
		})
	}
}

func TestPaymentOrderWalletMigrationProtectsImmutableFactsAndSignedBalances(t *testing.T) {
	db, ctx := openPaymentOrderWalletSchema(t, "test_payment_order_wallet_guards")
	applyPaymentOrderWalletMigration(t, db, ctx)
	seedPaymentOrderWalletFacts(t, db, ctx)
	for name, statement := range map[string]string{
		"wallet owner":                         `UPDATE payment_wallet SET user_id=8 WHERE user_id=7`,
		"wallet created at":                    `UPDATE payment_wallet SET created_at=created_at+interval '1 second' WHERE user_id=7`,
		"order snapshot":                       `UPDATE payment_order SET amount_cents=301 WHERE id=1`,
		"order identity":                       `UPDATE payment_order SET user_id=8 WHERE id=1`,
		"order request":                        `UPDATE payment_order SET request_id='00000000-0000-4000-8000-000000000003' WHERE id=1`,
		"order credential":                     `UPDATE payment_order SET config_version=2 WHERE id=1`,
		"order return url":                     `UPDATE payment_order SET return_url='https://different.example.test/' WHERE id=1`,
		"transaction update":                   `UPDATE payment_transaction SET balance_after_units=-600000000 WHERE id=1`,
		"transaction delete":                   `DELETE FROM payment_transaction WHERE id=1`,
		"transaction truncate":                 `TRUNCATE payment_transaction`,
		"callback payload":                     `UPDATE payment_callback_event SET payload=jsonb_set(payload,'{totalAmount}','"4.00"') WHERE id=1`,
		"callback credential":                  `UPDATE payment_callback_event SET config_version=2 WHERE id=1`,
		"callback dedupe":                      `UPDATE payment_callback_event SET dedupe_key=decode(repeat('03',32),'hex') WHERE id=1`,
		"callback relink":                      `UPDATE payment_callback_event SET order_id=NULL WHERE id=1`,
		"package amount overflow":              `INSERT INTO payment_recharge_package(name,amount_cents) VALUES('Overflow',9223372036855)`,
		"transaction before balance underflow": `INSERT INTO payment_transaction(transaction_no,user_id,platform_id,amount_units,balance_after_units,order_id) VALUES('PT-underflow',7,1,1,-9223372036854775808,2)`,
		"transaction missing wallet":           `INSERT INTO payment_transaction(transaction_no,user_id,platform_id,amount_units,balance_after_units,order_id) VALUES('PT-no-wallet',8,1,1,1,2)`,
		"callback incomplete payload":          `INSERT INTO payment_callback_event(config_id,config_version,dedupe_key,payload,process_status) VALUES(1,1,decode(repeat('04',32),'hex'),'{}',1)`,
		"callback extra sensitive field":       `INSERT INTO payment_callback_event(config_id,config_version,dedupe_key,payload,process_status) SELECT 1,1,decode(repeat('04',32),'hex'),payload || '{"sign":"secret"}',1 FROM payment_callback_event WHERE id=1`,
	} {
		t.Run(name, func(t *testing.T) {
			rejected := false
			err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				err := tx.Exec(statement).Error
				rejected = err != nil
				if err != nil {
					return err
				}
				return errors.New("rollback invalid fact accepted by migration")
			})
			if err == nil || !rejected {
				t.Fatal("immutable or invalid payment fact accepted")
			}
		})
	}
	// Exact signed range is legal for a wallet, including both endpoints.
	for _, balance := range []int64{-9223372036854775808, 0, 9223372036854775807} {
		if err := db.WithContext(ctx).Exec(`UPDATE payment_wallet SET balance_units=? WHERE user_id=7`, balance).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.WithContext(ctx).Exec(`UPDATE payment_callback_event SET process_status=2,error_code=NULL,updated_at=CURRENT_TIMESTAMP WHERE id=1`).Error; err != nil {
		t.Fatalf("legitimate callback processing rejected: %v", err)
	}
	// A signed query is allowed to omit its payment timestamp. Never invent one.
	if err := db.WithContext(ctx).Exec(`UPDATE payment_order SET status=2,provider_trade_no='TRADE-2',credited_at=CURRENT_TIMESTAMP WHERE id=2`).Error; err != nil {
		t.Fatalf("trusted payment without reported paid_at rejected: %v", err)
	}
	before := paymentSchemaDigest(t, db, ctx)
	rowsBefore := paymentRowsDigest(t, db, ctx)
	applyPaymentOrderWalletMigration(t, db, ctx)
	if paymentSchemaDigest(t, db, ctx) != before || paymentRowsDigest(t, db, ctx) != rowsBefore {
		t.Fatal("migration replay changed financial rows or guard metadata")
	}
}

func paymentRowsDigest(t *testing.T, db *gorm.DB, ctx context.Context) string {
	t.Helper()
	var digest string
	if err := db.WithContext(ctx).Raw(`SELECT md5(jsonb_build_object(
 'packages',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM payment_recharge_package p),
 'wallets',(SELECT jsonb_agg(to_jsonb(w) ORDER BY user_id) FROM payment_wallet w),
 'orders',(SELECT jsonb_agg(to_jsonb(o) ORDER BY id) FROM payment_order o),
 'transactions',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM payment_transaction t),
 'callbacks',(SELECT jsonb_agg(to_jsonb(c) ORDER BY id) FROM payment_callback_event c)
 )::text)`).Scan(&digest).Error; err != nil {
		t.Fatal(err)
	}
	return digest
}

func seedPaymentOrderWalletFacts(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	if err := db.WithContext(ctx).Exec(`
INSERT INTO user_account(id) VALUES(8);
INSERT INTO payment_config_version(config_id,version) VALUES(1,2);
INSERT INTO payment_recharge_package(name,amount_cents,is_enabled) VALUES('Credits',300,1);
INSERT INTO payment_wallet(user_id,balance_units) VALUES(7,-700000000);
INSERT INTO payment_order(order_no,user_id,platform_id,package_id,package_name,amount_cents,config_id,config_version,pay_method,request_id,request_fingerprint,return_url,status,provider_trade_no,expires_at,paid_at,credited_at)
VALUES('PO-1',7,1,1,'Credits',300,1,1,1,'00000000-0000-4000-8000-000000000001',decode(repeat('01',32),'hex'),'https://app.example.test/return',2,'TRADE-1',CURRENT_TIMESTAMP+interval '1 hour',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
('PO-2',7,1,1,'Credits',300,1,1,1,'00000000-0000-4000-8000-000000000002',decode(repeat('02',32),'hex'),'https://app.example.test/return',1,NULL,CURRENT_TIMESTAMP+interval '1 hour',NULL,NULL);
INSERT INTO payment_transaction(transaction_no,user_id,platform_id,amount_units,balance_after_units,order_id) VALUES('PT-1',7,1,300000000,-700000000,1);
INSERT INTO payment_callback_event(config_id,config_version,dedupe_key,order_id,payload,process_status)
VALUES(1,1,decode(repeat('01',32),'hex'),1,'{"notifyId":"NOTIFY-1","orderNo":"PO-1","tradeNo":"TRADE-1","tradeStatus":"TRADE_SUCCESS","appId":"20260001","sellerId":"20880001","totalAmount":"3.00","paidAt":null}',1);
`).Error; err != nil {
		t.Fatal(err)
	}
}

func paymentSchemaDigest(t *testing.T, db *gorm.DB, ctx context.Context) string {
	t.Helper()
	var result string
	if err := db.WithContext(ctx).Raw(`SELECT md5(jsonb_build_object(
 'columns',(SELECT jsonb_agg(jsonb_build_array(c.relname,a.attname,a.atttypid,a.atttypmod,a.attnotnull,a.attidentity,pg_get_expr(d.adbin,d.adrelid)) ORDER BY c.relname,a.attnum) FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum WHERE n.nspname=current_schema() AND c.relname LIKE 'payment_%' AND c.relkind='r' AND a.attnum>0 AND NOT a.attisdropped),
 'constraints',(SELECT jsonb_agg(jsonb_build_array(c.conname,c.conrelid,pg_get_constraintdef(c.oid),c.convalidated) ORDER BY c.conname,c.conrelid) FROM pg_constraint c JOIN pg_class t ON t.oid=c.conrelid JOIN pg_namespace n ON n.oid=t.relnamespace WHERE n.nspname=current_schema() AND t.relname LIKE 'payment_%'),
 'indexes',(SELECT jsonb_agg(to_jsonb(i) ORDER BY indexname) FROM pg_indexes i WHERE schemaname=current_schema() AND tablename LIKE 'payment_%'),
 'triggers',(SELECT jsonb_agg(jsonb_build_array(t.tgname,t.tgrelid,t.tgtype,t.tgenabled,t.tgfoid,pg_get_triggerdef(t.oid)) ORDER BY t.tgname) FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=current_schema() AND c.relname LIKE 'payment_%' AND NOT t.tgisinternal),
 'guards',(SELECT jsonb_agg(jsonb_build_array(p.proname,p.prosrc,p.prosecdef,p.proconfig) ORDER BY p.proname) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname=current_schema() AND p.proname LIKE 'payment_%')
)::text)`).Scan(&result).Error; err != nil {
		t.Fatal(err)
	}
	return result
}

func openPaymentOrderWalletSchema(t *testing.T, prefix string) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	_ = godotenv.Load(filepath.Join(repoRoot(t), "server", ".env"))
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_DSN is not configured")
	}
	db, ctx := testschema.Open(t, dsn, prefix)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	for _, statement := range []string{
		`CREATE TABLE user_account(id bigint PRIMARY KEY)`,
		`CREATE TABLE permission_auth_platform(id bigint PRIMARY KEY)`,
		`CREATE TABLE payment_config(id bigint PRIMARY KEY)`,
		`CREATE TABLE payment_config_version(config_id bigint NOT NULL,version bigint NOT NULL,PRIMARY KEY(config_id,version),FOREIGN KEY(config_id) REFERENCES payment_config(id))`,
		`INSERT INTO user_account(id) VALUES(7)`,
		`INSERT INTO permission_auth_platform(id) VALUES(1)`,
		`INSERT INTO payment_config(id) VALUES(1)`,
		`INSERT INTO payment_config_version(config_id,version) VALUES(1,1)`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db, ctx
}

func applyPaymentOrderWalletMigration(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	sql := readPaymentOrderWalletMigration(t)
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(sql).Error }); err != nil {
		t.Fatal(err)
	}
}

func readPaymentOrderWalletMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-10-16-payment-order-wallet.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}
