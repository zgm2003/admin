package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type executionRepositoryStub struct {
	job       Job
	run       Run
	claimed   int
	completed int
	failed    int
	retryAt   *time.Time
}

func (s *executionRepositoryStub) GetJob(context.Context, int64) (Job, error) { return s.job, nil }
func (s *executionRepositoryStub) ClaimRun(_ context.Context, _ int64, attempt int, dispatchToken, workerID, token string, now time.Time, _ time.Duration) (Run, Job, error) {
	s.claimed++
	s.job.Status = JobRunning
	s.job.AttemptCount = attempt
	s.job.RunToken = token
	s.run = Run{JobID: s.job.ID, AttemptNo: attempt, StartedAt: now}
	return s.run, s.job, nil
}
func (s *executionRepositoryStub) CompleteRun(context.Context, int64, int, string, time.Time) error {
	s.completed++
	return nil
}
func (s *executionRepositoryStub) RenewRun(context.Context, int64, int, string, time.Time) error {
	return nil
}
func (s *executionRepositoryStub) FailRun(_ context.Context, _ int64, _ int, _ string, _, _ string, retryAt *time.Time, _ time.Time) error {
	s.failed++
	s.retryAt = retryAt
	return nil
}

func TestExecutorCompletesSuccessfulAttempt(t *testing.T) {
	now := time.Date(2026, 9, 20, 3, 0, 0, 0, time.UTC)
	repository := &executionRepositoryStub{job: Job{ID: 1, TaskType: "example", Payload: json.RawMessage(`{}`), Status: JobQueued, MaxAttempts: 3, PublishToken: "dispatch"}}
	catalog, _ := NewTaskCatalog(testDefinition("example"))
	executor := NewExecutor(repository, catalog, "worker-1")
	executor.now = func() time.Time { return now }
	if err := executor.Execute(context.Background(), JobEnvelope{SchemaVersion: 1, JobID: 1, Attempt: 1, DispatchToken: "dispatch"}); err != nil {
		t.Fatal(err)
	}
	if repository.claimed != 1 || repository.completed != 1 || repository.failed != 0 {
		t.Fatalf("repository=%+v", repository)
	}
}

func TestExecutorUsesPayloadSpecificTimeout(t *testing.T) {
	now := time.Now().UTC()
	observed := time.Duration(0)
	definition := testDefinition("ai.run.execute")
	definition.TimeoutForPayload = func(_ context.Context, _ json.RawMessage, at time.Time) (time.Duration, error) {
		if !at.Equal(now) {
			t.Fatalf("resolver time=%s want=%s", at, now)
		}
		return 400 * time.Millisecond, nil
	}
	definition.Execute = func(ctx context.Context, _ ExecutionContext) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			return errors.New("execution has no timeout")
		}
		observed = time.Until(deadline)
		return nil
	}
	repository := &executionRepositoryStub{job: Job{ID: 1, TaskType: "ai.run.execute", Payload: json.RawMessage(`{"runId":1}`), Status: JobQueued, MaxAttempts: 1, PublishToken: "dispatch"}}
	catalog, _ := NewTaskCatalog(definition)
	executor := NewExecutor(repository, catalog, "worker-1")
	executor.now = func() time.Time { return now }
	if err := executor.Execute(context.Background(), JobEnvelope{SchemaVersion: 1, JobID: 1, Attempt: 1, DispatchToken: "dispatch"}); err != nil {
		t.Fatal(err)
	}
	if observed <= 0 || observed > 400*time.Millisecond {
		t.Fatalf("execution timeout remaining=%s", observed)
	}
}
func TestExecutorPersistsRetryableFailure(t *testing.T) {
	now := time.Now().UTC()
	definition := testDefinition("example")
	definition.Execute = func(context.Context, ExecutionContext) error { return errors.New("secret database detail") }
	repository := &executionRepositoryStub{job: Job{ID: 1, TaskType: "example", Payload: json.RawMessage(`{}`), Status: JobQueued, MaxAttempts: 3, PublishToken: "dispatch"}}
	catalog, _ := NewTaskCatalog(definition)
	executor := NewExecutor(repository, catalog, "worker-1")
	executor.now = func() time.Time { return now }
	if err := executor.Execute(context.Background(), JobEnvelope{SchemaVersion: 1, JobID: 1, Attempt: 1, DispatchToken: "dispatch"}); err != nil {
		t.Fatal(err)
	}
	if repository.failed != 1 || repository.retryAt == nil {
		t.Fatalf("repository=%+v", repository)
	}
}
func TestDecodeJobEnvelopeRejectsUnknownFields(t *testing.T) {
	if _, err := DecodeJobEnvelope([]byte(`{"schemaVersion":1,"jobId":1,"attempt":1,"dispatchToken":"a","extra":true}`)); err == nil {
		t.Fatal("expected strict decode failure")
	}
}

func TestExecutorDoesNotCompleteAfterParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	definition := testDefinition("example")
	definition.Execute = func(context.Context, ExecutionContext) error { cancel(); return nil }
	repository := &executionRepositoryStub{job: Job{ID: 1, TaskType: "example", Payload: json.RawMessage(`{}`), Status: JobQueued, MaxAttempts: 3, PublishToken: "dispatch"}}
	catalog, _ := NewTaskCatalog(definition)
	err := NewExecutor(repository, catalog, "worker-1").Execute(ctx, JobEnvelope{SchemaVersion: 1, JobID: 1, Attempt: 1, DispatchToken: "dispatch"})
	if !errors.Is(err, context.Canceled) || repository.completed != 0 || repository.failed != 0 {
		t.Fatalf("err=%v completed=%d failed=%d", err, repository.completed, repository.failed)
	}
}
