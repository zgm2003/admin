package notificationtask

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

type batchProcessorStub struct{ err error }

func (s batchProcessorStub) Process(context.Context, BatchPayload) error { return s.err }

type taskFailerStub struct {
	err    error
	called int
}

func (s *taskFailerStub) MarkFailed(context.Context, int64, string, time.Time) error {
	s.called++
	return s.err
}

func validBatchTask() *asynq.Task {
	return asynq.NewTask(TaskType, []byte(`{"schemaVersion":1,"taskId":42,"batchNo":0}`))
}

func TestTaskHandlerMarksTransientFailureWhenRetriesAreExhausted(t *testing.T) {
	failer := &taskFailerStub{}
	handler := &TaskHandler{
		processor: batchProcessorStub{err: errors.New("postgres unavailable")},
		failer:    failer,
		retriesExhausted: func(context.Context) bool {
			return true
		},
	}
	err := handler.ProcessTask(context.Background(), validBatchTask())
	if err == nil || failer.called != 1 {
		t.Fatalf("error=%v markFailed calls=%d", err, failer.called)
	}
}

func TestTaskHandlerRetriesFrozenFailureWhenFailedStateCannotBeSaved(t *testing.T) {
	failer := &taskFailerStub{err: errors.New("mark failed unavailable")}
	handler := &TaskHandler{
		processor: batchProcessorStub{err: ErrFrozenFacts},
		failer:    failer,
	}
	err := handler.ProcessTask(context.Background(), validBatchTask())
	if err == nil || errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("error=%v must remain retryable", err)
	}
}
