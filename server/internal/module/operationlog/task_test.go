package operationlog

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

func validTaskPayload() TaskPayload {
	return TaskPayload{
		SchemaVersion: 2,
		EventID:       "event-1",
		RequestID:     "request-1",
		Method:        "PUT",
		Route:         "/api/admin/v1/users/:id",
		Module:        "user",
		Action:        "user.update",
		ClientIP:      "127.0.0.1",
		UserAgent:     "test",
		StatusCode:    200,
		IsSuccess:     1,
		LatencyMs:     4,
		RequestData:   JSON(`{"username":"admin"}`),
		ResponseData:  JSON(`{"code":0}`),
		CreatedAt:     time.Now().UTC(),
	}
}

func TestDecodePayloadRejectsUnknownOrMissingFields(t *testing.T) {
	valid, err := json.Marshal(validTaskPayload())
	if err != nil {
		t.Fatal(err)
	}
	cases := []string{
		string(valid) + `{"trailing":true}`,
		strings.Replace(string(valid), `"requestId":"request-1"`, `"unknownId":"request-1"`, 1),
		strings.Replace(string(valid), `"eventId":"event-1"`, `"unknownId":"event-1"`, 1),
		strings.Replace(string(valid), `"schemaVersion":2`, `"schemaVersion":1`, 1),
		strings.Replace(string(valid), `"statusCode":200`, `"statusCode":-1`, 1),
	}
	for _, payload := range cases {
		if _, err := decodePayload([]byte(payload)); err == nil {
			t.Fatalf("payload was accepted: %s", payload)
		}
	}
}

func TestTaskHandlerMarksMalformedPayloadSkipRetry(t *testing.T) {
	handler := NewTaskHandler(&recordingProcessor{})
	err := handler.ProcessTask(context.Background(), asynq.NewTask(TaskType, []byte(`{"schemaVersion":1}`)))
	if err == nil || !strings.Contains(err.Error(), "skip retry") {
		t.Fatalf("malformed payload error = %v", err)
	}
}

func TestEnqueueUsesEventIDAsTaskID(t *testing.T) {
	payload := validTaskPayload()
	queue := &recordingQueueClient{}
	enqueuer := &QueueEnqueuer{client: queue}
	if err := enqueuer.Enqueue(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	task := queue.task
	if task.Type() != TaskType {
		t.Fatalf("task type = %q", task.Type())
	}
	if queue.taskID != payload.EventID {
		t.Fatalf("task ID = %q, want %q", queue.taskID, payload.EventID)
	}
}

type recordingQueueClient struct {
	task   *asynq.Task
	taskID string
}

func (c *recordingQueueClient) Enqueue(_ context.Context, task *asynq.Task, options ...asynq.Option) (*asynq.TaskInfo, error) {
	c.task = task
	for _, option := range options {
		if option.Type() == asynq.TaskIDOpt {
			c.taskID, _ = option.Value().(string)
		}
	}
	return &asynq.TaskInfo{ID: c.taskID}, nil
}

type recordingProcessor struct {
	payload TaskPayload
}

func (p *recordingProcessor) Process(_ context.Context, payload TaskPayload) error {
	p.payload = payload
	return nil
}
