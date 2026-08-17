package taskdemo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"admin/server/internal/shared/apperror"
)

type taskStore interface {
	Create(context.Context, *Task) error
	Find(context.Context, string) (Task, error)
	UpdateStatus(context.Context, string, string) error
}

type Enqueuer interface {
	Enqueue(context.Context, string) error
}

type Service struct {
	repository taskStore
	enqueuer   Enqueuer
	logger     *slog.Logger
}

func NewService(repository taskStore, enqueuer Enqueuer, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{repository: repository, enqueuer: enqueuer, logger: logger}
}

func (s *Service) Create(ctx context.Context, message string) (Created, error) {
	message = strings.TrimSpace(message)
	if message == "" || len([]rune(message)) > 200 {
		return Created{}, apperror.InvalidRequest(fmt.Errorf("message must contain 1 to 200 characters"))
	}
	if s.enqueuer == nil {
		return Created{}, apperror.Internal(fmt.Errorf("task enqueuer is required"))
	}

	taskID, err := newTaskID()
	if err != nil {
		return Created{}, apperror.Internal(err)
	}
	task := Task{ID: taskID, Message: message, Status: StatusPending}
	if err := s.repository.Create(ctx, &task); err != nil {
		return Created{}, apperror.Internal(err)
	}
	if err := s.enqueuer.Enqueue(ctx, taskID); err != nil {
		statusErr := s.repository.UpdateStatus(ctx, taskID, StatusFailed)
		return Created{}, apperror.Internal(errors.Join(err, statusErr))
	}
	return Created{TaskID: taskID}, nil
}

func (s *Service) Process(ctx context.Context, taskID string) error {
	if strings.TrimSpace(taskID) == "" {
		return apperror.InvalidRequest(fmt.Errorf("task ID is required"))
	}
	task, err := s.repository.Find(ctx, taskID)
	if err != nil {
		return apperror.Internal(err)
	}
	if err := s.repository.UpdateStatus(ctx, taskID, StatusRunning); err != nil {
		return apperror.Internal(err)
	}

	s.logger.InfoContext(ctx, "processing foundation task", "taskId", task.ID, "message", task.Message)

	if err := s.repository.UpdateStatus(ctx, taskID, StatusCompleted); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func newTaskID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate task ID: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
