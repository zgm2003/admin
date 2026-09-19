package notificationtask

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"io"
	"log/slog"
	"time"
)

const TaskType = "message:notificationtask:dispatch:v1"

type BatchPayload struct {
	SchemaVersion int   `json:"schemaVersion"`
	TaskID        int64 `json:"taskId"`
	BatchNo       int   `json:"batchNo"`
}
type DispatchClaim struct {
	ID       int64 `gorm:"column:id"`
	TaskID   int64 `gorm:"column:task_id"`
	BatchNo  int   `gorm:"column:batch_no"`
	Attempts int   `gorm:"column:attempts"`
}

func DecodeBatchPayload(raw []byte) (BatchPayload, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var payload BatchPayload
	if err := decoder.Decode(&payload); err != nil {
		return BatchPayload{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return BatchPayload{}, errors.New("payload must contain one JSON document")
	}
	if payload.SchemaVersion != 1 || payload.TaskID <= 0 || payload.BatchNo < 0 {
		return BatchPayload{}, errors.New("batch payload is invalid")
	}
	return payload, nil
}

type asynqClient interface {
	Enqueue(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
}
type QueueEnqueuer struct{ client asynqClient }

func NewQueueEnqueuer(client asynqClient) *QueueEnqueuer { return &QueueEnqueuer{client: client} }
func (e *QueueEnqueuer) EnqueueBatch(ctx context.Context, payload BatchPayload) error {
	if payload.SchemaVersion != 1 || payload.TaskID <= 0 || payload.BatchNo < 0 {
		return errors.New("batch payload is invalid")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = e.client.Enqueue(ctx, asynq.NewTask(TaskType, raw), asynq.TaskID(fmt.Sprintf("message-notification:%d:%d", payload.TaskID, payload.BatchNo)), asynq.MaxRetry(10), asynq.Timeout(60*time.Second))
	return err
}

type dispatchRepository interface {
	ClaimDispatch(context.Context, int, string, time.Time, time.Duration) ([]DispatchClaim, error)
	MarkDispatch(context.Context, int64, string, time.Time) error
	RescheduleDispatch(context.Context, int64, string, string, time.Time, time.Time) error
}
type dispatchQueue interface {
	EnqueueBatch(context.Context, BatchPayload) error
}
type DispatchRelay struct {
	repository dispatchRepository
	queue      dispatchQueue
	logger     *slog.Logger
	now        func() time.Time
}

func NewDispatchRelay(repository dispatchRepository, queue dispatchQueue, logger *slog.Logger) *DispatchRelay {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &DispatchRelay{repository: repository, queue: queue, logger: logger, now: time.Now}
}
func (r *DispatchRelay) Run(ctx context.Context) error {
	for {
		if _, err := r.RunOnce(ctx); err != nil && ctx.Err() == nil {
			r.logger.Error("notification dispatch relay failed", "errorClass", "dependency-unavailable")
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}
func (r *DispatchRelay) RunOnce(ctx context.Context) (int, error) {
	if r == nil || r.repository == nil || r.queue == nil {
		return 0, errors.New("dispatch relay is not configured")
	}
	now := r.now().UTC()
	token := uuid.NewString()
	rows, err := r.repository.ClaimDispatch(ctx, 50, token, now, 30*time.Second)
	if err != nil {
		return 0, err
	}
	published := 0
	var iterationErr error
	for _, row := range rows {
		err = r.queue.EnqueueBatch(ctx, BatchPayload{SchemaVersion: 1, TaskID: row.TaskID, BatchNo: row.BatchNo})
		if err == nil || errors.Is(err, asynq.ErrTaskIDConflict) {
			if markErr := r.repository.MarkDispatch(ctx, row.ID, token, now); markErr != nil {
				iterationErr = errors.Join(iterationErr, fmt.Errorf("mark notification dispatch %d: %w", row.ID, markErr))
				continue
			}
			published++
			continue
		}
		if rescheduleErr := r.repository.RescheduleDispatch(ctx, row.ID, token, "dependency-unavailable", now.Add(dispatchBackoff(row.Attempts)), now); rescheduleErr != nil {
			iterationErr = errors.Join(iterationErr, fmt.Errorf("reschedule notification dispatch %d: %w", row.ID, rescheduleErr))
		}
	}
	return published, iterationErr
}
func dispatchBackoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	delay := time.Second
	for step := 1; step < attempts && delay < 5*time.Minute; step++ {
		delay *= 2
	}
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}

func (r *Repository) ClaimDispatch(ctx context.Context, limit int, token string, now time.Time, lease time.Duration) ([]DispatchClaim, error) {
	var rows []DispatchClaim
	err := r.db.WithContext(ctx).Raw(`WITH candidates AS (SELECT outbox.id FROM message_notification_dispatch_outbox outbox JOIN message_notification_task task ON task.id=outbox.task_id WHERE outbox.published_at IS NULL AND outbox.available_at<=? AND (outbox.locked_until IS NULL OR outbox.locked_until<=?) AND task.status<>'canceled' ORDER BY outbox.available_at,outbox.id LIMIT ? FOR UPDATE OF outbox SKIP LOCKED), claimed AS (UPDATE message_notification_dispatch_outbox outbox SET locked_until=?,lock_token=?,updated_at=? FROM candidates WHERE outbox.id=candidates.id RETURNING outbox.id,outbox.task_id,outbox.batch_no,outbox.attempts) SELECT * FROM claimed ORDER BY id`, now, now, limit, now.Add(lease), token, now).Scan(&rows).Error
	return rows, err
}
func (r *Repository) MarkDispatch(ctx context.Context, id int64, token string, now time.Time) error {
	result := r.db.WithContext(ctx).Exec(`UPDATE message_notification_dispatch_outbox SET published_at=?,locked_until=NULL,lock_token=NULL,last_error=NULL,updated_at=? WHERE id=? AND published_at IS NULL AND lock_token=?`, now, now, id, token)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("dispatch claim lost")
	}
	return nil
}
func (r *Repository) RescheduleDispatch(ctx context.Context, id int64, token, errorClass string, available, now time.Time) error {
	result := r.db.WithContext(ctx).Exec(`UPDATE message_notification_dispatch_outbox SET attempts=attempts+1,available_at=?,locked_until=NULL,lock_token=NULL,last_error=?,updated_at=? WHERE id=? AND published_at IS NULL AND lock_token=?`, available, errorClass, now, id, token)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("dispatch claim lost")
	}
	return nil
}
