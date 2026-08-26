package testschema

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"admin/server/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var prefixPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Open creates a PostgreSQL schema isolated from other integration tests.
func Open(t *testing.T, dsn, prefix string) (*gorm.DB, context.Context) {
	t.Helper()
	if !prefixPattern.MatchString(prefix) {
		t.Fatalf("invalid PostgreSQL test schema prefix %q", prefix)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	root, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	schema := fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	if err := root.GORM.WithContext(ctx).Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
		_ = root.Close()
		t.Fatalf("create PostgreSQL test schema: %v", err)
	}

	var sqlDBCloser interface{ Close() error }
	t.Cleanup(func() {
		if sqlDBCloser != nil {
			_ = sqlDBCloser.Close()
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = root.GORM.WithContext(cleanupCtx).Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`).Error
		_ = root.Close()
	})

	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse PostgreSQL DSN: %v", err)
	}
	config.RuntimeParams["search_path"] = schema
	sqlDB := stdlib.OpenDB(*config)
	sqlDBCloser = sqlDB
	if err := sqlDB.PingContext(ctx); err != nil {
		t.Fatalf("ping isolated PostgreSQL schema: %v", err)
	}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open isolated GORM: %v", err)
	}
	return db, ctx
}
