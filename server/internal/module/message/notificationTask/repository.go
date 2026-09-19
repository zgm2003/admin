package notificationtask

import (
	"admin/server/internal/module/message/notification"
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }
func (r *Repository) Transaction(ctx context.Context, fn func(*Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return fn(NewRepository(tx)) })
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
		return tx.replaceTargets(ctx, task.ID, input.AudienceType, input.TargetIDs, now)
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
		result := tx.db.WithContext(ctx).Model(&Task{}).Where("id=? AND status='draft' AND deleted_at IS NULL", id).Updates(map[string]any{"content_html": normalized.ContentHTML, "summary": summary, "status": status, "audience_max_user_id": maximum, "submitted_at": now, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrNotDraft
		}
		outbox := DispatchOutbox{TaskID: id, BatchNo: 0, AvailableAt: available, CreatedAt: now, UpdatedAt: now}
		if err = tx.db.WithContext(ctx).Create(&outbox).Error; err != nil {
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
		if err = tx.db.WithContext(ctx).Where("task_id=? AND published_at IS NULL", id).Delete(&DispatchOutbox{}).Error; err != nil {
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
		return tx.replaceTargets(ctx, copied.ID, input.AudienceType, input.TargetIDs, now)
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
	err := r.db.WithContext(ctx).Raw(`SELECT task.*,notification.id AS notification_id FROM message_notification_task task LEFT JOIN message_notification notification ON notification.source_task_id=task.id WHERE task.id=? AND task.deleted_at IS NULL`, id).Scan(&row).Error
	if err != nil {
		return Task{}, err
	}
	if row.ID == 0 {
		return Task{}, gorm.ErrRecordNotFound
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
	if err := r.validatePlatformCapability(ctx, input.PlatformID); err != nil {
		return err
	}
	if input.AudienceType == AudiencePlatform {
		return nil
	}
	table := "user_account"
	if input.AudienceType == AudienceRole {
		table = "permission_role"
	}
	var count int64
	if err := r.db.WithContext(ctx).Table(table).Where("id IN ? AND is_enabled=1 AND deleted_at IS NULL", input.TargetIDs).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(input.TargetIDs)) {
		return fmt.Errorf("%w: target is unavailable", ErrInvalidFacts)
	}
	return nil
}
func (r *Repository) validatePlatformCapability(ctx context.Context, platformID int64) error {
	var enabled int64
	if err := r.db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_auth_platform WHERE id=? AND is_enabled=1 AND deleted_at IS NULL`, platformID).Scan(&enabled).Error; err != nil {
		return err
	}
	if enabled != 1 {
		return fmt.Errorf("%w: platform is unavailable", ErrInvalidFacts)
	}
	type node struct {
		ID       int64
		ParentID *int64
		MenuType string
		Code     string
		IsHidden int16
	}
	var nodes []node
	if err := r.db.WithContext(ctx).Raw(`SELECT id,parent_id,menu_type,code,is_hidden FROM permission_menu WHERE platform_id=? AND code IN ('message:notification:view','message:notification:list','message:notification:read','message:notification:delete') AND is_enabled=1 AND deleted_at IS NULL`, platformID).Scan(&nodes).Error; err != nil {
		return err
	}
	if len(nodes) == 0 {
		return fmt.Errorf("%w: notification capability is not enabled for platform", ErrInvalidFacts)
	}
	if len(nodes) != 4 {
		return errors.New("notification capability facts are incomplete")
	}
	var pageID int64
	actions := map[string]bool{}
	for _, node := range nodes {
		if node.Code == "message:notification:view" {
			if node.MenuType != "page" || node.IsHidden != 1 || node.ParentID != nil {
				return errors.New("notification page fact is invalid")
			}
			pageID = node.ID
		} else {
			actions[node.Code] = node.MenuType == "action" && node.ParentID != nil
		}
	}
	if pageID == 0 {
		return errors.New("notification page fact is missing")
	}
	for _, node := range nodes {
		if node.Code != "message:notification:view" && (!actions[node.Code] || node.ParentID == nil || *node.ParentID != pageID) {
			return errors.New("notification action fact is invalid")
		}
	}
	return nil
}
func (r *Repository) Find(ctx context.Context, id int64) (Task, error) { return r.find(ctx, id) }
func (r *Repository) List(ctx context.Context, status Status, page, size int) ([]Task, int64, error) {
	query := r.db.WithContext(ctx).Model(&Task{}).Where("deleted_at IS NULL")
	if status != "" {
		query = query.Where("status=?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []Task
	err := query.Order("id DESC").Limit(size).Offset((page - 1) * size).Find(&rows).Error
	return rows, total, err
}

type Option struct {
	ID    int64  `json:"id"`
	Label string `json:"label"`
}

func (r *Repository) Options(ctx context.Context, kind, keyword string, after int64, limit int) ([]Option, error) {
	table, name := "user_account", "username"
	condition := "is_enabled=1 AND deleted_at IS NULL"
	if kind == "role" {
		table, name = "permission_role", "name"
	}
	var rows []Option
	if kind == "platform" {
		pattern := "%" + strings.NewReplacer("%", "\\%", "_", "\\_").Replace(keyword) + "%"
		var malformed int64
		if err := r.db.WithContext(ctx).Raw(`
SELECT count(*)
FROM permission_auth_platform platform
WHERE platform.id>?
  AND platform.is_enabled=1
  AND platform.deleted_at IS NULL
  AND (?='' OR platform.name ILIKE ? ESCAPE '\\')
  AND EXISTS (
    SELECT 1 FROM permission_menu node
    WHERE node.platform_id=platform.id
      AND node.code IN ('message:notification:view','message:notification:list','message:notification:read','message:notification:delete')
      AND node.is_enabled=1
      AND node.deleted_at IS NULL
  )
  AND NOT EXISTS (
    SELECT 1
    FROM permission_menu page
    WHERE page.platform_id=platform.id
      AND page.code='message:notification:view'
      AND page.menu_type='page'
      AND page.parent_id IS NULL
      AND page.is_hidden=1
      AND page.is_enabled=1
      AND page.deleted_at IS NULL
      AND (
        SELECT count(DISTINCT action.code)
        FROM permission_menu action
        WHERE action.platform_id=platform.id
          AND action.parent_id=page.id
          AND action.code IN ('message:notification:list','message:notification:read','message:notification:delete')
          AND action.menu_type='action'
          AND action.is_enabled=1
          AND action.deleted_at IS NULL
      )=3
  )`, after, keyword, pattern).Scan(&malformed).Error; err != nil {
			return nil, err
		}
		if malformed != 0 {
			return nil, errors.New("notification platform capability facts are incomplete")
		}
		err := r.db.WithContext(ctx).Raw(`
SELECT platform.id, platform.name AS label
FROM permission_auth_platform platform
JOIN permission_menu page
  ON page.platform_id=platform.id
 AND page.code='message:notification:view'
 AND page.menu_type='page'
 AND page.parent_id IS NULL
 AND page.is_hidden=1
 AND page.is_enabled=1
 AND page.deleted_at IS NULL
JOIN permission_menu action
  ON action.platform_id=platform.id
 AND action.parent_id=page.id
 AND action.code IN ('message:notification:list','message:notification:read','message:notification:delete')
 AND action.menu_type='action'
 AND action.is_enabled=1
 AND action.deleted_at IS NULL
WHERE platform.id>?
  AND platform.is_enabled=1
  AND platform.deleted_at IS NULL
  AND (?='' OR platform.name ILIKE ? ESCAPE '\\')
GROUP BY platform.id,platform.name
HAVING count(DISTINCT action.code)=3
ORDER BY platform.id
LIMIT ?`, after, keyword, pattern, limit).Scan(&rows).Error
		return rows, err
	}
	query := r.db.WithContext(ctx).Table(table).Select("id,"+name+" AS label").Where("id>? AND "+condition, after)
	if keyword != "" {
		query = query.Where(name+" ILIKE ?", "%"+strings.NewReplacer("%", "\\%", "_", "\\_").Replace(keyword)+"%")
	}
	if err := query.Order("id").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
