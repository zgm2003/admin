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
		SchemaVersion: 1,
		RequestID:     "request-1",
		Method:        "PUT",
		Route:         "/api/v1/users/:id",
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
		strings.Replace(string(valid), `"schemaVersion":1`, `"schemaVersion":2`, 1),
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
	err := handler.ProcessTask(context.Background(), asynq.NewTask(Type, []byte(`{"schemaVersion":1}`)))
	if err == nil || !strings.Contains(err.Error(), "skip retry") {
		t.Fatalf("malformed payload error = %v", err)
	}
}

func TestEnqueueUsesStableOperationLogTaskType(t *testing.T) {
	payload := validTaskPayload()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	task := asynq.NewTask(Type, encoded, asynq.TaskID(payload.RequestID), asynq.MaxRetry(3), asynq.Timeout(30*time.Second))
	if task.Type() != "system:operation-log:v1" {
		t.Fatalf("task type = %q", task.Type())
	}
	if string(task.Payload()) != string(encoded) {
		t.Fatalf("task payload changed")
	}
}

type recordingProcessor struct {
	payload TaskPayload
}

func (p *recordingProcessor) Process(_ context.Context, payload TaskPayload) error {
	p.payload = payload
	return nil
}
