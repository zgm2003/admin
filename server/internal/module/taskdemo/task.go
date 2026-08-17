package taskdemo

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

const Type = "foundation:echo"

type Payload struct {
	TaskID string `json:"taskId"`
}

type enqueueClient interface {
	Enqueue(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
}

type QueueEnqueuer struct {
	client enqueueClient
}

func NewQueueEnqueuer(client *queue.Client) *QueueEnqueuer {
	return &QueueEnqueuer{client: client}
}

func (e *QueueEnqueuer) Enqueue(ctx context.Context, taskID string) error {
	payload, err := json.Marshal(Payload{TaskID: taskID})
	if err != nil {
		return fmt.Errorf("encode foundation task: %w", err)
	}
	task := asynq.NewTask(Type, payload)
	_, err = e.client.Enqueue(ctx, task, asynq.TaskID(taskID), asynq.MaxRetry(3), asynq.Timeout(30*time.Second))
	return err
}

type Processor interface {
	Process(context.Context, string) error
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
		return fmt.Errorf("invalid foundation task payload: %v: %w", err, asynq.SkipRetry)
	}
	return h.processor.Process(ctx, payload.TaskID)
}

func Register(mux *asynq.ServeMux, processor Processor) {
	mux.Handle(Type, NewTaskHandler(processor))
}

func decodePayload(data []byte) (Payload, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var payload Payload
	if err := decoder.Decode(&payload); err != nil {
		return Payload{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Payload{}, fmt.Errorf("payload must contain exactly one JSON document")
	}
	payload.TaskID = strings.TrimSpace(payload.TaskID)
	if payload.TaskID == "" {
		return Payload{}, fmt.Errorf("taskId is required")
	}
	return payload, nil
}
