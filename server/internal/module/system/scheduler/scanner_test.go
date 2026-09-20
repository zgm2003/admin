package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

type dueRepositoryStub struct {
	schedule Schedule
	created  Job
	calls    int
}

func (s *dueRepositoryStub) CreateDueJobs(_ context.Context, now time.Time, _ int, builder DueJobBuilder) (int, error) {
	s.calls++
	job, err := builder(s.schedule, *s.schedule.NextRunAt)
	s.created = job
	return 1, err
}

func TestScannerFreezesValidatedPayload(t *testing.T) {
	now := time.Date(2026, 9, 20, 1, 2, 0, 0, time.UTC)
	due := now.Add(-3 * time.Minute)
	repository := &dueRepositoryStub{schedule: Schedule{ID: 7, TaskType: "example", Params: json.RawMessage(`{"value":1}`), NextRunAt: &due}}
	catalog, err := NewTaskCatalog(TaskDefinition{Type: "example", DisplayName: "Example", Queue: "default", Timeout: time.Minute, MaxAttempts: 3, DefaultParams: json.RawMessage(`{}`), ValidateParams: func(raw json.RawMessage) error { return nil }, Execute: func(context.Context, ExecutionContext) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	scanner := NewScanner(repository, catalog)
	count, err := scanner.RunOnce(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || repository.created.SourceKey != "schedule:7:"+fmtInt(due.UnixNano()) || repository.created.Status != JobScheduled {
		t.Fatalf("job=%+v", repository.created)
	}
}

func fmtInt(value int64) string { return fmt.Sprintf("%d", value) }
