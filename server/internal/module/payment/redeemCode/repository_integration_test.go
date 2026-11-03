package redeemcode

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	settings "admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"admin/server/internal/module/payment/transaction"
	"admin/server/internal/module/payment/wallet"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestRepositoryRedeemPersistsCreditAndReplaysOneLedger(t *testing.T) {
	db, ctx := redemptionDB(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	const code = "ABCDEFGHJKLMNPQRSTU2"
	const nextCode = "ABCDEFGHJKLMNPQRSTU3"
	if err := db.WithContext(ctx).Exec(`
INSERT INTO user_account(id) VALUES(7),(8);
INSERT INTO permission_auth_platform(id) VALUES(1);
INSERT INTO payment_wallet(user_id,balance_units,created_at,updated_at) VALUES(7,-1000000000,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
INSERT INTO payment_redeem_code_batch(id,batch_no,request_id,request_fingerprint,amount_cents,quantity,created_by,created_at,updated_at)
VALUES(1,'RC-integration','00000000-0000-4000-8000-000000000001',decode(repeat('01',32),'hex'),300,2,7,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
INSERT INTO payment_redeem_code(id,batch_id,code,status,created_at,updated_at)
VALUES(1,1,'ABCDEFGHJKLMNPQRSTU2',1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
      (2,1,'ABCDEFGHJKLMNPQRSTU3',1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);`).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	first, err := repo.Redeem(ctx, 7, 1, code, now)
	if err != nil {
		t.Fatal(err)
	}
	assertRedeemLedger(t, first, 1, -1_000_000_000, -700_000_000)
	var persisted transaction.Model
	if err := db.WithContext(ctx).First(&persisted, first.ID).Error; err != nil {
		t.Fatal(err)
	}
	assertRedeemLedger(t, persisted, 1, -1_000_000_000, -700_000_000)

	replayed, err := repo.Redeem(ctx, 7, 1, code, now.Add(time.Second))
	if err != nil || replayed.ID != first.ID || replayed.TransactionNo != first.TransactionNo || !replayed.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("replay=%+v first=%+v err=%v", replayed, first, err)
	}
	assertRedeemLedger(t, replayed, 1, -1_000_000_000, -700_000_000)
	assertRedemptionState(t, db, ctx, -700_000_000, 1)

	if _, err := repo.Redeem(ctx, 8, 1, code, now.Add(2*time.Second)); !errors.Is(err, ErrCodeUnavailable) {
		t.Fatalf("another user's replay error=%v, want code unavailable", err)
	}
	var foreignWallets int64
	if err := db.WithContext(ctx).Model(&wallet.Model{}).Where("user_id = ?", 8).Count(&foreignWallets).Error; err != nil || foreignWallets != 0 {
		t.Fatalf("foreign wallet count=%d err=%v", foreignWallets, err)
	}
	assertRedemptionState(t, db, ctx, -700_000_000, 1)

	// Each call uses its own real PostgreSQL transaction. The code row lock must
	// serialize the first credit and all same-user idempotent replays.
	var group sync.WaitGroup
	results := make(chan transaction.Model, 4)
	errors := make(chan error, 4)
	start := make(chan struct{})
	for i := 0; i < 4; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			row, err := repo.Redeem(ctx, 7, 1, nextCode, now.Add(3*time.Second))
			results <- row
			errors <- err
		}()
	}
	close(start)
	group.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var transactionID int64
	for row := range results {
		assertRedeemLedger(t, row, 2, -700_000_000, -400_000_000)
		if transactionID != 0 && row.ID != transactionID {
			t.Fatalf("concurrent replay returned different ledger IDs: %d, %d", transactionID, row.ID)
		}
		transactionID = row.ID
	}
	assertRedemptionState(t, db, ctx, -400_000_000, 2)
}

func assertRedeemLedger(t *testing.T, row transaction.Model, codeID, before, after int64) {
	t.Helper()
	if row.ID < 1 || row.TransactionNo == "" || row.UserID != 7 || row.PlatformID != 1 || row.Direction != transaction.DirectionCredit || row.AmountUnits != 300_000_000 || row.BalanceBeforeUnits != before || row.BalanceAfterUnits != after || row.RedeemCodeID == nil || *row.RedeemCodeID != codeID || row.OrderID != nil || row.AIUsageChargeID != nil {
		t.Fatalf("unexpected redemption ledger: %+v", row)
	}
	if err := row.Validate(); err != nil {
		t.Fatalf("invalid ledger: %v", err)
	}
}

func assertRedemptionState(t *testing.T, db *gorm.DB, ctx context.Context, balance, count int64) {
	t.Helper()
	var walletRow wallet.Model
	if err := db.WithContext(ctx).First(&walletRow, "user_id = ?", 7).Error; err != nil || walletRow.BalanceUnits != balance {
		t.Fatalf("wallet=%+v want=%d err=%v", walletRow, balance, err)
	}
	var ledgerCount int64
	if err := db.WithContext(ctx).Model(&transaction.Model{}).Count(&ledgerCount).Error; err != nil || ledgerCount != count {
		t.Fatalf("ledger count=%d want=%d err=%v", ledgerCount, count, err)
	}
	var usedCount int64
	if err := db.WithContext(ctx).Model(&Model{}).Where("status = ? AND used_by = ? AND used_at IS NOT NULL", StatusUsed, 7).Count(&usedCount).Error; err != nil || usedCount != count {
		t.Fatalf("used codes=%d want=%d err=%v", usedCount, count, err)
	}
}

func redemptionDB(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	if err := godotenv.Load("../../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	cfg, err := settings.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	db, ctx := testschema.Open(t, cfg.PostgresDSN, "test_payment_redeem")
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(4)
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE user_account(id bigint PRIMARY KEY);
CREATE TABLE permission_auth_platform(id bigint PRIMARY KEY);
CREATE TABLE payment_order(id bigint PRIMARY KEY);
CREATE TABLE ai_usage_charge(id bigint PRIMARY KEY);
CREATE TABLE payment_wallet(user_id bigint PRIMARY KEY REFERENCES user_account(id),balance_units bigint NOT NULL DEFAULT 0,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL);
CREATE TABLE payment_redeem_code_batch(
 id bigint PRIMARY KEY,batch_no varchar(32) NOT NULL UNIQUE,request_id uuid NOT NULL,request_fingerprint bytea NOT NULL CHECK(octet_length(request_fingerprint)=32),
 amount_cents bigint NOT NULL CHECK(amount_cents>0),quantity integer NOT NULL CHECK(quantity BETWEEN 1 AND 1000),expires_at timestamptz,remark varchar(500),
 created_by bigint NOT NULL REFERENCES user_account(id),created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,UNIQUE(created_by,request_id));
CREATE TABLE payment_redeem_code(
 id bigint PRIMARY KEY,batch_id bigint NOT NULL REFERENCES payment_redeem_code_batch(id),code varchar(20) NOT NULL UNIQUE CHECK(length(code)=20),
 status smallint NOT NULL CHECK(status IN (1,2,3)),used_by bigint REFERENCES user_account(id),used_at timestamptz,
 created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,CHECK((status=2 AND used_by IS NOT NULL AND used_at IS NOT NULL) OR (status<>2 AND used_by IS NULL AND used_at IS NULL)));
CREATE TABLE payment_transaction(
 id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,transaction_no varchar(32) NOT NULL UNIQUE,
 user_id bigint NOT NULL REFERENCES payment_wallet(user_id),platform_id bigint NOT NULL REFERENCES permission_auth_platform(id),
 direction smallint NOT NULL CHECK(direction IN (1,2)),amount_units bigint NOT NULL CHECK(amount_units>0),balance_before_units bigint NOT NULL,balance_after_units bigint NOT NULL,
 order_id bigint UNIQUE REFERENCES payment_order(id),redeem_code_id bigint UNIQUE REFERENCES payment_redeem_code(id),ai_usage_charge_id bigint UNIQUE REFERENCES ai_usage_charge(id),
 created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,
 CHECK(num_nonnulls(order_id,redeem_code_id,ai_usage_charge_id)=1),
 CHECK((direction=1 AND ai_usage_charge_id IS NULL) OR (direction=2 AND ai_usage_charge_id IS NOT NULL)),
 CHECK((direction=1 AND balance_after_units::numeric=balance_before_units::numeric+amount_units::numeric) OR (direction=2 AND balance_after_units::numeric=balance_before_units::numeric-amount_units::numeric)));
CREATE FUNCTION payment_transaction_immutable_guard() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'payment transactions are immutable'; END $$;
CREATE TRIGGER payment_transaction_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON payment_transaction FOR EACH STATEMENT EXECUTE FUNCTION payment_transaction_immutable_guard();`).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}
