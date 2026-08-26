package operationlog_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"admin/server/internal/database"
	"admin/server/internal/module/operationlog"
	"github.com/hibiken/asynq"
)

func TestOperationLogWorkerPersistsIdempotently(t *testing.T) {
	connection, ctx := openOperationLogDatabase(t)
	if err := database.AutoMigrate(ctx, connection.GORM, &operationlog.OperationLog{}); err != nil {
		t.Fatal(err)
	}
	if err := operationlog.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatal(err)
	}

	userID := int64(7)
	sessionID := int64(9)
	platform := "admin"
	payload := operationlog.TaskPayload{
		SchemaVersion: 2, EventID: "operation-event-1", RequestID: "operation-integration-1", UserID: &userID, SessionID: &sessionID,
		Platform: &platform, Method: "PUT", Route: "/api/v1/users/:id", Module: "user", Action: "user.update",
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
	if stored.Action != payload.Action || storedRequest["password"] != expectedRequest["password"] {
		t.Fatalf("stored operation log = %+v", stored)
	}
}
