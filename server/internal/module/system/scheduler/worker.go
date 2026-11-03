package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

type executionRepository interface {
	GetJob(context.Context, int64) (Job, error)
	ClaimRun(context.Context, int64, int, string, string, string, time.Time, time.Duration) (Run, Job, error)
	RenewRun(context.Context, int64, int, string, time.Time) error
	CompleteRun(context.Context, int64, int, string, time.Time) error
	FailRun(context.Context, int64, int, string, string, string, *time.Time, time.Time) error
}

type Executor struct {
	repository        executionRepository
	catalog           TaskCatalog
	workerID          string
	now               func() time.Time
	lease             time.Duration
	heartbeatInterval time.Duration
}

func NewExecutor(repository executionRepository, catalog TaskCatalog, workerID string) *Executor {
	return &Executor{repository: repository, catalog: catalog, workerID: workerID, now: time.Now, lease: 5 * time.Minute, heartbeatInterval: time.Minute}
}

func (e *Executor) Execute(ctx context.Context, envelope JobEnvelope) error {
	if e == nil || e.repository == nil || e.workerID == "" {
		return errors.New("scheduler executor is not configured")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	job, err := e.repository.GetJob(ctx, envelope.JobID)
	if err != nil {
		return err
	}
	switch job.Status {
	case JobCompleted, JobFailed, JobCanceled:
		return nil
	case JobScheduled:
		return errors.New("scheduler job is not queued yet")
	case JobRunning:
		return errors.New("scheduler job is already running")
	case JobQueued:
	default:
		return errors.New("scheduler job has invalid status")
	}
	if job.AttemptCount+1 != envelope.Attempt || job.PublishToken != envelope.DispatchToken {
		return fmt.Errorf("stale scheduler delivery: %w", asynq.SkipRetry)
	}
	definition, ok := e.catalog.Lookup(job.TaskType)
	if ok {
		if err := e.catalog.ValidatePayload(job.TaskType, job.Payload); err != nil {
			definition.TimeoutForPayload = nil
		} else if definition.TimeoutForPayload != nil {
			timeout, resolveErr := definition.TimeoutForPayload(ctx, job.Payload, e.now().UTC())
			if resolveErr != nil || timeout <= 0 {
				if resolveErr == nil {
					resolveErr = ErrInvalidTaskDefinition
				}
				return fmt.Errorf("resolve scheduler job timeout: %w", resolveErr)
			}
			definition.Timeout = timeout
		}
	}
	token := newToken()
	now := e.now().UTC()
	run, claimed, err := e.repository.ClaimRun(ctx, job.ID, envelope.Attempt, envelope.DispatchToken, e.workerID, token, now, e.lease)
	if err != nil {
		return err
	}
	if !ok {
		return e.finishFailure(ctx, claimed, run, token, "unknown-task-type", nil, now)
	}
	if err := e.catalog.ValidatePayload(claimed.TaskType, claimed.Payload); err != nil {
		return e.finishFailure(ctx, claimed, run, token, "invalid-payload", nil, now)
	}
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, definition.Timeout)
	defer timeoutCancel()
	executionCtx, cancel := context.WithCancelCause(timeoutCtx)
	defer cancel(nil)
	heartbeatCtx, stopHeartbeat := context.WithCancel(executionCtx)
	heartbeatDone := make(chan error, 1)
	go func() { heartbeatDone <- e.renewLease(heartbeatCtx, claimed, token, cancel) }()
	if err := executionCtx.Err(); err != nil {
		stopHeartbeat()
		<-heartbeatDone
		return err
	}
	err = definition.Execute(executionCtx, ExecutionContext{JobID: claimed.ID, Attempt: claimed.AttemptCount, MaxAttempts: claimed.MaxAttempts, TaskType: claimed.TaskType, Payload: append([]byte(nil), claimed.Payload...), ScheduledAt: claimed.ScheduledAt, StartedAt: run.StartedAt})
	stopHeartbeat()
	heartbeatErr := <-heartbeatDone
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if heartbeatErr != nil {
		return heartbeatErr
	}
	if executionCtx.Err() != nil {
		err = context.Cause(executionCtx)
	}
	finished := e.now().UTC()
	if err == nil {
		return e.repository.CompleteRun(ctx, claimed.ID, claimed.AttemptCount, token, finished)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var retryAt *time.Time
	if claimed.AttemptCount < claimed.MaxAttempts {
		delay := time.Second
		if definition.Backoff != nil {
			delay = definition.Backoff(claimed.AttemptCount)
		}
		if delay < time.Second {
			delay = time.Second
		}
		if delay > 5*time.Minute {
			delay = 5 * time.Minute
		}
		value := finished.Add(delay)
		retryAt = &value
	}
	return e.finishFailure(ctx, claimed, run, token, taskErrorClass(err), retryAt, finished)
}

func (e *Executor) renewLease(ctx context.Context, job Job, token string, cancel context.CancelCauseFunc) error {
	ticker := time.NewTicker(e.heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := ctx.Err(); err != nil {
				return nil
			}
			// A stalled database operation must not keep the business context
			// alive until its much longer generation deadline.
			renewCtx, renewCancel := context.WithTimeout(ctx, e.heartbeatInterval)
			err := e.repository.RenewRun(renewCtx, job.ID, job.AttemptCount, token, e.now().UTC())
			renewCancel()
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				cancel(err)
				return err
			}
		}
	}
}

func (e *Executor) finishFailure(ctx context.Context, job Job, run Run, token, class string, retryAt *time.Time, now time.Time) error {
	if err := e.repository.FailRun(ctx, job.ID, run.AttemptNo, token, class, class, retryAt, now); err != nil {
		return err
	}
	return nil
}

func taskErrorClass(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return "task-failed"
	}
}

type WorkerHandler struct{ executor *Executor }

func NewWorkerHandler(executor *Executor) *WorkerHandler { return &WorkerHandler{executor: executor} }
func (h *WorkerHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	if h == nil || h.executor == nil {
		return errors.New("scheduler worker handler is not configured")
	}
	envelope, err := DecodeJobEnvelope(task.Payload())
	if err != nil {
		return fmt.Errorf("decode scheduler envelope: %v: %w", err, asynq.SkipRetry)
	}
	return h.executor.Execute(ctx, envelope)
}
func Register(mux *asynq.ServeMux, handler *WorkerHandler) { mux.Handle(EnvelopeTaskType, handler) }
