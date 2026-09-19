package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"admin/server/internal/module/realtime"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type realtimeAppender interface {
	AppendTx(context.Context, *gorm.DB, realtime.EventInput) (realtime.Event, error)
}

type Repository struct {
	db       *gorm.DB
	realtime realtimeAppender
}

func NewRepository(db *gorm.DB, realtimeRepository realtimeAppender) *Repository {
	return &Repository{db: db, realtime: realtimeRepository}
}

func (r *Repository) CreateForUsers(ctx context.Context, input CreateForUsersInput) (Notification, error) {
	if r == nil || r.db == nil || r.realtime == nil {
		return Notification{}, errors.New("notification repository is not configured")
	}
	var result Notification
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		created, err := r.insertOrFindNotification(ctx, tx, input)
		if err != nil {
			return err
		}
		result = created.notification
		if !created.inserted {
			return nil
		}
		now := input.PublishedAt.UTC()
		recipients := make([]Recipient, 0, len(input.UserIDs))
		for _, userID := range input.UserIDs {
			recipients = append(recipients, Recipient{NotificationID: result.ID, PlatformID: input.PlatformID, UserID: userID, CreatedAt: now, UpdatedAt: now})
		}
		if err := tx.WithContext(ctx).Create(&recipients).Error; err != nil {
			return fmt.Errorf("create notification recipients: %w", err)
		}
		payload, err := json.Marshal(struct {
			NotificationID int64     `json:"notificationId"`
			Title          string    `json:"title"`
			Summary        string    `json:"summary"`
			Variant        Variant   `json:"variant"`
			Priority       Priority  `json:"priority"`
			LinkType       LinkType  `json:"linkType"`
			Link           string    `json:"link"`
			PublishedAt    time.Time `json:"publishedAt"`
		}{result.ID, result.Title, result.Summary, result.Variant, result.Priority, result.LinkType, result.Link, result.PublishedAt.UTC()})
		if err != nil {
			return err
		}
		for _, userID := range input.UserIDs {
			if _, err := r.realtime.AppendTx(ctx, tx, realtime.EventInput{
				EventID: uuid.NewString(), DedupKey: fmt.Sprintf("notification:%d:user:%d", result.ID, userID), PlatformID: input.PlatformID,
				EventType: realtime.EventNotificationCreated, TargetType: realtime.TargetUser, TargetUserID: &userID, Payload: payload, OccurredAt: result.PublishedAt,
			}); err != nil {
				return fmt.Errorf("append notification realtime event: %w", err)
			}
		}
		return nil
	})
	return result, err
}

type insertedNotification struct {
	notification Notification
	inserted     bool
}

func (r *Repository) insertOrFindNotification(ctx context.Context, tx *gorm.DB, input CreateForUsersInput) (insertedNotification, error) {
	now := input.PublishedAt.UTC()
	row := Notification{PlatformID: input.PlatformID, SourceTaskID: input.SourceTaskID, SourceType: input.SourceType, SourceKey: input.SourceKey, AudienceType: AudienceTargeted, Title: input.Title, ContentHTML: input.ContentHTML, Summary: input.Summary, Variant: input.Variant, Priority: input.Priority, LinkType: input.LinkType, Link: input.Link, PublishedAt: now, CreatedAt: now, UpdatedAt: now}
	result := tx.WithContext(ctx).Exec(`INSERT INTO message_notification(platform_id,source_task_id,source_type,source_key,audience_type,audience_max_user_id,title,content_html,summary,variant,priority,link_type,link,published_at,created_at,updated_at) VALUES(?,?,?,?,?,0,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(platform_id,source_type,source_key) DO NOTHING`, row.PlatformID, row.SourceTaskID, row.SourceType, row.SourceKey, row.AudienceType, row.Title, row.ContentHTML, row.Summary, row.Variant, row.Priority, row.LinkType, row.Link, row.PublishedAt, row.CreatedAt, row.UpdatedAt)
	if result.Error != nil {
		return insertedNotification{}, fmt.Errorf("create notification: %w", result.Error)
	}
	inserted := result.RowsAffected == 1
	if err := tx.WithContext(ctx).Where("platform_id=? AND source_type=? AND source_key=?", input.PlatformID, input.SourceType, input.SourceKey).Take(&row).Error; err != nil {
		return insertedNotification{}, fmt.Errorf("find notification source: %w", err)
	}
	return insertedNotification{notification: row, inserted: inserted}, nil
}

func (r *Repository) ListMailbox(ctx context.Context, query MailboxQuery, cutoff time.Time) (MailboxPage, error) {
	filterUnread := query.Filter == MailboxUnread
	var rows []struct {
		Notification
		IsRead bool `gorm:"column:is_read"`
	}
	if err := r.db.WithContext(ctx).Raw(mailboxVisibleSQL+`
SELECT id,platform_id,source_task_id,source_type,source_key,audience_type,audience_max_user_id,title,content_html,summary,variant,priority,link_type,link,published_at,created_at,updated_at,is_read
FROM visible
WHERE (?=0 OR id<?) AND (?=FALSE OR is_read=FALSE) AND (?='' OR variant=?) AND (?='' OR priority=?)
ORDER BY id DESC LIMIT ?`, query.PlatformID, query.UserID, query.PlatformID, query.UserID, cutoff, query.UserID, query.PlatformID, query.UserID, cutoff,
		query.BeforeID, query.BeforeID, filterUnread, string(query.Variant), query.Variant, string(query.Priority), query.Priority, query.Limit+1).Scan(&rows).Error; err != nil {
		return MailboxPage{}, fmt.Errorf("list notification mailbox: %w", err)
	}
	page := MailboxPage{Items: make([]MailboxItem, 0, min(len(rows), query.Limit))}
	for index, row := range rows {
		if index == query.Limit {
			next := page.Items[len(page.Items)-1].ID
			page.NextBeforeID = &next
			break
		}
		page.Items = append(page.Items, MailboxItem{Notification: row.Notification, IsRead: row.IsRead})
	}
	return page, nil
}

func (r *Repository) SummaryMailbox(ctx context.Context, platformID, userID int64, cutoff time.Time) (MailboxSummary, error) {
	var rows []struct {
		ID          int64     `gorm:"column:id"`
		Title       string    `gorm:"column:title"`
		Summary     string    `gorm:"column:summary"`
		Variant     Variant   `gorm:"column:variant"`
		Priority    Priority  `gorm:"column:priority"`
		LinkType    LinkType  `gorm:"column:link_type"`
		Link        string    `gorm:"column:link"`
		PublishedAt time.Time `gorm:"column:published_at"`
		IsRead      bool      `gorm:"column:is_read"`
		UnreadCount int64     `gorm:"column:unread_count"`
	}
	if err := r.db.WithContext(ctx).Raw(mailboxVisibleSQL+`, ranked AS (
 SELECT id,title,summary,variant,priority,link_type,link,published_at,is_read,
 count(*) FILTER (WHERE is_read=FALSE) OVER () AS unread_count,
 row_number() OVER (ORDER BY id DESC) AS row_number FROM visible
)
SELECT id,title,summary,variant,priority,link_type,link,published_at,is_read,unread_count FROM ranked WHERE row_number<=5 ORDER BY id DESC`,
		platformID, userID, platformID, userID, cutoff, userID, platformID, userID, cutoff).Scan(&rows).Error; err != nil {
		return MailboxSummary{}, fmt.Errorf("summarize notification mailbox: %w", err)
	}
	result := MailboxSummary{Recent: make([]SummaryItem, 0, len(rows))}
	for _, row := range rows {
		result.UnreadCount = row.UnreadCount
		result.Recent = append(result.Recent, SummaryItem{ID: row.ID, Title: row.Title, Summary: row.Summary, Variant: row.Variant, Priority: row.Priority, LinkType: row.LinkType, Link: row.Link, PublishedAt: row.PublishedAt, IsRead: row.IsRead})
	}
	return result, nil
}

func (r *Repository) ReadNotification(ctx context.Context, platformID, userID, notificationID int64, now time.Time) (bool, error) {
	return r.mutateNotification(ctx, platformID, userID, notificationID, now, "read")
}

func (r *Repository) DeleteNotification(ctx context.Context, platformID, userID, notificationID int64, now time.Time) (bool, error) {
	return r.mutateNotification(ctx, platformID, userID, notificationID, now, "delete")
}

func (r *Repository) mutateNotification(ctx context.Context, platformID, userID, notificationID int64, now time.Time, operation string) (bool, error) {
	changed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var audience AudienceType
		if err := tx.Raw(`SELECT audience_type FROM message_notification WHERE id=? AND platform_id=? AND published_at<=? AND (audience_type='platform' AND audience_max_user_id>=? OR audience_type='targeted' AND EXISTS(SELECT 1 FROM message_notification_recipient WHERE notification_id=message_notification.id AND platform_id=? AND user_id=?))`, notificationID, platformID, now, userID, platformID, userID).Scan(&audience).Error; err != nil {
			return err
		}
		if audience == "" {
			return gorm.ErrRecordNotFound
		}
		var result *gorm.DB
		if audience == AudienceTargeted {
			if operation == "read" {
				result = tx.Exec(`UPDATE message_notification_recipient SET read_at=?,updated_at=? WHERE notification_id=? AND platform_id=? AND user_id=? AND deleted_at IS NULL AND read_at IS NULL`, now, now, notificationID, platformID, userID)
			} else {
				result = tx.Exec(`UPDATE message_notification_recipient SET deleted_at=?,updated_at=? WHERE notification_id=? AND platform_id=? AND user_id=? AND deleted_at IS NULL`, now, now, notificationID, platformID, userID)
			}
		} else {
			if err := tx.Exec(`INSERT INTO message_notification_broadcast_state(notification_id,platform_id,user_id,created_at,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(notification_id,user_id) DO NOTHING`, notificationID, platformID, userID, now, now).Error; err != nil {
				return err
			}
			if operation == "read" {
				result = tx.Exec(`UPDATE message_notification_broadcast_state SET read_at=?,updated_at=? WHERE notification_id=? AND platform_id=? AND user_id=? AND deleted_at IS NULL AND read_at IS NULL`, now, now, notificationID, platformID, userID)
			} else {
				result = tx.Exec(`UPDATE message_notification_broadcast_state SET deleted_at=?,updated_at=? WHERE notification_id=? AND platform_id=? AND user_id=? AND deleted_at IS NULL`, now, now, notificationID, platformID, userID)
			}
		}
		if result.Error != nil {
			return result.Error
		}
		changed = result.RowsAffected == 1
		if !changed {
			return nil
		}
		return r.appendStateEvent(ctx, tx, platformID, userID, fmt.Sprintf("notification-state:%s:%d:%d:%d", operation, platformID, userID, notificationID), operation, &notificationID, nil, now)
	})
	return changed, err
}

func (r *Repository) ReadAllNotifications(ctx context.Context, platformID, userID int64, cutoff, now time.Time) (bool, error) {
	changed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var maximum int64
		if err := tx.Raw(mailboxVisibleSQL+` SELECT COALESCE(max(id),0) FROM visible`, platformID, userID, platformID, userID, cutoff, userID, platformID, userID, cutoff).Scan(&maximum).Error; err != nil {
			return err
		}
		if maximum == 0 {
			return nil
		}
		result := tx.Exec(`INSERT INTO message_notification_mailbox_state(platform_id,user_id,read_through_notification_id,created_at,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(platform_id,user_id) DO UPDATE SET read_through_notification_id=EXCLUDED.read_through_notification_id,updated_at=EXCLUDED.updated_at WHERE message_notification_mailbox_state.read_through_notification_id<EXCLUDED.read_through_notification_id`, platformID, userID, maximum, now, now)
		if result.Error != nil {
			return result.Error
		}
		changed = result.RowsAffected == 1
		if !changed {
			return nil
		}
		return r.appendStateEvent(ctx, tx, platformID, userID, fmt.Sprintf("notification-state:read-all:%d:%d:%d", platformID, userID, maximum), "readAll", nil, &maximum, now)
	})
	return changed, err
}

func (r *Repository) appendStateEvent(ctx context.Context, tx *gorm.DB, platformID, userID int64, dedup, kind string, notificationID, readThroughNotificationID *int64, now time.Time) error {
	payload, err := json.Marshal(struct {
		Kind                      string `json:"kind"`
		NotificationID            *int64 `json:"notificationId"`
		ReadThroughNotificationID *int64 `json:"readThroughNotificationId"`
	}{kind, notificationID, readThroughNotificationID})
	if err != nil {
		return err
	}
	_, err = r.realtime.AppendTx(ctx, tx, realtime.EventInput{EventID: uuid.NewString(), DedupKey: dedup, PlatformID: platformID, EventType: realtime.EventNotificationStateChanged, TargetType: realtime.TargetUser, TargetUserID: &userID, Payload: payload, OccurredAt: now})
	return err
}

const mailboxVisibleSQL = `WITH mailbox AS (
 SELECT COALESCE((SELECT read_through_notification_id FROM message_notification_mailbox_state WHERE platform_id=? AND user_id=?),0) AS read_through
), visible AS (
 SELECT notification.*, (recipient.read_at IS NOT NULL OR notification.id<=mailbox.read_through) AS is_read
 FROM message_notification notification
 JOIN message_notification_recipient recipient ON recipient.notification_id=notification.id AND recipient.platform_id=notification.platform_id
 CROSS JOIN mailbox
 WHERE notification.platform_id=? AND recipient.user_id=? AND recipient.deleted_at IS NULL AND notification.published_at>=?
 UNION ALL
 SELECT notification.*, (state.read_at IS NOT NULL OR notification.id<=mailbox.read_through) AS is_read
 FROM message_notification notification
 CROSS JOIN mailbox
 LEFT JOIN message_notification_broadcast_state state ON state.notification_id=notification.id AND state.platform_id=notification.platform_id AND state.user_id=?
 WHERE notification.platform_id=? AND notification.audience_type='platform' AND notification.audience_max_user_id>=? AND notification.published_at>=? AND state.deleted_at IS NULL
)`

func (r *Repository) CleanupNotifications(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	if cutoff.IsZero() || limit < 1 || limit > 1000 {
		return 0, errors.New("invalid notification retention cleanup")
	}
	var deleted int
	if err := r.db.WithContext(ctx).Raw(`WITH candidates AS (SELECT id FROM message_notification WHERE published_at<? ORDER BY published_at,id LIMIT ? FOR UPDATE SKIP LOCKED), deleted AS (DELETE FROM message_notification notification USING candidates WHERE notification.id=candidates.id RETURNING notification.id) SELECT count(*) FROM deleted`, cutoff, limit).Scan(&deleted).Error; err != nil {
		return 0, fmt.Errorf("cleanup notifications: %w", err)
	}
	return deleted, nil
}
