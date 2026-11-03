package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type heartbeatRepository struct {
	executionRepositoryStub
	renew func(context.Context, int64, int, string, time.Time) error
}

func (r *heartbeatRepository) RenewRun(ctx context.Context, id int64, attempt int, token string, now time.Time) error {
	return r.renew(ctx, id, attempt, token, now)
}

func TestExecutorHeartbeatLossCancelsTaskAndPreventsFinalization(t *testing.T) {
	for _, taskType := range []string{"example", "ai.run.execute"} {
		t.Run(taskType, func(t *testing.T) {
			definition := testDefinition(taskType)
			definition.Execute = func(ctx context.Context, _ ExecutionContext) error { <-ctx.Done(); return nil }
			repository := &heartbeatRepository{executionRepositoryStub: executionRepositoryStub{job: Job{ID: 1, TaskType: taskType, Payload: json.RawMessage(`{}`), Status: JobQueued, MaxAttempts: 1, PublishToken: "dispatch"}}}
			repository.renew = func(_ context.Context, id int64, attempt int, token string, _ time.Time) error {
				if id != 1 || attempt != 1 || token == "" || token != repository.job.RunToken {
					t.Errorf("invalid heartbeat identity")
				}
				return ErrClaimLost
			}
			catalog, _ := NewTaskCatalog(definition)
			executor := NewExecutor(repository, catalog, "worker-1")
			executor.heartbeatInterval = time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err := executor.Execute(ctx, JobEnvelope{SchemaVersion: 1, JobID: 1, Attempt: 1, DispatchToken: "dispatch"})
			if !errors.Is(err, ErrClaimLost) || repository.completed != 0 || repository.failed != 0 {
				t.Fatalf("err=%v completed=%d failed=%d", err, repository.completed, repository.failed)
			}
		})
	}
}

func TestExecutorHeartbeatIOIsBoundedBeforeTheExecutionDeadline(t *testing.T) {
	definition := testDefinition("example")
	definition.Execute = func(ctx context.Context, _ ExecutionContext) error { <-ctx.Done(); return nil }
	repository := &heartbeatRepository{executionRepositoryStub: executionRepositoryStub{job: Job{ID: 1, TaskType: "example", Payload: json.RawMessage(`{}`), Status: JobQueued, MaxAttempts: 1, PublishToken: "dispatch"}}}
	repository.renew = func(ctx context.Context, _ int64, _ int, _ string, _ time.Time) error {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 100*time.Millisecond {
			return errors.New("renewal I/O is only bounded by the task deadline")
		}
		<-ctx.Done()
		return ctx.Err()
	}
	catalog, _ := NewTaskCatalog(definition)
	executor := NewExecutor(repository, catalog, "worker-1")
	executor.heartbeatInterval = 10 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := executor.Execute(ctx, JobEnvelope{SchemaVersion: 1, JobID: 1, Attempt: 1, DispatchToken: "dispatch"})
	if !errors.Is(err, context.DeadlineExceeded) || repository.completed != 0 || repository.failed != 0 {
		t.Fatalf("err=%v completed=%d failed=%d", err, repository.completed, repository.failed)
	}
}

func TestRepositoryRunLeaseRejectsExpiredAndReplacedOwners(t *testing.T) {
	db, ctx := openSchedulerRepositoryDatabase(t)
	if err := db.WithContext(ctx).Exec(`CREATE TABLE system_scheduler_job (id BIGINT PRIMARY KEY,status SMALLINT NOT NULL,attempt_count INTEGER NOT NULL,run_token TEXT,run_lease_until TIMESTAMPTZ,updated_at TIMESTAMPTZ NOT NULL,error_class TEXT,last_error TEXT,completed_at TIMESTAMPTZ,available_at TIMESTAMPTZ);
CREATE TABLE system_scheduler_run (id BIGINT PRIMARY KEY,job_id BIGINT,attempt_no INTEGER,status SMALLINT,started_at TIMESTAMPTZ,finished_at TIMESTAMPTZ,duration_ms BIGINT,error_class TEXT,error_message TEXT,updated_at TIMESTAMPTZ);`).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	for _, tc := range []struct {
		name, token string
		attempt     int
		until       time.Time
	}{
		{"replaced-token", "old", 1, now.Add(time.Minute)}, {"expired-token", "current", 1, now}, {"wrong-attempt", "current", 2, now.Add(time.Minute)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := db.WithContext(ctx).Exec(`TRUNCATE system_scheduler_job,system_scheduler_run`).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.WithContext(ctx).Exec(`INSERT INTO system_scheduler_job(id,status,attempt_count,run_token,run_lease_until,updated_at) VALUES(1,3,1,'current',?,?)`, tc.until, now).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.WithContext(ctx).Exec(`INSERT INTO system_scheduler_run(id,job_id,attempt_no,status,started_at,updated_at) VALUES(1,1,1,1,?,?)`, now, now).Error; err != nil {
				t.Fatal(err)
			}
			for _, operation := range []func() error{
				func() error { return repository.RenewRun(ctx, 1, tc.attempt, tc.token, now) },
				func() error { return repository.CompleteRun(ctx, 1, tc.attempt, tc.token, now) },
				func() error { return repository.FailRun(ctx, 1, tc.attempt, tc.token, "failed", "failed", nil, now) },
			} {
				if err := operation(); !errors.Is(err, ErrClaimLost) {
					t.Fatalf("got %v, want ErrClaimLost", err)
				}
			}
			var run Run
			if err := db.First(&run, 1).Error; err != nil {
				t.Fatal(err)
			}
			if run.Status != RunRunning {
				t.Fatalf("stale owner mutated run: %+v", run)
			}
		})
	}
	if err := db.Exec(`UPDATE system_scheduler_job SET run_lease_until=?`, now.Add(time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.RenewRun(ctx, 1, 1, "current", now); err != nil {
		t.Fatal(err)
	}
	var job Job
	if err := db.First(&job, 1).Error; err != nil {
		t.Fatal(err)
	}
	if job.RunLeaseUntil == nil || !job.RunLeaseUntil.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("lease=%v", job.RunLeaseUntil)
	}
	if err := repository.CompleteRun(ctx, 1, 1, "current", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
}
