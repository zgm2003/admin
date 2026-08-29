package operationlog_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/audit/operationlog"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestOperationLogWorkerPersistsIdempotently(t *testing.T) {
	connection, ctx := openOperationLogDatabase(t)
	for _, statement := range []string{
		`CREATE TABLE auth_platform (id BIGINT PRIMARY KEY, code VARCHAR(49) NOT NULL)`,
		`INSERT INTO auth_platform (id, code) VALUES (17, 'admin')`,
		`CREATE TABLE audit_operation_log (
			id BIGSERIAL PRIMARY KEY,
			event_id VARCHAR(64) NOT NULL UNIQUE,
			request_id VARCHAR(128) NOT NULL,
			user_id BIGINT,
			session_id BIGINT,
			platform_id BIGINT REFERENCES auth_platform(id) ON DELETE RESTRICT,
			method VARCHAR(10) NOT NULL,
			route VARCHAR(255) NOT NULL,
			module VARCHAR(64) NOT NULL,
			action VARCHAR(128) NOT NULL,
			client_ip VARCHAR(64) NOT NULL,
			user_agent VARCHAR(512) NOT NULL,
			status_code INTEGER NOT NULL,
			is_success SMALLINT NOT NULL CHECK (is_success IN (0, 1)),
			latency_ms BIGINT NOT NULL CHECK (latency_ms >= 0),
			request_data JSONB,
			response_data JSONB,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
	} {
		if err := connection.GORM.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}

	platformID := int64(17)
	payload := operationlog.TaskPayload{
		SchemaVersion: 2, EventID: "operation-event-1", RequestID: "operation-integration-1", PlatformID: &platformID,
		Method: "PUT", Route: "/api/admin/v1/users/:id", Module: "user", Action: "user.update",
		ClientIP: "127.0.0.1", UserAgent: "integration", StatusCode: 200, IsSuccess: 1, LatencyMs: 4,
		RequestData: operationlog.JSON(`{"password":"***"}`), ResponseData: operationlog.JSON(`{"code":0}`),
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	mux := asynq.NewServeMux()
	operationlog.Register(mux, operationlog.NewService(operationlog.NewRepository(connection.GORM)))
	task := asynq.NewTask(operationlog.TaskType, encoded)
	for index := 0; index < 2; index++ {
		if err := mux.ProcessTask(context.Background(), task); err != nil {
			t.Fatalf("ProcessTask(%d): %v", index, err)
		}
	}

	var count int64
	if err := connection.GORM.WithContext(ctx).Model(&operationlog.OperationLog{}).
		Where("event_id = ?", payload.EventID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("operation log count = %d, want 1", count)
	}
	var stored operationlog.OperationLog
	if err := connection.GORM.WithContext(ctx).Where("event_id = ?", payload.EventID).Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	var storedRequest, expectedRequest map[string]string
	if err := json.Unmarshal(stored.RequestData, &storedRequest); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(payload.RequestData, &expectedRequest); err != nil {
		t.Fatal(err)
	}
	if stored.Action != payload.Action || stored.PlatformID == nil || *stored.PlatformID != platformID || storedRequest["password"] != expectedRequest["password"] {
		t.Fatalf("stored operation log = %+v", stored)
	}
	items, total, err := operationlog.NewRepository(connection.GORM).List(ctx, operationlog.ListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].Platform != "admin" {
		t.Fatalf("listed operation logs = %+v, total = %d", items, total)
	}
}

func openOperationLogDatabase(t *testing.T) (*database.Connection, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration test")
	}
	if err := godotenv.Load("../../../../.env"); err != nil && !os.IsNotExist(err) {
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
