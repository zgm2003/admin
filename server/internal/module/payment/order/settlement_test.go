package order

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"testing"
	"time"

	settings "admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"admin/server/internal/module/payment"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

type confirmerStub struct{ calls int }

func (s *confirmerStub) Confirm(ctx context.Context, payment ConfirmedPayment, callbackEventID *int64, now time.Time) (SettlementResult, error) {
	s.calls++
	return SettlementResult{OrderID: 1, TransactionID: 2, BalanceUnits: -7_000_000_000}, nil
}

func validConfirmation() ConfirmedPayment {
	seller := "20880001"
	return ConfirmedPayment{
		ConfigID: 1, ConfigVersion: 1, Environment: payment.EnvironmentSandbox, OrderNo: "PO20261015000000000000000001", TradeNo: "TRADE-1",
		AppID: "20260001", SellerID: &seller, ProofSource: ProofSourceCallback,
		AmountCents: 300, Status: "TRADE_SUCCESS", PaidAt: timePtr(time.Date(2026, 10, 15, 1, 0, 0, 0, time.UTC)),
	}
}

func TestServiceConfirmRejectsUntrustedOrIncompleteProofBeforeRepository(t *testing.T) {
	for name, change := range map[string]func(*ConfirmedPayment){
		"unknown source":          func(value *ConfirmedPayment) { value.ProofSource = 0 },
		"unknown status":          func(value *ConfirmedPayment) { value.Status = "TRADE_PENDING" },
		"missing callback seller": func(value *ConfirmedPayment) { value.SellerID = nil },
		"zero amount":             func(value *ConfirmedPayment) { value.AmountCents = 0 },
		"missing coordinates":     func(value *ConfirmedPayment) { value.ConfigVersion = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			repository := &confirmerStub{}
			payment := validConfirmation()
			change(&payment)
			if _, err := NewService(repository).Confirm(context.Background(), payment, nil); err == nil {
				t.Fatal("invalid confirmation accepted")
			}
			if repository.calls != 0 {
				t.Fatalf("repository called %d times for invalid proof", repository.calls)
			}
		})
	}
}

func TestServiceConfirmPassesValidatedEvidenceToRepository(t *testing.T) {
	repository := &confirmerStub{}
	proof := validConfirmation()
	service := NewService(repository)
	service.environment = payment.EnvironmentSandbox
	got, err := service.Confirm(context.Background(), proof, nil)
	if err != nil || repository.calls != 1 || got.OrderID != 1 || got.TransactionID != 2 || got.BalanceUnits != -7_000_000_000 {
		t.Fatalf("result=%+v calls=%d err=%v", got, repository.calls, err)
	}
}

func TestServiceConfirmRejectsMissingOrMismatchedEnvironmentBeforeRepository(t *testing.T) {
	for _, test := range []struct {
		name            string
		deployed, proof payment.Environment
	}{
		{"missing deployment", 0, payment.EnvironmentSandbox},
		{"missing evidence environment", payment.EnvironmentSandbox, 0},
		{"sandbox proof in production", payment.EnvironmentProduction, payment.EnvironmentSandbox},
		{"production proof in sandbox", payment.EnvironmentSandbox, payment.EnvironmentProduction},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &confirmerStub{}
			service := NewService(repo)
			service.environment = test.deployed
			proof := validConfirmation()
			proof.Environment = test.proof
			if _, err := service.Confirm(context.Background(), proof, nil); err == nil {
				t.Fatal("mismatched environment reached settlement")
			}
			if repo.calls != 0 {
				t.Fatal("repository called for wrong payment environment")
			}
		})
	}
}

func timePtr(value time.Time) *time.Time { return &value }

func TestConfirmRejectsPaidOrderWithoutLedgerAndRollsBackCallback(t *testing.T) {
	db, ctx := settlementDB(t)
	now := time.Now().UTC()
	setupSettlementRows(t, db, ctx, StatusPaid, nil)
	eventID := int64(1)

	_, err := NewRepository(db).Confirm(ctx, validConfirmation(), &eventID, now)
	if !errors.Is(err, ErrSettlementInvariant) {
		t.Fatalf("confirm error = %v, want settlement invariant", err)
	}

	var walletCount, transactionCount int64
	if err = db.WithContext(ctx).Table("payment_wallet").Count(&walletCount).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.WithContext(ctx).Table("payment_transaction").Count(&transactionCount).Error; err != nil {
		t.Fatal(err)
	}
	var orderStatus, callbackStatus int16
	if err = db.WithContext(ctx).Table("payment_order").Select("status").Where("id = 1").Scan(&orderStatus).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.WithContext(ctx).Table("payment_callback_event").Select("process_status").Where("id = 1").Scan(&callbackStatus).Error; err != nil {
		t.Fatal(err)
	}
	if walletCount != 0 || transactionCount != 0 || orderStatus != int16(StatusPaid) || callbackStatus != 1 {
		t.Fatalf("partial settlement persisted: wallets=%d transactions=%d order=%d callback=%d", walletCount, transactionCount, orderStatus, callbackStatus)
	}
}

func TestConfirmCreditsSignedWalletBalancesExactly(t *testing.T) {
	for _, test := range []struct {
		name        string
		before      int64
		amountCents int64
		want        int64
	}{
		{name: "negative remains negative", before: -1_000_000_000, amountCents: 300, want: -700_000_000},
		{name: "negative reaches zero", before: -1_000_000_000, amountCents: 1000, want: 0},
		{name: "negative becomes positive", before: -1_000_000_000, amountCents: 1500, want: 500_000_000},
		{name: "minimum int64 accepts positive credit", before: math.MinInt64, amountCents: 1, want: math.MinInt64 + 1_000_000},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, ctx := settlementDB(t)
			setupSettlementRows(t, db, ctx, StatusPending, &test.before)
			payment := validConfirmation()
			payment.AmountCents = test.amountCents
			if err := db.WithContext(ctx).Exec(`UPDATE payment_order SET amount_cents = ? WHERE id = 1`, test.amountCents).Error; err != nil {
				t.Fatal(err)
			}
			result, err := NewRepository(db).Confirm(ctx, payment, nil, time.Now().UTC())
			if err != nil {
				t.Fatal(err)
			}
			if result.BalanceUnits != test.want {
				t.Fatalf("balance = %d, want %d", result.BalanceUnits, test.want)
			}
			var ledgerBalance int64
			if err := db.WithContext(ctx).Table("payment_transaction").Select("balance_after_units").Where("order_id = 1").Scan(&ledgerBalance).Error; err != nil {
				t.Fatal(err)
			}
			if ledgerBalance != test.want {
				t.Fatalf("ledger balance = %d, want %d", ledgerBalance, test.want)
			}
		})
	}
}

func TestConfirmOverflowRollsBackWalletOrderAndCallback(t *testing.T) {
	db, ctx := settlementDB(t)
	setupSettlementRows(t, db, ctx, StatusPending, ptrInt64(math.MaxInt64))
	payment := validConfirmation()
	eventID := int64(1)
	_, err := NewRepository(db).Confirm(ctx, payment, &eventID, time.Now().UTC())
	if !errors.Is(err, ErrBalanceOverflow) {
		t.Fatalf("confirm error = %v, want balance overflow", err)
	}
	var balance int64
	if err = db.WithContext(ctx).Table("payment_wallet").Select("balance_units").Where("user_id = 7").Scan(&balance).Error; err != nil {
		t.Fatal(err)
	}
	var transactionCount int64
	if err = db.WithContext(ctx).Table("payment_transaction").Count(&transactionCount).Error; err != nil {
		t.Fatal(err)
	}
	var orderStatus, callbackStatus int16
	if err = db.WithContext(ctx).Table("payment_order").Select("status").Where("id = 1").Scan(&orderStatus).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.WithContext(ctx).Table("payment_callback_event").Select("process_status").Where("id = 1").Scan(&callbackStatus).Error; err != nil {
		t.Fatal(err)
	}
	if balance != math.MaxInt64 || transactionCount != 0 || orderStatus != int16(StatusPending) || callbackStatus != 1 {
		t.Fatalf("overflow partially persisted: balance=%d transactions=%d order=%d callback=%d", balance, transactionCount, orderStatus, callbackStatus)
	}
}

func TestConfirmReplaysExistingLedgerAndProcessesCallbackOnce(t *testing.T) {
	db, ctx := settlementDB(t)
	setupSettlementRows(t, db, ctx, StatusPending, ptrInt64(-1_000_000_000))
	payment := validConfirmation()
	eventID := int64(1)
	first, err := NewRepository(db).Confirm(ctx, payment, &eventID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewRepository(db).Confirm(ctx, payment, &eventID, time.Now().UTC().Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if second.TransactionID != first.TransactionID || !second.Replayed || second.BalanceUnits != first.BalanceUnits {
		t.Fatalf("replay result = %+v, first = %+v", second, first)
	}
	var transactionCount int64
	if err = db.WithContext(ctx).Table("payment_transaction").Count(&transactionCount).Error; err != nil {
		t.Fatal(err)
	}
	if transactionCount != 1 {
		t.Fatalf("transaction count = %d, want 1", transactionCount)
	}
}

func TestConfirmAcceptsLatePaymentForClosedOrder(t *testing.T) {
	db, ctx := settlementDB(t)
	setupSettlementRows(t, db, ctx, StatusClosed, ptrInt64(-1_000_000_000))
	result, err := NewRepository(db).Confirm(ctx, validConfirmation(), nil, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if result.BalanceUnits != -700_000_000 {
		t.Fatalf("late payment balance = %d, want -700000000", result.BalanceUnits)
	}
	var status int16
	if err := db.WithContext(ctx).Table("payment_order").Select("status").Where("id = 1").Scan(&status).Error; err != nil {
		t.Fatal(err)
	}
	if status != int16(StatusPaid) {
		t.Fatalf("late payment status = %d, want paid", status)
	}
}

func ptrInt64(value int64) *int64 { return &value }

func settlementDB(t *testing.T) (*gorm.DB, context.Context) {
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
	db, ctx := testschema.Open(t, cfg.PostgresDSN, "test_payment_settlement")
	schema := `
CREATE TABLE payment_config(id bigint PRIMARY KEY, app_id varchar(64) NOT NULL, seller_id varchar(64) NOT NULL, environment smallint NOT NULL CHECK(environment IN (1,2)));
CREATE TABLE payment_config_version(config_id bigint NOT NULL, version bigint NOT NULL, PRIMARY KEY(config_id, version));
CREATE TABLE payment_order(
 id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY, order_no varchar(32) NOT NULL UNIQUE, user_id bigint NOT NULL, platform_id bigint NOT NULL,
 package_id bigint NOT NULL, package_name varchar(128) NOT NULL, amount_cents bigint NOT NULL CHECK(amount_cents > 0),
 config_id bigint NOT NULL, config_version bigint NOT NULL, pay_method smallint NOT NULL, request_id uuid NOT NULL,
 request_fingerprint bytea NOT NULL, return_url varchar(2048) NOT NULL, status smallint NOT NULL CHECK(status IN (1,2,3)),
 provider_trade_no varchar(64), expires_at timestamptz NOT NULL, paid_at timestamptz, credited_at timestamptz,
 closed_at timestamptz, next_check_at timestamptz, check_token uuid, check_lease_until timestamptz,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL, UNIQUE(user_id, platform_id, request_id)
);
CREATE UNIQUE INDEX ux_payment_order_provider_trade ON payment_order(config_id, provider_trade_no) WHERE provider_trade_no IS NOT NULL;
CREATE TABLE payment_wallet(user_id bigint PRIMARY KEY, balance_units bigint NOT NULL DEFAULT 0, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL);
CREATE TABLE payment_transaction(
 id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY, transaction_no varchar(32) NOT NULL UNIQUE,
 user_id bigint NOT NULL, platform_id bigint NOT NULL, amount_units bigint NOT NULL CHECK(amount_units > 0),
 balance_after_units bigint NOT NULL, order_id bigint UNIQUE, redeem_code_id bigint,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL
);
CREATE TABLE payment_callback_event(
 id bigint PRIMARY KEY, order_id bigint, process_status smallint NOT NULL CHECK(process_status IN (1,2,3,4)),
 config_id bigint NOT NULL, config_version bigint NOT NULL, payload jsonb NOT NULL,
 error_code smallint, updated_at timestamptz NOT NULL
);`
	if err := db.WithContext(ctx).Exec(schema).Error; err != nil {
		t.Fatalf("create isolated settlement schema: %v", err)
	}
	return db, ctx
}

func setupSettlementRows(t *testing.T, db *gorm.DB, ctx context.Context, status Status, balance *int64) {
	t.Helper()
	now := time.Now().UTC()
	tradeNo := "TRADE-1"
	var providerTradeNo any
	var paidAt, creditedAt any
	if status == StatusPaid {
		providerTradeNo = tradeNo
		paidAt, creditedAt = now, now
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO payment_config(id,app_id,seller_id,environment) VALUES(1,'20260001','20880001',1)`).Error; err != nil {
		t.Fatalf("seed payment config: %v", err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO payment_config_version(config_id,version) VALUES(1,1)`).Error; err != nil {
		t.Fatalf("seed payment credential: %v", err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO payment_order(id,order_no,user_id,platform_id,package_id,package_name,amount_cents,config_id,config_version,pay_method,request_id,request_fingerprint,return_url,status,provider_trade_no,expires_at,paid_at,credited_at,created_at,updated_at)
VALUES(1,'PO20261015000000000000000001',7,1,1,'Credits',300,1,1,1,'00000000-0000-4000-8000-000000000001',decode('01','hex'),'https://app.example.test/return',?,?,?,?,?,?,?)`, status, providerTradeNo, now.Add(time.Hour), paidAt, creditedAt, now, now).Error; err != nil {
		t.Fatalf("seed settlement schema: %v", err)
	}
	proof := validConfirmation()
	payload, err := json.Marshal(map[string]any{"notifyId": "NOTIFY-1", "orderNo": proof.OrderNo, "tradeNo": proof.TradeNo, "tradeStatus": proof.Status, "appId": proof.AppID, "sellerId": proof.SellerID, "totalAmount": "3.00", "paidAt": proof.PaidAt})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO payment_callback_event(id,process_status,config_id,config_version,payload,updated_at) VALUES(1,1,1,1,?::jsonb,?)`, string(payload), now).Error; err != nil {
		t.Fatalf("seed callback: %v", err)
	}
	if balance != nil {
		if err := db.WithContext(ctx).Exec(`INSERT INTO payment_wallet(user_id,balance_units,created_at,updated_at) VALUES(7,?,?,?)`, *balance, now, now).Error; err != nil {
			t.Fatalf("seed wallet: %v", err)
		}
	}
}
