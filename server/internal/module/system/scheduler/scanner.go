package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type dueJobRepository interface {
	CreateDueJobs(context.Context, time.Time, int, DueJobBuilder) (int, error)
}

type Scanner struct {
	repository dueJobRepository
	catalog    TaskCatalog
	now        func() time.Time
	idle       time.Duration
	batchSize  int
}

func NewScanner(repository dueJobRepository, catalog TaskCatalog) *Scanner {
	return &Scanner{repository: repository, catalog: catalog, now: time.Now, idle: time.Second, batchSize: 50}
}

func (s *Scanner) Run(ctx context.Context) error {
	if s == nil || s.repository == nil {
		return errors.New("scheduler scanner is not configured")
	}
	for {
		count, err := s.RunOnce(ctx, s.now().UTC())
		if err != nil && ctx.Err() == nil {
			return fmt.Errorf("scan scheduler plans: %w", err)
		}
		if ctx.Err() != nil {
			return nil
		}
		if count == s.batchSize {
			continue
		}
		timer := time.NewTimer(s.idle)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

func (s *Scanner) RunOnce(ctx context.Context, now time.Time) (int, error) {
	if s == nil || s.repository == nil || now.IsZero() {
		return 0, errors.New("scheduler scanner is not configured")
	}
	return s.repository.CreateDueJobs(ctx, now.UTC(), s.batchSize, func(schedule Schedule, scheduledAt time.Time) (Job, error) {
		definition, ok := s.catalog.Lookup(schedule.TaskType)
		if !ok {
			return Job{}, ErrUnknownTaskType
		}
		payload := schedule.Params
		if len(payload) == 0 {
			payload = definition.DefaultParams
		}
		if err := s.catalog.ValidatePayload(schedule.TaskType, payload); err != nil {
			return Job{}, err
		}
		frozen := append(json.RawMessage(nil), payload...)
		return Job{TaskType: schedule.TaskType, Payload: frozen, TriggerSource: TriggerCron, SourceKey: fmt.Sprintf("schedule:%d:%d", schedule.ID, scheduledAt.UnixNano()), ScheduledAt: scheduledAt.UTC(), AvailableAt: now.UTC(), Status: JobScheduled, MaxAttempts: definition.MaxAttempts, Queue: definition.Queue, TimeoutSeconds: int(definition.Timeout / time.Second), CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
	})
}
