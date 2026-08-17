package taskdemo_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"

	"admin/server/internal/module/taskdemo"
)

type taskStore struct {
	task       taskdemo.Task
	createErr  error
	findErr    error
	updateErr  error
	operations *[]string
}

func (s *taskStore) Create(_ context.Context, task *taskdemo.Task) error {
	*s.operations = append(*s.operations, "repository.create")
	s.task = *task
	return s.createErr
}

func (s *taskStore) Find(_ context.Context, taskID string) (taskdemo.Task, error) {
	*s.operations = append(*s.operations, "repository.find:"+taskID)
	return s.task, s.findErr
}

func (s *taskStore) UpdateStatus(_ context.Context, taskID, status string) error {
	*s.operations = append(*s.operations, "repository.status:"+status)
	s.task.ID = taskID
	s.task.Status = status
	return s.updateErr
}

type enqueuer struct {
	err        error
	taskID     string
	operations *[]string
}

func (e *enqueuer) Enqueue(_ context.Context, taskID string) error {
	*e.operations = append(*e.operations, "queue.enqueue")
	e.taskID = taskID
	return e.err
}

func TestCreatePersistsBeforeEnqueue(t *testing.T) {
	operations := []string{}
	store := &taskStore{operations: &operations}
	queue := &enqueuer{operations: &operations}
	service := taskdemo.NewService(store, queue, slog.New(slog.NewTextHandler(io.Discard, nil)))

	created, err := service.Create(context.Background(), "foundation-check")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.TaskID == "" || queue.taskID != created.TaskID || store.task.ID != created.TaskID {
		t.Fatalf("IDs created=%q queued=%q stored=%q", created.TaskID, queue.taskID, store.task.ID)
	}
	if store.task.Message != "foundation-check" || store.task.Status != taskdemo.StatusPending {
		t.Fatalf("stored task = %+v", store.task)
	}
	if !reflect.DeepEqual(operations, []string{"repository.create", "queue.enqueue"}) {
		t.Fatalf("operations = %v", operations)
	}
}

func TestCreateMarksRecordFailedWhenEnqueueFails(t *testing.T) {
	operations := []string{}
	store := &taskStore{operations: &operations}
	queue := &enqueuer{operations: &operations, err: errors.New("redis down")}
	service := taskdemo.NewService(store, queue, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := service.Create(context.Background(), "foundation-check")
	if err == nil {
		t.Fatal("expected enqueue failure")
	}
	if !reflect.DeepEqual(operations, []string{"repository.create", "queue.enqueue", "repository.status:failed"}) {
		t.Fatalf("operations = %v", operations)
	}
}

func TestCreateRejectsBlankMessageBeforeWriting(t *testing.T) {
	operations := []string{}
	service := taskdemo.NewService(
		&taskStore{operations: &operations},
		&enqueuer{operations: &operations},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	_, err := service.Create(context.Background(), "   ")
	if err == nil {
		t.Fatal("expected blank message to fail")
	}
	if len(operations) != 0 {
		t.Fatalf("operations = %v", operations)
	}
}

func TestProcessUsesStoredMessageAndUpdatesStatus(t *testing.T) {
	operations := []string{}
	store := &taskStore{
		operations: &operations,
		task:       taskdemo.Task{ID: "task-1", Message: "stored-message", Status: taskdemo.StatusPending},
	}
	service := taskdemo.NewService(store, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := service.Process(context.Background(), "task-1"); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	want := []string{"repository.find:task-1", "repository.status:running", "repository.status:completed"}
	if !reflect.DeepEqual(operations, want) {
		t.Fatalf("operations = %v, want %v", operations, want)
	}
}
