package architecture_test

import (
	"os"
	"strings"
	"testing"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestMenuCatalogVersionMigrationPreservesPolicyAndRollsBack(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	if err := godotenv.Load("../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	cfg, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	db, ctx := testschema.Open(t, cfg.PostgresDSN, "test_menu_version_migration")
	if err := db.WithContext(ctx).Exec(`CREATE TABLE permission_auth_platform (id bigint PRIMARY KEY,policy_version bigint NOT NULL); INSERT INTO permission_auth_platform VALUES(1,7),(2,8)`).Error; err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../../docs/database/2026-09-08-menu-catalog-version.sql")
	if err != nil {
		t.Fatal(err)
	}
	script := string(raw)
	failing := strings.Replace(script, "COMMIT;", "SELECT 1/0; COMMIT;", 1)
	if err := db.WithContext(ctx).Connection(func(conn *gorm.DB) error {
		_, failure := conn.Statement.ConnPool.ExecContext(ctx, failing)
		if _, err := conn.Statement.ConnPool.ExecContext(ctx, "ROLLBACK"); err != nil {
			return err
		}
		if failure == nil {
			t.Error("failure injection did not fail")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='permission_auth_platform' AND column_name='menu_version'`).Scan(&count).Error; err != nil || count != 0 {
		t.Fatalf("partial DDL remained: count=%d err=%v", count, err)
	}
	for run := 0; run < 2; run++ {
		if err := db.WithContext(ctx).Exec(script).Error; err != nil {
			t.Fatal(err)
		}
		if run == 0 {
			if err := db.WithContext(ctx).Exec(`UPDATE permission_auth_platform SET menu_version=9 WHERE id=1`).Error; err != nil {
				t.Fatal(err)
			}
		}
		if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_auth_platform WHERE (id=1 AND policy_version=7 AND menu_version=9) OR (id=2 AND policy_version=8 AND menu_version=1)`).Scan(&count).Error; err != nil || count != 2 {
			t.Fatalf("migration changed existing facts: count=%d err=%v", count, err)
		}
	}
	if err := db.WithContext(ctx).Exec(`UPDATE permission_auth_platform SET menu_version=0 WHERE id=1`).Error; err == nil {
		t.Fatal("non-positive menu version accepted")
	}
}
