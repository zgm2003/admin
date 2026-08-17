package database_test

import (
	"context"
	"strings"
	"testing"

	"admin/server/internal/database"
)

func TestOpenHonorsCanceledContext(t *testing.T) {
	context, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := database.Open(context, "host=127.0.0.1 user=postgres password=postgres dbname=admin port=1 sslmode=disable connect_timeout=1")
	if err == nil {
		t.Fatal("expected canceled PostgreSQL connection to fail")
	}
	if !strings.Contains(err.Error(), "ping PostgreSQL") {
		t.Fatalf("error = %q", err)
	}
}

func TestAutoMigrateRejectsEmptyModels(t *testing.T) {
	if err := database.AutoMigrate(context.Background(), nil); err == nil {
		t.Fatal("expected empty AutoMigrate call to fail")
	}
}
