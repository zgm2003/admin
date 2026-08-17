package taskdemo_test

import (
	"context"
	"errors"
	"testing"

	"admin/server/internal/module/taskdemo"
	"github.com/hibiken/asynq"
)

type processor struct {
	taskID string
	ctx    context.Context
	err    error
}

func (p *processor) Process(ctx context.Context, taskID string) error {
	p.ctx = ctx
	p.taskID = taskID
	return p.err
}

func TestTaskHandlerStrictlyDecodesPayloadAndPassesContext(t *testing.T) {
	processor := &processor{}
	handler := taskdemo.NewTaskHandler(processor)
	ctx := context.WithValue(context.Background(), "request", "task-context")

	err := handler.ProcessTask(ctx, asynq.NewTask(taskdemo.Type, []byte(`{"taskId":"task-1"}`)))
	if err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if processor.taskID != "task-1" || processor.ctx != ctx {
		t.Fatalf("processor taskID=%q context=%v", processor.taskID, processor.ctx)
	}
}

func TestTaskHandlerRejectsInvalidPayloadWithoutCallingService(t *testing.T) {
	for _, payload := range []string{
		`{}`,
		`{"taskId":""}`,
		`{"taskId":"task-1","unknown":true}`,
		`{"taskId":"task-1"} {"taskId":"task-2"}`,
	} {
		t.Run(payload, func(t *testing.T) {
			processor := &processor{}
			handler := taskdemo.NewTaskHandler(processor)

			err := handler.ProcessTask(context.Background(), asynq.NewTask(taskdemo.Type, []byte(payload)))
			if err == nil || !errors.Is(err, asynq.SkipRetry) {
				t.Fatalf("error = %v, want SkipRetry", err)
			}
			if processor.taskID != "" {
				t.Fatalf("processor called with %q", processor.taskID)
			}
		})
	}
}
