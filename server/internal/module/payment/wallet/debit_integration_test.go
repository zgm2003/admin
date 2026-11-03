package wallet

import (
	"context"
	"errors"
	"math"
	"os"
	"sync"
	"testing"
	"time"

	settings "admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestDebitTxSerializesConcurrentCreditsAndAllowsNegative(t *testing.T) {
	db, ctx := debitDB(t)
	repo := NewRepository(db)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(debit bool) {
			defer wg.Done()
			errs <- db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				if debit {
					_, _, err := repo.DebitTx(ctx, tx, 7, 3, time.Now().UTC())
					return err
				}
				_, _, err := repo.CreditTx(ctx, tx, 7, 1, time.Now().UTC())
				return err
			})
		}(i%2 == 0)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	row, found, err := repo.Find(ctx, 7)
	if err != nil || !found || row.BalanceUnits != -20 {
		t.Fatalf("row=%+v found=%v err=%v", row, found, err)
	}
}

func TestDebitTxUnderflowAndCancelledContextRollBack(t *testing.T) {
	db, ctx := debitDB(t)
	repo := NewRepository(db)
	now := time.Now().UTC()
	if err := db.WithContext(ctx).Create(&Model{UserID: 7, BalanceUnits: math.MinInt64, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { _, _, err := repo.DebitTx(ctx, tx, 7, 1, now); return err })
	if !errors.Is(err, ErrBalanceOverflow) {
		t.Fatalf("underflow=%v", err)
	}
	row, _, err := repo.Find(ctx, 7)
	if err != nil || row.BalanceUnits != math.MinInt64 {
		t.Fatalf("balance=%d err=%v", row.BalanceUnits, err)
	}
	for _, amount := range []int64{0, -1} {
		err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { _, _, err := repo.DebitTx(ctx, tx, 8, amount, now); return err })
		if err == nil {
			t.Fatalf("accepted %d", amount)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { _, _, err := repo.DebitTx(cancelled, tx, 8, 1, now); return err })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled=%v", err)
	}
	_, found, err := repo.Find(ctx, 8)
	if err != nil || found {
		t.Fatalf("invalid debit created wallet found=%v err=%v", found, err)
	}
}

func debitDB(t *testing.T) (*gorm.DB, context.Context) {
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
	db, ctx := testschema.Open(t, cfg.PostgresDSN, "test_payment_debit")
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(4)
	if err := db.WithContext(ctx).Exec(`CREATE TABLE payment_wallet(user_id bigint PRIMARY KEY,balance_units bigint NOT NULL DEFAULT 0,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL)`).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}
