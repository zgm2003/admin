package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

type publishRepositoryStub struct {
	job         PublishableJob
	marked      int
	rescheduled int
	markErr     error
}

func (s *publishRepositoryStub) RecoverExpiredRuns(context.Context, time.Time, int) (int, error) {
	return 0, nil
}
func (s *publishRepositoryStub) RecoverExpiredQueued(context.Context, time.Time, int) (int, error) {
	return 0, nil
}
func (s *publishRepositoryStub) ClaimPublishableJobs(context.Context, time.Time, int, string, time.Duration) ([]PublishableJob, error) {
	return []PublishableJob{s.job}, nil
}
func (s *publishRepositoryStub) MarkQueued(context.Context, int64, string, time.Time) error {
	s.marked++
	return s.markErr
}
func (s *publishRepositoryStub) ReschedulePublish(context.Context, int64, string, string, time.Time, time.Time) error {
	s.rescheduled++
	return nil
}

type queueStub struct {
	err      error
	taskType string
	raw      []byte
	options  QueueOptions
}

func (s *queueStub) Enqueue(_ context.Context, taskType string, raw []byte, options QueueOptions) error {
	s.taskType = taskType
	s.raw = append([]byte(nil), raw...)
	s.options = options
	return s.err
}

func TestPublisherEnqueuesStaticDefinitionAndMarksQueued(t *testing.T) {
	now := time.Date(2026, 9, 20, 2, 0, 0, 0, time.UTC)
	repository := &publishRepositoryStub{job: PublishableJob{Job: Job{ID: 9, TaskType: "example", AttemptCount: 0, AvailableAt: now, Status: JobScheduled}, DispatchToken: "lease"}}
	queue := &queueStub{}
	catalog, _ := NewTaskCatalog(testDefinition("example"))
	publisher := NewPublisher(repository, queue, catalog, nil)
	count, err := publisher.RunOnce(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || repository.marked != 1 || repository.rescheduled != 0 {
		t.Fatalf("count=%d repository=%+v", count, repository)
	}
	if queue.taskType != EnvelopeTaskType || queue.options.TaskID != "system-scheduler:9:1" || queue.options.Queue != "default" {
		t.Fatalf("queue=%+v", queue)
	}
	var envelope JobEnvelope
	if err = json.Unmarshal(queue.raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.JobID != 9 || envelope.Attempt != 1 || envelope.DispatchToken != "lease" {
		t.Fatalf("envelope=%+v", envelope)
	}
}
func TestPublisherReschedulesRedisFailure(t *testing.T) {
	now := time.Now().UTC()
	repository := &publishRepositoryStub{job: PublishableJob{Job: Job{ID: 1, TaskType: "example", AvailableAt: now}, DispatchToken: "lease"}}
	queue := &queueStub{err: errors.New("redis down")}
	catalog, _ := NewTaskCatalog(testDefinition("example"))
	publisher := NewPublisher(repository, queue, catalog, nil)
	if count, err := publisher.RunOnce(context.Background(), now); err != nil || count != 0 || repository.rescheduled != 1 || repository.marked != 0 {
		t.Fatalf("count=%d err=%v repository=%+v", count, err, repository)
	}
}
func TestPublisherTreatsTaskIDConflictAsAlreadyPublished(t *testing.T) {
	now := time.Now().UTC()
	repository := &publishRepositoryStub{job: PublishableJob{Job: Job{ID: 1, TaskType: "example", AvailableAt: now}, DispatchToken: "lease"}}
	queue := &queueStub{err: asynq.ErrTaskIDConflict}
	catalog, _ := NewTaskCatalog(testDefinition("example"))
	publisher := NewPublisher(repository, queue, catalog, nil)
	if count, err := publisher.RunOnce(context.Background(), now); err != nil || count != 1 || repository.marked != 1 {
		t.Fatalf("count=%d err=%v repository=%+v", count, err, repository)
	}
}
