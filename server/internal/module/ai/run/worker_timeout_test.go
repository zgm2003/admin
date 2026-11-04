package run

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type workerTimeoutLoader struct{ row Model }

func (l *workerTimeoutLoader) Find(context.Context, int64) (Model, error) { return l.row, nil }

func TestWorkerTimeoutUsesFrozenRunAndTransferDeadlines(t *testing.T) {
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	loader := &workerTimeoutLoader{row: Model{ID: 9, ExecutionStatus: 0, Operation: "image.generate", InputSnapshot: json.RawMessage(`{"count":3}`), DeadlineAt: now.Add(2 * time.Minute), PolicySnapshot: json.RawMessage(`{"transferTimeoutSeconds":900}`)}}
	worker := &Worker{Loader: loader}
	got, err := worker.TimeoutForPayload(context.Background(), json.RawMessage(`{"runId":9}`), now)
	if err != nil || got != 47*time.Minute+30*time.Second {
		t.Fatalf("timeout=%s err=%v want=47m30s for three image imports", got, err)
	}
	loader.row.PolicySnapshot = json.RawMessage(`{"transferTimeoutSeconds":1800}`)
	got, err = worker.TimeoutForPayload(context.Background(), json.RawMessage(`{"runId":9}`), now)
	if err != nil || got != 92*time.Minute+30*time.Second {
		t.Fatalf("updated frozen policy timeout=%s err=%v want=92m30s", got, err)
	}
	loader.row.DeadlineAt = now.Add(-time.Second)
	got, err = worker.TimeoutForPayload(context.Background(), json.RawMessage(`{"runId":9}`), now)
	if err != nil || got != 30*time.Second {
		t.Fatalf("expired run timeout=%s err=%v want=30s for terminalization", got, err)
	}
}

func TestWorkerTimeoutAllowsInvalidPayloadToReachFailureHandler(t *testing.T) {
	worker := &Worker{Loader: &workerTimeoutLoader{row: Model{ID: 1}}}
	got, err := worker.TimeoutForPayload(context.Background(), json.RawMessage(`{"runId":0}`), time.Now())
	if err != nil || got != 30*time.Second {
		t.Fatalf("timeout=%s err=%v want 30s", got, err)
	}
	if _, err := parseRunID(json.RawMessage(`{"runId":0}`)); err == nil || errors.Is(err, context.Canceled) {
		t.Fatalf("invalid payload error=%v", err)
	}
}
