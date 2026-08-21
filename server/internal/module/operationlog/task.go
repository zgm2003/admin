package operationlog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"admin/server/internal/queue"
	"github.com/hibiken/asynq"
)

const Type = "system:operation-log:v2"

type Processor interface {
	Process(context.Context, TaskPayload) error
}

type QueueEnqueuer struct {
	client interface {
		Enqueue(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
	}
}

func NewQueueEnqueuer(client *queue.Client) *QueueEnqueuer {
	return &QueueEnqueuer{client: client}
}

func (e *QueueEnqueuer) Enqueue(ctx context.Context, payload TaskPayload) error {
	if err := validateTaskPayload(payload); err != nil {
		return fmt.Errorf("enqueue operation log: %w", err)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode operation log task: %w", err)
	}
	_, err = e.client.Enqueue(ctx, asynq.NewTask(Type, encoded), asynq.TaskID(payload.EventID), asynq.MaxRetry(3), asynq.Timeout(30*time.Second))
	if err != nil {
		return fmt.Errorf("enqueue operation log task: %w", err)
	}
	return nil
}

type TaskHandler struct {
	processor Processor
}

func NewTaskHandler(processor Processor) *TaskHandler {
	return &TaskHandler{processor: processor}
}

func (h *TaskHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := decodePayload(task.Payload())
	if err != nil {
		return fmt.Errorf("invalid operation log task payload: %v: %w", err, asynq.SkipRetry)
	}
	if err := h.processor.Process(ctx, payload); err != nil {
		return err
	}
	return nil
}

func Register(mux *asynq.ServeMux, processor Processor) {
	mux.Handle(Type, NewTaskHandler(processor))
}

func decodePayload(data []byte) (TaskPayload, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var payload TaskPayload
	if err := decoder.Decode(&payload); err != nil {
		return TaskPayload{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return TaskPayload{}, fmt.Errorf("payload must contain exactly one JSON document")
	}
	if err := validateTaskPayload(payload); err != nil {
		return TaskPayload{}, err
	}
	return payload, nil
}

func validateTaskPayload(payload TaskPayload) error {
	if payload.SchemaVersion != 2 {
		return fmt.Errorf("schemaVersion is invalid")
	}
	if strings.TrimSpace(payload.EventID) == "" || len(payload.EventID) > 64 {
		return fmt.Errorf("eventId is invalid")
	}
	if strings.TrimSpace(payload.RequestID) == "" || len(payload.RequestID) > 128 {
		return fmt.Errorf("requestId is invalid")
	}
	if payload.Method == "" || payload.Route == "" || payload.Module == "" || payload.Action == "" {
		return fmt.Errorf("operation route fields are required")
	}
	if payload.StatusCode < 100 || payload.StatusCode > 599 {
		return fmt.Errorf("statusCode is invalid")
	}
	if payload.IsSuccess != 0 && payload.IsSuccess != 1 {
		return fmt.Errorf("isSuccess is invalid")
	}
	if payload.LatencyMs < 0 {
		return fmt.Errorf("latencyMs is invalid")
	}
	if payload.CreatedAt.IsZero() {
		return fmt.Errorf("createdAt is required")
	}
	if len(payload.RequestData) > maxSummaryBytes || len(payload.ResponseData) > maxSummaryBytes {
		return fmt.Errorf("operation log summary is too large")
	}
	return nil
}
