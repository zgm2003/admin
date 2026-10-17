package database_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"admin/server/internal/database/testschema"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestPaymentRedeemCodeMigrationIsIdempotentAndProtectsFacts(t *testing.T) {
	db, ctx := openPaymentRedeemSchema(t, "test_payment_redeem")
	if err := db.WithContext(ctx).Exec(`
INSERT INTO payment_recharge_package(name,amount_cents) VALUES('Redeem fixture',100);
INSERT INTO payment_wallet(user_id,balance_units) VALUES(7,-1000000000);
INSERT INTO payment_order(order_no,user_id,platform_id,package_id,package_name,amount_cents,config_id,config_version,pay_method,request_id,request_fingerprint,return_url,status,expires_at)
VALUES('PO-R-original',7,1,1,'Redeem fixture',100,1,1,1,'00000000-0000-4000-8000-000000000001',decode(repeat('01',32),'hex'),'https://app.example.test/return',1,CURRENT_TIMESTAMP+interval '1 hour');
INSERT INTO payment_transaction(transaction_no,user_id,platform_id,amount_units,balance_after_units,order_id) VALUES('PT-original',7,1,100000000,-900000000,1);
`).Error; err != nil {
		t.Fatal(err)
	}
	applyPaymentRedeemMigration(t, db, ctx)
	applyPaymentRedeemMigration(t, db, ctx)
	var amount int64
	if err := db.WithContext(ctx).Raw(`SELECT balance_after_units FROM payment_transaction WHERE transaction_no='PT-original'`).Scan(&amount).Error; err != nil || amount != -900000000 {
		t.Fatalf("original negative order transaction altered: %d %v", amount, err)
	}

	if err := db.WithContext(ctx).Exec(`
INSERT INTO payment_redeem_code_batch(batch_no,request_id,request_fingerprint,amount_cents,quantity,created_by)
VALUES ('RB20261017000000000000000001','00000000-0000-4000-8000-000000000001',decode(repeat('01',32),'hex'),100,1,7);
INSERT INTO payment_redeem_code(batch_id,code_hash,code_ciphertext,status)
VALUES (1,decode(repeat('02',32),'hex'),'ciphertext',1);
`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`UPDATE payment_redeem_code_batch SET amount_cents=200 WHERE id=1`).Error; err == nil {
		t.Fatal("batch immutable update accepted")
	}
	if err := db.WithContext(ctx).Exec(`UPDATE payment_redeem_code SET batch_id=2 WHERE id=1`).Error; err == nil {
		t.Fatal("code identity update accepted")
	}
	if err := db.WithContext(ctx).Exec(`UPDATE payment_redeem_code SET status=2,used_by=7,used_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE id=1`).Error; err != nil {
		t.Fatalf("valid code redemption update rejected: %v", err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO payment_transaction(transaction_no,user_id,platform_id,amount_units,balance_after_units,redeem_code_id) VALUES('PT-R1',7,1,100000000,100000000,1)`).Error; err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO payment_redeem_code_batch(batch_no,request_id,request_fingerprint,amount_cents,quantity,created_by) VALUES('RB2','00000000-0000-4000-8000-000000000002',decode(repeat('03',32),'hex'),0,1,7)`,
		`INSERT INTO payment_redeem_code(batch_id,code_hash,code_ciphertext,status) VALUES(1,decode(repeat('04',32),'hex'),'x',4)`,
		`INSERT INTO payment_transaction(transaction_no,user_id,platform_id,amount_units,balance_after_units,order_id,redeem_code_id) VALUES('PT-R2',7,1,100000000,200000000,1,1)`,
		`INSERT INTO payment_transaction(transaction_no,user_id,platform_id,amount_units,balance_after_units) VALUES('PT-R3',7,1,100000000,200000000)`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err == nil {
			t.Fatalf("invalid payment fact accepted: %s", statement)
		}
	}
	if err := db.WithContext(ctx).Exec(`DELETE FROM payment_redeem_code WHERE id=1`).Error; err == nil {
		t.Fatal("redeem code deletion accepted")
	}
	if err := db.WithContext(ctx).Exec(`UPDATE payment_transaction SET amount_units=200000000 WHERE id=1`).Error; err == nil {
		t.Fatal("transaction immutable update accepted")
	}
}

func TestPaymentRedeemCodeMigrationRejectsConflictingExistingTable(t *testing.T) {
	db, ctx := openPaymentRedeemSchema(t, "test_payment_redeem_conflict")
	if err := db.WithContext(ctx).Exec(`CREATE TABLE payment_redeem_code(id bigint PRIMARY KEY)`).Error; err != nil {
		t.Fatal(err)
	}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(readPaymentRedeemMigration(t)).Error })
	if err == nil {
		t.Fatal("incompatible redeem table accepted")
	}
	var count int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name='payment_redeem_code_batch'`).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed migration left redeem batch table")
	}
}

func TestPaymentRedeemCodeMigrationRejectsDriftWithoutRepair(t *testing.T) {
	for name, mutation := range map[string]string{
		"amount_check":     `ALTER TABLE payment_redeem_code_batch DROP CONSTRAINT ck_payment_redeem_batch_amount; ALTER TABLE payment_redeem_code_batch ADD CONSTRAINT ck_payment_redeem_batch_amount CHECK(amount_cents>=0)`,
		"column_default":   `ALTER TABLE payment_redeem_code ALTER COLUMN status SET DEFAULT 2`,
		"source_check":     `ALTER TABLE payment_transaction DROP CONSTRAINT ck_payment_transaction_source_xor; ALTER TABLE payment_transaction ADD CONSTRAINT ck_payment_transaction_source_xor CHECK(order_id IS NULL OR redeem_code_id IS NULL)`,
		"missing_index":    `DROP INDEX ux_payment_redeem_code_hash`,
		"wrong_index":      `DROP INDEX ix_payment_redeem_code_used_by; CREATE INDEX ix_payment_redeem_code_used_by ON payment_redeem_code(used_by,used_at,id) WHERE status=2`,
		"wrong_fk_column":  `ALTER TABLE payment_transaction DROP CONSTRAINT fk_payment_transaction_redeem_code; ALTER TABLE payment_transaction ADD CONSTRAINT fk_payment_transaction_redeem_code FOREIGN KEY(order_id) REFERENCES payment_redeem_code(id) ON DELETE RESTRICT`,
		"disabled_trigger": `ALTER TABLE payment_redeem_code DISABLE TRIGGER payment_redeem_code_guard`,
		"function_body":    `CREATE OR REPLACE FUNCTION payment_redeem_code_guard() RETURNS trigger LANGUAGE plpgsql AS $body$ BEGIN RETURN NEW; END $body$`,
	} {
		t.Run(name, func(t *testing.T) {
			db, ctx := openPaymentRedeemSchema(t, "test_payment_redeem_drift")
			applyPaymentRedeemMigration(t, db, ctx)
			if err := db.WithContext(ctx).Exec(mutation).Error; err != nil {
				t.Fatal(err)
			}
			before := paymentRedeemSchemaDigest(t, db, ctx)
			err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(readPaymentRedeemMigration(t)).Error })
			if err == nil {
				t.Fatal("schema drift accepted")
			}
			if after := paymentRedeemSchemaDigest(t, db, ctx); after != before {
				t.Fatal("failed migration silently repaired schema drift")
			}
		})
	}
}

func paymentRedeemSchemaDigest(t *testing.T, db *gorm.DB, ctx context.Context) string {
	t.Helper()
	var digest string
	err := db.WithContext(ctx).Raw(`SELECT md5(jsonb_build_object(
'columns',(SELECT jsonb_agg(to_jsonb(a) ORDER BY attrelid,attnum) FROM pg_attribute a WHERE attrelid IN ('payment_redeem_code_batch'::regclass,'payment_redeem_code'::regclass,'payment_transaction'::regclass)),
'constraints',(SELECT jsonb_agg(to_jsonb(c) ORDER BY conrelid,conname) FROM pg_constraint c WHERE conrelid IN ('payment_redeem_code_batch'::regclass,'payment_redeem_code'::regclass,'payment_transaction'::regclass)),
'indexes',(SELECT jsonb_agg(indexdef ORDER BY indexname) FROM pg_indexes WHERE schemaname=current_schema()),
'triggers',(SELECT jsonb_agg(to_jsonb(tr) ORDER BY tgrelid,tgname) FROM pg_trigger tr WHERE tgrelid IN ('payment_redeem_code_batch'::regclass,'payment_redeem_code'::regclass,'payment_transaction'::regclass)),
'functions',(SELECT jsonb_agg(pg_get_functiondef(oid) ORDER BY proname) FROM pg_proc WHERE oid IN (to_regprocedure('payment_redeem_batch_guard()'),to_regprocedure('payment_redeem_code_guard()'),to_regprocedure('payment_transaction_immutable_guard()')))
)::text)`).Scan(&digest).Error
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func openPaymentRedeemSchema(t *testing.T, prefix string) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	db, ctx := openPaymentRedeemFixtureSchema(t, prefix)
	return db, ctx
}

func openPaymentRedeemFixtureSchema(t *testing.T, prefix string) (*gorm.DB, context.Context) {
	t.Helper()
	_ = godotenv.Load(filepath.Join(repoRoot(t), "server", ".env"))
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_DSN is not configured")
	}
	db, ctx := testschema.Open(t, dsn, prefix)
	for _, statement := range []string{
		`CREATE TABLE user_account(id bigint PRIMARY KEY)`,
		`CREATE TABLE permission_auth_platform(id bigint PRIMARY KEY)`,
		`CREATE TABLE payment_transaction(id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,transaction_no varchar(32) NOT NULL,user_id bigint NOT NULL,platform_id bigint NOT NULL,amount_units bigint NOT NULL,balance_after_units bigint NOT NULL,order_id bigint,redeem_code_id bigint,created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE payment_recharge_package(id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,name varchar(128) NOT NULL,amount_cents bigint NOT NULL,is_enabled smallint NOT NULL DEFAULT 0,created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,deleted_at timestamptz)`,
		`CREATE TABLE payment_wallet(user_id bigint PRIMARY KEY,balance_units bigint NOT NULL,created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE payment_order(id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,order_no varchar(32) NOT NULL,user_id bigint NOT NULL,platform_id bigint NOT NULL,package_id bigint NOT NULL,package_name varchar(128) NOT NULL,amount_cents bigint NOT NULL,config_id bigint NOT NULL,config_version bigint NOT NULL,pay_method smallint NOT NULL,request_id uuid NOT NULL,request_fingerprint bytea NOT NULL,return_url varchar(2048) NOT NULL,status smallint NOT NULL,expires_at timestamptz NOT NULL)`,
		`ALTER TABLE payment_transaction ADD CONSTRAINT ck_payment_transaction_payment_source CHECK (order_id IS NOT NULL AND redeem_code_id IS NULL), ADD CONSTRAINT fk_payment_transaction_order FOREIGN KEY(order_id) REFERENCES payment_order(id) ON DELETE RESTRICT`,
		`INSERT INTO user_account(id) VALUES(7)`,
		`INSERT INTO permission_auth_platform(id) VALUES(1)`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db, ctx
}

func applyPaymentRedeemMigration(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(readPaymentRedeemMigration(t)).Error }); err != nil {
		t.Fatal(err)
	}
}

func readPaymentRedeemMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-10-17-payment-redeem-code.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}
