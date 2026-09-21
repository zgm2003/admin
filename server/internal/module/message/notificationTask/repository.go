package notificationtask

import (
	"admin/server/internal/module/message/notification"
	permissionnotification "admin/server/internal/module/permission/notification"
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

type BatchJobWriter interface {
	CreateBatchJobTx(context.Context, *gorm.DB, int64, int, time.Time, time.Time) error
	CancelBatchJobsTx(context.Context, *gorm.DB, int64, time.Time) error
}

type PermissionReaderFactory func(*gorm.DB) permissionnotification.Reader

type Repository struct {
	db                *gorm.DB
	jobs              BatchJobWriter
	permissions       permissionnotification.Reader
	permissionFactory func(*gorm.DB) permissionnotification.Reader
}

func NewRepository(db *gorm.DB, jobs ...BatchJobWriter) *Repository {
	var writer BatchJobWriter
	if len(jobs) > 0 {
		writer = jobs[0]
	}
	return newRepository(db, writer, func(tx *gorm.DB) permissionnotification.Reader { return permissionnotification.NewRepository(tx) })
}

func NewRepositoryWithPermissions(db *gorm.DB, jobs BatchJobWriter, factory PermissionReaderFactory) *Repository {
	return newRepository(db, jobs, factory)
}

func newRepository(db *gorm.DB, jobs BatchJobWriter, factory PermissionReaderFactory) *Repository {
	if factory == nil {
		return &Repository{db: db, jobs: jobs}
	}
	return &Repository{db: db, jobs: jobs, permissions: factory(db), permissionFactory: factory}
}
func (r *Repository) Transaction(ctx context.Context, fn func(*Repository) error) error {
	if r == nil || r.db == nil || r.jobs == nil || r.permissions == nil || r.permissionFactory == nil {
		return errors.New("notification task repository dependencies are required")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Repository{db: tx, jobs: r.jobs, permissions: r.permissionFactory(tx), permissionFactory: r.permissionFactory})
	})
}
func (r *Repository) Create(ctx context.Context, creator int64, input DraftInput) (Task, error) {
	var task Task
	err := r.Transaction(ctx, func(tx *Repository) error {
		if err := tx.validateDraftFacts(ctx, input); err != nil {
			return err
		}
		now := time.Now().UTC()
		task = taskFromDraft(0, creator, input, now)
		if err := tx.db.WithContext(ctx).Create(&task).Error; err != nil {
			return err
		}
		if err := tx.replaceTargets(ctx, task.ID, input.AudienceType, input.TargetIDs, now); err != nil {
			return err
		}
		created, err := tx.find(ctx, task.ID)
		if err != nil {
			return err
		}
		task = created
		return nil
	})
	return task, err
}
func (r *Repository) Update(ctx context.Context, id int64, input DraftInput) (Task, error) {
	var task Task
	err := r.Transaction(ctx, func(tx *Repository) error {
		current, err := tx.lockTask(ctx, id)
		if err != nil {
			return err
		}
		if current.Status != StatusDraft {
			return ErrNotDraft
		}
		if err = tx.validateDraftFacts(ctx, input); err != nil {
			return err
		}
		summary, _ := notification.SummaryFromHTML(input.ContentHTML)
		updates := map[string]any{"platform_id": input.PlatformID, "title": input.Title, "content_html": input.ContentHTML, "summary": summary, "variant": input.Variant, "priority": input.Priority, "link_type": input.LinkType, "link": input.Link, "audience_type": input.AudienceType, "scheduled_at": input.ScheduledAt, "updated_at": time.Now().UTC()}
		if err = tx.db.WithContext(ctx).Model(&Task{}).Where("id=? AND deleted_at IS NULL", id).Updates(updates).Error; err != nil {
			return err
		}
		if err = tx.replaceTargets(ctx, id, input.AudienceType, input.TargetIDs, time.Now().UTC()); err != nil {
			return err
		}
		task, err = tx.find(ctx, id)
		return err
	})
	return task, err
}
func (r *Repository) Submit(ctx context.Context, id int64, now time.Time) (Task, error) {
	var task Task
	err := r.Transaction(ctx, func(tx *Repository) error {
		current, err := tx.lockTask(ctx, id)
		if err != nil {
			return err
		}
		if current.Status != StatusDraft {
			return ErrNotDraft
		}
		targets, err := tx.targetIDs(ctx, id)
		if err != nil {
			return err
		}
		input := DraftInput{PlatformID: current.PlatformID, Title: current.Title, ContentHTML: current.ContentHTML, Variant: current.Variant, Priority: current.Priority, LinkType: current.LinkType, Link: current.Link, AudienceType: current.AudienceType, TargetIDs: targets, ScheduledAt: current.ScheduledAt}
		normalized, err := NormalizeDraft(input)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidFacts, err)
		}
		if err = tx.validateDraftFacts(ctx, normalized); err != nil {
			return err
		}
		if normalized.ScheduledAt != nil && !normalized.ScheduledAt.After(now) {
			return fmt.Errorf("%w: scheduled time must be in the future", ErrInvalidFacts)
		}
		var maximum int64
		if err = tx.db.WithContext(ctx).Raw(`SELECT COALESCE(max(id),0) FROM user_account`).Scan(&maximum).Error; err != nil {
			return err
		}
		status := StatusQueued
		available := now
		if normalized.ScheduledAt != nil && normalized.ScheduledAt.After(now) {
			status = StatusScheduled
			available = normalized.ScheduledAt.UTC()
		}
		summary, _ := notification.SummaryFromHTML(normalized.ContentHTML)
		result := tx.db.WithContext(ctx).Model(&Task{}).Where("id=? AND status=? AND deleted_at IS NULL", id, StatusDraft).Updates(map[string]any{"content_html": normalized.ContentHTML, "summary": summary, "status": status, "audience_max_user_id": maximum, "submitted_at": now, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrNotDraft
		}
		if err = tx.jobs.CreateBatchJobTx(ctx, tx.db, id, 0, available, now); err != nil {
			return err
		}
		task, err = tx.find(ctx, id)
		return err
	})
	return task, err
}
func (r *Repository) Cancel(ctx context.Context, id int64, now time.Time) (Task, error) {
	var task Task
	err := r.Transaction(ctx, func(tx *Repository) error {
		current, err := tx.lockTask(ctx, id)
		if err != nil {
			return err
		}
		if current.Status != StatusScheduled && current.Status != StatusQueued && current.Status != StatusProcessing {
			return ErrInvalidTransition
		}
		if err = tx.db.WithContext(ctx).Model(&Task{}).Where("id=?", id).Updates(map[string]any{"status": StatusCanceled, "canceled_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		if err = tx.jobs.CancelBatchJobsTx(ctx, tx.db, id, now); err != nil {
			return err
		}
		task, err = tx.find(ctx, id)
		return err
	})
	return task, err
}
func (r *Repository) Copy(ctx context.Context, id, creator int64, now time.Time) (Task, error) {
	var copied Task
	err := r.Transaction(ctx, func(tx *Repository) error {
		source, err := tx.find(ctx, id)
		if err != nil {
			return err
		}
		targets, err := tx.targetIDs(ctx, id)
		if err != nil {
			return err
		}
		input := DraftInput{PlatformID: source.PlatformID, Title: source.Title, ContentHTML: source.ContentHTML, Variant: source.Variant, Priority: source.Priority, LinkType: source.LinkType, Link: source.Link, AudienceType: source.AudienceType, TargetIDs: targets, ScheduledAt: source.ScheduledAt}
		if err = tx.validateDraftFacts(ctx, input); err != nil {
			return err
		}
		copied = taskFromDraft(0, creator, input, now)
		if err = tx.db.WithContext(ctx).Create(&copied).Error; err != nil {
			return err
		}
		if err = tx.replaceTargets(ctx, copied.ID, input.AudienceType, input.TargetIDs, now); err != nil {
			return err
		}
		copied, err = tx.find(ctx, copied.ID)
		return err
	})
	return copied, err
}
func (r *Repository) Delete(ctx context.Context, id int64, now time.Time) error {
	return r.Transaction(ctx, func(tx *Repository) error {
		task, err := tx.lockTask(ctx, id)
		if err != nil {
			return err
		}
		if task.Status != StatusDraft {
			return ErrNotDraft
		}
		if err = tx.db.WithContext(ctx).Model(&Target{}).Where("task_id=? AND deleted_at IS NULL", id).Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.db.WithContext(ctx).Model(&Task{}).Where("id=?", id).Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error
	})
}
func (r *Repository) find(ctx context.Context, id int64) (Task, error) {
	var row Task
	err := r.db.WithContext(ctx).Raw(`
SELECT task.*, notification.id AS notification_id
FROM message_notification_task task
LEFT JOIN message_notification notification ON notification.source_task_id=task.id
WHERE task.id=? AND task.deleted_at IS NULL`, id).Scan(&row).Error
	if err != nil {
		return Task{}, err
	}
	if row.ID == 0 {
		return Task{}, gorm.ErrRecordNotFound
	}
	if names, nameErr := r.permissions.PlatformNames(ctx, []int64{row.PlatformID}); nameErr != nil {
		return Task{}, nameErr
	} else {
		row.PlatformName = names[row.PlatformID]
	}
	row.TargetIDs, err = r.targetIDs(ctx, id)
	return row, err
}
func (r *Repository) lockTask(ctx context.Context, id int64) (Task, error) {
	var row Task
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND deleted_at IS NULL", id).Take(&row).Error
	return row, err
}
func (r *Repository) targetIDs(ctx context.Context, id int64) ([]int64, error) {
	var ids []int64
	err := r.db.WithContext(ctx).Model(&Target{}).Where("task_id=? AND deleted_at IS NULL", id).Order("target_id").Pluck("target_id", &ids).Error
	return ids, err
}
func (r *Repository) replaceTargets(ctx context.Context, id int64, audience AudienceType, ids []int64, now time.Time) error {
	if err := r.db.WithContext(ctx).Model(&Target{}).Where("task_id=? AND deleted_at IS NULL", id).Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error; err != nil {
		return err
	}
	if audience == AudiencePlatform {
		return nil
	}
	rows := make([]Target, 0, len(ids))
	for _, targetID := range ids {
		rows = append(rows, Target{TaskID: id, TargetType: audience, TargetID: targetID, CreatedAt: now, UpdatedAt: now})
	}
	return r.db.WithContext(ctx).Create(&rows).Error
}
func (r *Repository) validateDraftFacts(ctx context.Context, input DraftInput) error {
	if err := r.permissions.ValidatePlatformNotification(ctx, input.PlatformID); err != nil {
		return mapPermissionFactError(err)
	}
	if input.AudienceType == AudiencePlatform {
		return nil
	}
	at := time.Now().UTC()
	switch input.AudienceType {
	case AudienceUser:
		if err := r.permissions.ValidateUserTargets(ctx, input.PlatformID, input.TargetIDs, at); err != nil {
			return mapPermissionFactError(err)
		}
	case AudienceRole:
		if err := r.permissions.ValidateRoleTargets(ctx, input.PlatformID, input.TargetIDs, at); err != nil {
			return mapPermissionFactError(err)
		}
	}
	return nil
}

func mapPermissionFactError(err error) error {
	if errors.Is(err, permissionnotification.ErrInvalidFacts) {
		return fmt.Errorf("%w: %v", ErrInvalidFacts, err)
	}
	return err
}
func (r *Repository) Find(ctx context.Context, id int64) (Task, error) { return r.find(ctx, id) }
func (r *Repository) List(ctx context.Context, input ListQuery) ([]Task, int64, error) {
	query := r.db.WithContext(ctx).Table("message_notification_task task").Where("task.deleted_at IS NULL")
	if input.PlatformID != nil {
		query = query.Where("task.platform_id=?", *input.PlatformID)
	}
	if input.Status != 0 {
		query = query.Where("task.status=?", input.Status)
	}
	if input.AudienceType != "" {
		query = query.Where("task.audience_type=?", input.AudienceType)
	}
	if input.Keyword != "" {
		pattern := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(input.Keyword) + "%"
		query = query.Where(`task.title ILIKE ? ESCAPE '\'`, pattern)
	}
	if input.From != nil {
		query = query.Where("task.created_at>=?", *input.From)
	}
	if input.To != nil {
		query = query.Where("task.created_at<=?", *input.To)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []Task
	err := query.Select("task.*").Order("task.id DESC").Limit(input.PageSize).Offset((input.Page - 1) * input.PageSize).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.PlatformID)
	}
	names, err := r.permissions.PlatformNames(ctx, ids)
	for index := range rows {
		rows[index].PlatformName = names[rows[index].PlatformID]
	}
	return rows, total, err
}

type Option struct {
	ID    int64  `json:"id"`
	Label string `json:"label"`
}

func (r *Repository) Options(ctx context.Context, kind string, platformID int64, keyword string, after int64, limit int) ([]Option, error) {
	rows, err := r.permissions.Options(ctx, kind, platformID, keyword, after, limit)
	if err != nil {
		return nil, err
	}
	result := make([]Option, 0, len(rows))
	for _, row := range rows {
		result = append(result, Option{ID: row.ID, Label: row.Label})
	}
	return result, nil
}
