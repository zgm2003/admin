package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/operationlog"
	"github.com/hibiken/asynq"
)

func TestCheckWorkerRedisReturnsFullStartupContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := checkWorkerRedis(ctx, "redis://127.0.0.1:1/0")
	if err == nil {
		t.Fatal("expected Redis startup check to fail")
	}
	for _, want := range []string{"check Worker Redis", "ping Redis"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want %q context", err, want)
		}
	}
}

type workerOperationLogProcessor struct {
	processed string
}

func (p *workerOperationLogProcessor) Process(_ context.Context, payload operationlog.TaskPayload) error {
	p.processed = payload.RequestID
	return nil
}

func TestBuildWorkerMuxRegistersOnlyOperationLogTasks(t *testing.T) {
	operationProcessor := &workerOperationLogProcessor{}
	mux := buildWorkerMux(operationProcessor)

	operationPayload, err := json.Marshal(operationlog.TaskPayload{
		SchemaVersion: 2, EventID: "worker-operation-event", RequestID: "request-1", Method: "PUT", Route: "/api/admin/v1/users/:id",
		Module: "user", Action: "user.update", ClientIP: "127.0.0.1", UserAgent: "test",
		StatusCode: 200, IsSuccess: 1, LatencyMs: 1, CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mux.ProcessTask(context.Background(), asynq.NewTask(operationlog.TaskType, operationPayload)); err != nil {
		t.Fatalf("process operation log task: %v", err)
	}
	if operationProcessor.processed != "request-1" {
		t.Fatalf("processed operation=%q", operationProcessor.processed)
	}
}
