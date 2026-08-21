package operationlog_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/operationlog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestEnsureSchemaCreatesOperationLogContract(t *testing.T) {
	connection, ctx := openOperationLogDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &operationlog.OperationLog{}); err != nil {
		t.Fatal(err)
	}
	if err := operationlog.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}
	if err := operationlog.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("second EnsureSchema() error = %v", err)
	}

	expectedColumns := map[string]string{
		"id": "bigint", "request_id": "character varying", "user_id": "bigint",
		"session_id": "bigint", "platform": "character varying", "method": "character varying",
		"route": "character varying", "module": "character varying", "action": "character varying",
		"client_ip": "character varying", "user_agent": "character varying", "status_code": "integer",
		"is_success": "smallint", "latency_ms": "bigint", "request_data": "jsonb",
		"response_data": "jsonb", "created_at": "timestamp with time zone", "updated_at": "timestamp with time zone",
	}
	for column, dataType := range expectedColumns {
		var actualType, actualNullable string
		err := connection.GORM.WithContext(ctx).Raw(
			"SELECT data_type, is_nullable FROM information_schema.columns "+
				"WHERE table_schema = current_schema() AND table_name = 'sys_operation_log' AND column_name = ?", column,
		).Row().Scan(&actualType, &actualNullable)
		if err != nil {
			t.Fatal(err)
		}
		if actualType != dataType {
			t.Errorf("column %s type = %q, want %q", column, actualType, dataType)
		}
		nullable := map[string]bool{"user_id": true, "session_id": true, "platform": true, "request_data": true, "response_data": true}[column]
		wantNullable := "NO"
		if nullable {
			wantNullable = "YES"
		}
		if actualNullable != wantNullable {
			t.Errorf("column %s nullable = %q, want %q", column, actualNullable, wantNullable)
		}
	}

	for _, indexName := range []string{
		"ux_sys_operation_log_request_id",
		"ix_sys_operation_log_created_at",
		"ix_sys_operation_log_user_created",
		"ix_sys_operation_log_action_created",
	} {
		var definition string
		if err := connection.GORM.WithContext(ctx).Raw(
			"SELECT indexdef FROM pg_indexes WHERE schemaname = current_schema() AND indexname = ?", indexName,
		).Row().Scan(&definition); err != nil {
			t.Fatal(err)
		}
		if definition == "" {
			t.Fatalf("index %s does not exist", indexName)
		}
	}

	var constraint string
	if err := connection.GORM.WithContext(ctx).Raw(
		"SELECT pg_get_constraintdef(oid) FROM pg_constraint " +
			"WHERE conname = 'ck_sys_operation_log_is_success' AND connamespace = current_schema()::regnamespace",
	).Row().Scan(&constraint); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(constraint, "is_success") || !strings.Contains(constraint, "0") || !strings.Contains(constraint, "1") {
		t.Fatalf("is_success constraint = %q", constraint)
	}
	if err := connection.GORM.WithContext(ctx).Exec(
		"INSERT INTO sys_operation_log " +
			"(request_id, method, route, module, action, client_ip, user_agent, status_code, is_success, latency_ms) " +
			"VALUES ('schema-invalid-success', 'POST', '/test', 'test', 'test.create', '127.0.0.1', 'test', 200, 2, 0)",
	).Error; err == nil {
		t.Fatal("is_success=2 was accepted")
	}
}

func openOperationLogDatabase(t *testing.T) (*database.Connection, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration test")
	}
	if err := godotenv.Load("../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("load server .env: %v", err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatalf("load worker config: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	root, err := database.Open(ctx, settings.PostgresDSN)
	if err != nil {
		cancel()
		t.Fatalf("open PostgreSQL: %v", err)
	}
	schema := fmt.Sprintf("test_operationlog_%d", time.Now().UnixNano())
	if err := root.GORM.WithContext(ctx).Exec("CREATE SCHEMA " + schema).Error; err != nil {
		_ = root.Close()
		cancel()
		t.Fatalf("create schema: %v", err)
	}
	pgxConfig, err := pgx.ParseConfig(settings.PostgresDSN)
	if err != nil {
		t.Fatal(err)
	}
	pgxConfig.RuntimeParams["search_path"] = schema
	sqlDB := stdlib.OpenDB(*pgxConfig)
	if err := sqlDB.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	connection := &database.Connection{GORM: gormDB, SQL: sqlDB}
	t.Cleanup(func() {
		_ = connection.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = root.GORM.WithContext(cleanupCtx).Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error
		_ = root.Close()
		cancel()
	})
	return connection, ctx
}
