package database_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	redeemcode "admin/server/internal/module/payment/redeemCode"
	"gorm.io/gorm"
)

func plaintextMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-10-22-payment-redeem-code-plaintext.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestPaymentRedeemPlaintextGenerationAndConcurrentCredit(t *testing.T) {
	db, ctx := openPaymentRedeemSchema(t, "test_redeem_plain_concurrent")
	applyPaymentRedeemMigration(t, db, ctx)
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(plaintextMigration(t)).Error }); err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO payment_wallet(user_id,balance_units) VALUES(7,-1000000000); INSERT INTO user_account(id) VALUES(8)`).Error; err != nil {
		t.Fatal(err)
	}
	repo := redeemcode.NewRepository(db)
	service := redeemcode.NewService(repo, nil)
	input := redeemcode.GenerateInput{RequestID: "00000000-0000-4000-8000-000000000001", AmountCents: 100, Quantity: 2}
	first, err := service.Generate(ctx, 7, input)
	if err != nil || len(first.Codes) != 2 || first.Codes[0] == first.Codes[1] {
		t.Fatalf("generation failed: %v", err)
	}
	replay, err := service.Generate(ctx, 7, input)
	if err != nil || replay.BatchID != first.BatchID || len(replay.Codes) != 2 || replay.Codes[0] != first.Codes[0] || replay.Codes[1] != first.Codes[1] {
		t.Fatalf("generation not idempotent: %v", err)
	}
	input.AmountCents++
	if _, err := service.Generate(ctx, 7, input); err == nil {
		t.Fatal("conflicting request accepted")
	}
	row, err := repo.Lookup(ctx, first.Codes[0])
	if err != nil || row.Code != first.Codes[0] {
		t.Fatalf("plaintext lookup failed: %v", err)
	}
	type outcome struct {
		id  int64
		err error
	}
	results := make(chan outcome, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx, err := repo.Redeem(ctx, 7, 1, first.Codes[0], time.Now())
			results <- outcome{tx.ID, err}
		}()
	}
	wg.Wait()
	close(results)
	var transactionID int64
	for result := range results {
		if result.err != nil || result.id < 1 {
			t.Fatalf("concurrent redemption failed: %v", result.err)
		}
		if transactionID != 0 && transactionID != result.id {
			t.Fatal("more than one credit")
		}
		transactionID = result.id
	}
	if _, err := repo.Redeem(ctx, 8, 1, first.Codes[0], time.Now()); err != redeemcode.ErrCodeUnavailable {
		t.Fatalf("other user reused code: %v", err)
	}
	if err := repo.Void(ctx, []int64{row.ID}, time.Now()); err != redeemcode.ErrCodeUnavailable {
		t.Fatalf("used code voided: %v", err)
	}
	var balance, count int64
	if err := db.WithContext(ctx).Raw(`SELECT balance_units FROM payment_wallet WHERE user_id=7`).Scan(&balance).Error; err != nil || balance != -900000000 {
		t.Fatalf("wrong balance: %d %v", balance, err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM payment_transaction`).Scan(&count).Error; err != nil || count != 1 {
		t.Fatalf("duplicate transaction: %d %v", count, err)
	}
}

func TestPaymentRedeemPlaintextMigrationPreservesFactsAndRedemption(t *testing.T) {
	db, ctx := openPaymentRedeemSchema(t, "test_redeem_plain")
	applyPaymentRedeemMigration(t, db, ctx)
	if err := db.WithContext(ctx).Exec(`
INSERT INTO payment_redeem_code_batch(batch_no,request_id,request_fingerprint,amount_cents,quantity,created_by)
VALUES('RB-plain','00000000-0000-4000-8000-000000000001',decode(repeat('01',32),'hex'),100,1,7);
INSERT INTO payment_redeem_code(batch_id,code_hash,code_ciphertext,status) VALUES(1,decode(repeat('02',32),'hex'),'legacy',1);
`).Error; err != nil {
		t.Fatal(err)
	}
	sql := plaintextMigration(t)
	// Missing conversion must fail atomically, not discard a historical code.
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(sql).Error }); err == nil {
		t.Fatal("legacy data discarded without a conversion map")
	}
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TEMP TABLE payment_redeem_code_plaintext(id bigint PRIMARY KEY,code text NOT NULL) ON COMMIT DROP;
INSERT INTO payment_redeem_code_plaintext VALUES(1,'ABCDEFGHJKLMNPQRSTUV');`).Error; err != nil {
			return err
		}
		return tx.Exec(sql).Error
	}); err != nil {
		t.Fatal(err)
	}
	before := paymentRedeemSchemaDigest(t, db, ctx)
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(sql).Error }); err != nil {
		t.Fatal(err)
	}
	if after := paymentRedeemSchemaDigest(t, db, ctx); after != before {
		t.Fatal("replay changed schema or facts")
	}
	var obsolete int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='payment_redeem_code' AND column_name IN ('code_hash','code_ciphertext')`).Scan(&obsolete).Error; err != nil || obsolete != 0 {
		t.Fatalf("obsolete columns remain: %d %v", obsolete, err)
	}
	for _, stmt := range []string{
		`UPDATE payment_redeem_code SET code='BCDEFGHJKLMNPQRSTUVW' WHERE id=1`,
		`DELETE FROM payment_redeem_code WHERE id=1`,
		`TRUNCATE payment_redeem_code CASCADE`,
		`INSERT INTO payment_redeem_code(batch_id,code) VALUES(1,'ABCDEFGHJKLMNPQRSTUV')`,
		`INSERT INTO payment_redeem_code(batch_id,code) VALUES(1,'short')`,
		`INSERT INTO payment_redeem_code(batch_id,code) VALUES(1,'abcdefghijklmnopqrst')`,
	} {
		if err := db.WithContext(ctx).Exec(stmt).Error; err == nil {
			t.Fatal("invalid fact accepted")
		}
	}
	repo := redeemcode.NewRepository(db)
	listed, total, err := repo.List(ctx, redeemcode.ListQuery{Page: 1, PageSize: 20}, time.Now())
	if err != nil || total != 1 || len(listed) != 1 || listed[0].Code != "ABCDEFGHJKLMNPQRSTUV" {
		t.Fatalf("list failed: %v", err)
	}
	first, err := repo.Redeem(ctx, 7, 1, listed[0].Code, time.Now())
	if err != nil || first.AmountUnits != 100000000 {
		t.Fatalf("redemption failed: %v", err)
	}
	replayed, err := repo.Redeem(ctx, 7, 1, listed[0].Code, time.Now())
	if err != nil || replayed.ID != first.ID {
		t.Fatalf("redemption not idempotent: %v", err)
	}
	var balance int64
	if err := db.WithContext(ctx).Raw(`SELECT balance_units FROM payment_wallet WHERE user_id=7`).Scan(&balance).Error; err != nil || balance != 100000000 {
		t.Fatalf("balance changed twice: %d %v", balance, err)
	}
}
