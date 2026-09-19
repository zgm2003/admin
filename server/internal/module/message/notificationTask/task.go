package notificationtask

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"admin/server/internal/module/message/notification"
	"admin/server/internal/module/realtime"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Processor struct {
	db       *gorm.DB
	realtime *realtime.Repository
}

var ErrFrozenFacts = errors.New("notification task frozen facts are invalid")

func NewProcessor(db *gorm.DB, realtimeRepository *realtime.Repository) *Processor {
	return &Processor{db: db, realtime: realtimeRepository}
}
func (p *Processor) Process(ctx context.Context, payload BatchPayload) error {
	if payload.SchemaVersion != 1 || payload.TaskID <= 0 || payload.BatchNo < 0 {
		return errors.New("invalid notification batch")
	}
	return p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return p.processTx(ctx, tx, payload) })
}
func (p *Processor) processTx(ctx context.Context, tx *gorm.DB, payload BatchPayload) error {
	var task Task
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND deleted_at IS NULL", payload.TaskID).Take(&task).Error; err != nil {
		return err
	}
	if task.Status == StatusCanceled || task.Status == StatusCompleted {
		return nil
	}
	if task.Status != StatusQueued && task.Status != StatusScheduled && task.Status != StatusProcessing {
		return fmt.Errorf("%w: status", ErrFrozenFacts)
	}
	if task.NextBatchNo != payload.BatchNo {
		return nil
	}
	if task.SubmittedAt == nil || task.AudienceMaxUserID == nil {
		return fmt.Errorf("%w: metadata", ErrFrozenFacts)
	}
	now := time.Now().UTC()
	notificationID, publishedAt, created, err := p.ensureNotification(ctx, tx, task, now)
	if err != nil {
		return err
	}
	if task.AudienceType == AudiencePlatform {
		return p.processPlatform(ctx, tx, task, notificationID, publishedAt, created, now)
	}
	users, more, err := p.batchUsers(ctx, tx, task)
	if err != nil {
		return err
	}
	inserted, err := p.insertRecipients(ctx, tx, task.PlatformID, notificationID, users, now)
	if err != nil {
		return err
	}
	for _, userID := range inserted {
		if err = p.appendUserEvent(ctx, tx, task, notificationID, userID, publishedAt); err != nil {
			return err
		}
	}
	nextUser := task.NextUserID
	if len(users) > 0 {
		nextUser = users[len(users)-1]
	}
	updates := map[string]any{"status": StatusProcessing, "next_user_id": nextUser, "next_batch_no": task.NextBatchNo + 1, "generated_count": gorm.Expr("generated_count + ?", len(inserted)), "published_at": gorm.Expr("COALESCE(published_at, ?)", publishedAt), "updated_at": now}
	if !more {
		updates["status"] = StatusCompleted
		updates["completed_at"] = now
	}
	if err = tx.WithContext(ctx).Model(&Task{}).Where("id=?", task.ID).Updates(updates).Error; err != nil {
		return err
	}
	if more {
		return tx.WithContext(ctx).Create(&DispatchOutbox{TaskID: task.ID, BatchNo: task.NextBatchNo + 1, AvailableAt: now, CreatedAt: now, UpdatedAt: now}).Error
	}
	return nil
}
func (p *Processor) ensureNotification(ctx context.Context, tx *gorm.DB, task Task, now time.Time) (int64, time.Time, bool, error) {
	audience := notification.AudienceTargeted
	if task.AudienceType == AudiencePlatform {
		audience = notification.AudiencePlatform
	}
	result := tx.WithContext(ctx).Exec(`INSERT INTO message_notification(platform_id,source_task_id,source_type,source_key,audience_type,audience_max_user_id,title,content_html,summary,variant,priority,link_type,link,published_at,created_at,updated_at) VALUES(?,?,'message.notificationtask',?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(source_task_id) WHERE source_task_id IS NOT NULL DO NOTHING`, task.PlatformID, task.ID, fmt.Sprint(task.ID), audience, *task.AudienceMaxUserID, task.Title, task.ContentHTML, task.Summary, task.Variant, task.Priority, task.LinkType, task.Link, now, now, now)
	if result.Error != nil {
		return 0, time.Time{}, false, result.Error
	}
	var row struct {
		ID          int64
		PublishedAt time.Time
	}
	if err := tx.WithContext(ctx).Raw(`SELECT id,published_at FROM message_notification WHERE source_task_id=?`, task.ID).Scan(&row).Error; err != nil || row.ID == 0 || row.PublishedAt.IsZero() {
		return 0, time.Time{}, false, errors.New("notification creation returned no row")
	}
	return row.ID, row.PublishedAt.UTC(), result.RowsAffected == 1, nil
}
func (p *Processor) processPlatform(ctx context.Context, tx *gorm.DB, task Task, notificationID int64, publishedAt time.Time, created bool, now time.Time) error {
	if created {
		payload, _ := eventPayload(task, notificationID, publishedAt)
		if _, err := p.realtime.AppendTx(ctx, tx, realtime.EventInput{EventID: uuid.NewString(), DedupKey: fmt.Sprintf("notification:%d:platform", notificationID), PlatformID: task.PlatformID, EventType: realtime.EventNotificationCreated, TargetType: realtime.TargetPlatform, AudienceMaxUserID: task.AudienceMaxUserID, Payload: payload, OccurredAt: publishedAt}); err != nil {
			return err
		}
	}
	return tx.WithContext(ctx).Model(&Task{}).Where("id=?", task.ID).Updates(map[string]any{"status": StatusCompleted, "generated_count": 1, "next_batch_no": task.NextBatchNo + 1, "published_at": gorm.Expr("COALESCE(published_at, ?)", publishedAt), "completed_at": now, "updated_at": now}).Error
}
func (p *Processor) batchUsers(ctx context.Context, tx *gorm.DB, task Task) ([]int64, bool, error) {
	var ids []int64
	if task.AudienceType == AudienceUser {
		err := tx.WithContext(ctx).Raw(`SELECT target_id FROM message_notification_task_target WHERE task_id=? AND target_type='user' AND deleted_at IS NULL AND target_id>? AND target_id<=? ORDER BY target_id LIMIT 501`, task.ID, task.NextUserID, *task.AudienceMaxUserID).Scan(&ids).Error
		if err != nil {
			return nil, false, err
		}
	} else {
		err := tx.WithContext(ctx).Raw(`SELECT DISTINCT user_role.user_id FROM permission_user_role user_role JOIN message_notification_task_target target ON target.task_id=? AND target.target_type='role' AND target.target_id=user_role.role_id AND target.deleted_at IS NULL WHERE user_role.created_at<=? AND (user_role.deleted_at IS NULL OR user_role.deleted_at>?) AND user_role.user_id>? AND user_role.user_id<=? ORDER BY user_role.user_id LIMIT 501`, task.ID, *task.SubmittedAt, *task.SubmittedAt, task.NextUserID, *task.AudienceMaxUserID).Scan(&ids).Error
		if err != nil {
			return nil, false, err
		}
	}
	more := len(ids) > 500
	if more {
		ids = ids[:500]
	}
	return ids, more, nil
}
func (p *Processor) insertRecipients(ctx context.Context, tx *gorm.DB, platformID, notificationID int64, users []int64, now time.Time) ([]int64, error) {
	if len(users) == 0 {
		return nil, nil
	}
	encodedUsers, err := json.Marshal(users)
	if err != nil {
		return nil, err
	}
	var inserted []int64
	if err := tx.WithContext(ctx).Raw(`INSERT INTO message_notification_recipient(notification_id,platform_id,user_id,created_at,updated_at) SELECT ?,?,value::bigint,?,? FROM jsonb_array_elements_text(?::jsonb) AS value ON CONFLICT(notification_id,user_id) DO NOTHING RETURNING user_id`, notificationID, platformID, now, now, string(encodedUsers)).Scan(&inserted).Error; err != nil {
		return nil, err
	}
	return inserted, nil
}
func (p *Processor) appendUserEvent(ctx context.Context, tx *gorm.DB, task Task, notificationID, userID int64, publishedAt time.Time) error {
	payload, _ := eventPayload(task, notificationID, publishedAt)
	_, err := p.realtime.AppendTx(ctx, tx, realtime.EventInput{EventID: uuid.NewString(), DedupKey: fmt.Sprintf("notification:%d:user:%d", notificationID, userID), PlatformID: task.PlatformID, EventType: realtime.EventNotificationCreated, TargetType: realtime.TargetUser, TargetUserID: &userID, Payload: payload, OccurredAt: publishedAt})
	return err
}
func eventPayload(task Task, notificationID int64, publishedAt time.Time) (json.RawMessage, error) {
	return json.Marshal(struct {
		NotificationID int64                 `json:"notificationId"`
		Title          string                `json:"title"`
		Summary        string                `json:"summary"`
		Variant        notification.Variant  `json:"variant"`
		Priority       notification.Priority `json:"priority"`
		LinkType       notification.LinkType `json:"linkType"`
		Link           string                `json:"link"`
		PublishedAt    time.Time             `json:"publishedAt"`
	}{notificationID, task.Title, task.Summary, task.Variant, task.Priority, task.LinkType, task.Link, publishedAt})
}

type batchProcessor interface {
	Process(context.Context, BatchPayload) error
}

type TaskHandler struct {
	processor batchProcessor
	failer    interface {
		MarkFailed(context.Context, int64, string, time.Time) error
	}
	retriesExhausted func(context.Context) bool
}

func NewTaskHandler(processor *Processor, failer interface {
	MarkFailed(context.Context, int64, string, time.Time) error
}) *TaskHandler {
	return &TaskHandler{
		processor: processor,
		failer:    failer,
		retriesExhausted: func(ctx context.Context) bool {
			retryCount, hasRetryCount := asynq.GetRetryCount(ctx)
			maxRetry, hasMaxRetry := asynq.GetMaxRetry(ctx)
			return hasRetryCount && hasMaxRetry && retryCount >= maxRetry
		},
	}
}
func (h *TaskHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := DecodeBatchPayload(task.Payload())
	if err != nil {
		return fmt.Errorf("invalid notification task payload: %v: %w", err, asynq.SkipRetry)
	}
	if err = h.processor.Process(ctx, payload); err != nil {
		if errors.Is(err, ErrFrozenFacts) {
			if h.failer == nil {
				return errors.New("notification task failure persistence is unavailable")
			}
			if markErr := h.failer.MarkFailed(ctx, payload.TaskID, "frozen-facts-invalid", time.Now().UTC()); markErr != nil {
				return fmt.Errorf("persist notification task failure: %w", markErr)
			}
			return fmt.Errorf("notification task cannot be processed: %w", asynq.SkipRetry)
		}
		if h.retriesExhausted != nil && h.retriesExhausted(ctx) && h.failer != nil {
			if markErr := h.failer.MarkFailed(ctx, payload.TaskID, "worker-retries-exhausted", time.Now().UTC()); markErr != nil {
				return fmt.Errorf("notification task failed after retries; persist failed state: %w", markErr)
			}
		}
		return err
	}
	return nil
}
func Register(mux *asynq.ServeMux, handler *TaskHandler) { mux.Handle(TaskType, handler) }
func (r *Repository) MarkFailed(ctx context.Context, id int64, message string, now time.Time) error {
	if len(message) > 512 {
		message = message[:512]
	}
	return r.db.WithContext(ctx).Model(&Task{}).Where("id=? AND status NOT IN ('completed','canceled','failed')", id).Updates(map[string]any{"status": StatusFailed, "failure_message": message, "failed_at": now, "updated_at": now}).Error
}
