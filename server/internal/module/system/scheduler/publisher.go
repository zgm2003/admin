package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
)

const EnvelopeTaskType = "system:scheduler:execute:v1"

const publisherMaxConsecutiveFailures = 5

type QueueOptions struct {
	TaskID    string
	Queue     string
	Timeout   time.Duration
	MaxRetry  int
	ProcessAt time.Time
}

type QueueEnqueuer interface {
	Enqueue(context.Context, string, []byte, QueueOptions) error
}

type publishRepository interface {
	ClaimPublishableJobs(context.Context, time.Time, int, string, time.Duration) ([]PublishableJob, error)
	MarkQueued(context.Context, int64, string, time.Time) error
	ReschedulePublish(context.Context, int64, string, string, time.Time, time.Time) error
	RecoverExpiredQueued(context.Context, time.Time, int) (int, error)
	RecoverExpiredRuns(context.Context, time.Time, int) (int, error)
}

type Publisher struct {
	repository publishRepository
	queue      QueueEnqueuer
	catalog    TaskCatalog
	logger     *slog.Logger
	now        func() time.Time
	idle       time.Duration
	lease      time.Duration
	batchSize  int
}

func NewPublisher(repository publishRepository, queue QueueEnqueuer, catalog TaskCatalog, logger *slog.Logger) *Publisher {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Publisher{repository: repository, queue: queue, catalog: catalog, logger: logger, now: time.Now, idle: time.Second, lease: 30 * time.Second, batchSize: 50}
}

func (p *Publisher) Run(ctx context.Context) error {
	if p == nil || p.repository == nil || p.queue == nil {
		return errors.New("scheduler publisher is not configured")
	}
	consecutiveFailures := 0
	for {
		count, err := p.RunOnce(ctx, p.now().UTC())
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			consecutiveFailures++
			p.logger.Error("scheduler publisher iteration failed", "error", err, "consecutiveFailures", consecutiveFailures)
			if consecutiveFailures >= publisherMaxConsecutiveFailures {
				return fmt.Errorf("scheduler publisher unhealthy after %d consecutive failures: %w", consecutiveFailures, err)
			}
		} else {
			consecutiveFailures = 0
		}
		if ctx.Err() != nil {
			return nil
		}
		if count == p.batchSize {
			continue
		}
		timer := time.NewTimer(p.idle)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

func (p *Publisher) RunOnce(ctx context.Context, now time.Time) (int, error) {
	if p == nil || p.repository == nil || p.queue == nil || now.IsZero() {
		return 0, errors.New("scheduler publisher is not configured")
	}
	if _, err := p.repository.RecoverExpiredQueued(ctx, now.UTC(), p.batchSize); err != nil {
		return 0, fmt.Errorf("recover expired scheduler deliveries: %w", err)
	}
	if _, err := p.repository.RecoverExpiredRuns(ctx, now.UTC(), p.batchSize); err != nil {
		return 0, fmt.Errorf("recover expired scheduler runs: %w", err)
	}
	token := newToken()
	jobs, err := p.repository.ClaimPublishableJobs(ctx, now.UTC(), p.batchSize, token, p.lease)
	if err != nil {
		return 0, err
	}
	published := 0
	var iterationErr error
	for _, claimed := range jobs {
		job := claimed.Job
		definition, ok := p.catalog.Lookup(job.TaskType)
		if !ok {
			iterationErr = errors.Join(iterationErr, p.repository.ReschedulePublish(ctx, job.ID, claimed.DispatchToken, "unknown-task-type", now.Add(5*time.Minute), now))
			continue
		}
		attempt := job.AttemptCount + 1
		envelope := JobEnvelope{SchemaVersion: 1, JobID: job.ID, Attempt: attempt, DispatchToken: claimed.DispatchToken}
		raw, encodeErr := json.Marshal(envelope)
		if encodeErr != nil {
			iterationErr = errors.Join(iterationErr, encodeErr)
			continue
		}
		options := QueueOptions{TaskID: fmt.Sprintf("system-scheduler:%d:%d", job.ID, attempt), Queue: definition.Queue, Timeout: definition.Timeout, MaxRetry: definition.MaxAttempts - 1, ProcessAt: job.AvailableAt}
		enqueueErr := p.queue.Enqueue(ctx, EnvelopeTaskType, raw, options)
		if enqueueErr == nil || errors.Is(enqueueErr, asynq.ErrTaskIDConflict) {
			if markErr := p.repository.MarkQueued(ctx, job.ID, claimed.DispatchToken, now); markErr != nil {
				iterationErr = errors.Join(iterationErr, fmt.Errorf("mark scheduler job %d queued: %w", job.ID, markErr))
				continue
			}
			published++
			continue
		}
		delay := publishBackoff(job.AttemptCount + 1)
		if rescheduleErr := p.repository.ReschedulePublish(ctx, job.ID, claimed.DispatchToken, "dependency-unavailable", now.Add(delay), now); rescheduleErr != nil {
			iterationErr = errors.Join(iterationErr, fmt.Errorf("reschedule scheduler job %d: %w", job.ID, rescheduleErr))
		}
		iterationErr = errors.Join(iterationErr, fmt.Errorf("enqueue scheduler job %d: %w", job.ID, enqueueErr))
	}
	return published, iterationErr
}

func publishBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Second
	for step := 1; step < attempt && delay < 5*time.Minute; step++ {
		delay *= 2
	}
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}

type AsynqEnqueuer struct {
	client interface {
		Enqueue(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
	}
}

func NewAsynqEnqueuer(client interface {
	Enqueue(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
}) *AsynqEnqueuer {
	return &AsynqEnqueuer{client: client}
}
func (e *AsynqEnqueuer) Enqueue(ctx context.Context, taskType string, payload []byte, options QueueOptions) error {
	if e == nil || e.client == nil || taskType == "" || options.TaskID == "" || options.Queue == "" || options.Timeout <= 0 {
		return errors.New("scheduler queue options are invalid")
	}
	asynqOptions := []asynq.Option{asynq.TaskID(options.TaskID), asynq.Queue(options.Queue), asynq.Timeout(options.Timeout), asynq.MaxRetry(options.MaxRetry)}
	if !options.ProcessAt.IsZero() && options.ProcessAt.After(time.Now()) {
		asynqOptions = append(asynqOptions, asynq.ProcessAt(options.ProcessAt))
	}
	_, err := e.client.Enqueue(ctx, asynq.NewTask(taskType, payload), asynqOptions...)
	return err
}
