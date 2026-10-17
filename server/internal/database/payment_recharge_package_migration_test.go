package database_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"admin/server/internal/database/testschema"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPaymentRechargePackageCodeMigrationIsIdempotentAndPreservesRows(t *testing.T) {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		if err := godotenv.Load(filepath.Join(repoRoot(t), "server", ".env")); err == nil {
			dsn = os.Getenv("POSTGRES_DSN")
		}
	}
	if dsn == "" {
		t.Skip("POSTGRES_DSN is not configured")
	}
	db, ctx := testschema.Open(t, dsn, "test_payment_recharge_package_code")
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	if err := db.WithContext(ctx).Exec(`CREATE TABLE payment_recharge_package(id bigint PRIMARY KEY, code varchar(64) NOT NULL UNIQUE, name varchar(128) NOT NULL)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO payment_recharge_package(id,code,name) VALUES(1,'starter','Starter')`).Error; err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-10-17-payment-recharge-package-code.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ReplaceAll(string(data), "\r\n", "\n")
	for range 2 {
		if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(sql).Error }); err != nil {
			t.Fatal(err)
		}
	}
	var hasCode bool
	if err := db.WithContext(ctx).Raw(`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='payment_recharge_package' AND column_name='code')`).Scan(&hasCode).Error; err != nil {
		t.Fatal(err)
	}
	if hasCode {
		t.Fatal("recharge package code column remains")
	}
	var name string
	if err := db.WithContext(ctx).Raw(`SELECT name FROM payment_recharge_package WHERE id=1`).Scan(&name).Error; err != nil || name != "Starter" {
		t.Fatalf("preserved row=%q err=%v", name, err)
	}
}
