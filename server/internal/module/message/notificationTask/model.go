package notificationtask

import (
	"admin/server/internal/module/message/notification"
	"time"
)

type Task struct {
	ID                int64                 `gorm:"column:id;primaryKey"`
	PlatformID        int64                 `gorm:"column:platform_id"`
	PlatformName      string                `gorm:"column:platform_name;->"`
	Title             string                `gorm:"column:title"`
	ContentHTML       string                `gorm:"column:content_html"`
	Summary           string                `gorm:"column:summary"`
	Variant           notification.Variant  `gorm:"column:variant"`
	Priority          notification.Priority `gorm:"column:priority"`
	LinkType          notification.LinkType `gorm:"column:link_type"`
	Link              string                `gorm:"column:link"`
	AudienceType      AudienceType          `gorm:"column:audience_type"`
	ScheduledAt       *time.Time            `gorm:"column:scheduled_at"`
	AudienceMaxUserID *int64                `gorm:"column:audience_max_user_id"`
	SubmittedAt       *time.Time            `gorm:"column:submitted_at"`
	Status            Status                `gorm:"column:status"`
	NextUserID        int64                 `gorm:"column:next_user_id"`
	NextBatchNo       int                   `gorm:"column:next_batch_no"`
	GeneratedCount    int64                 `gorm:"column:generated_count"`
	FailureMessage    *string               `gorm:"column:failure_message"`
	PublishedAt       *time.Time            `gorm:"column:published_at"`
	CompletedAt       *time.Time            `gorm:"column:completed_at"`
	CanceledAt        *time.Time            `gorm:"column:canceled_at"`
	FailedAt          *time.Time            `gorm:"column:failed_at"`
	CreatedBy         int64                 `gorm:"column:created_by"`
	CreatedAt         time.Time             `gorm:"column:created_at"`
	UpdatedAt         time.Time             `gorm:"column:updated_at"`
	DeletedAt         *time.Time            `gorm:"column:deleted_at"`
	TargetIDs         []int64               `gorm:"-"`
	NotificationID    *int64                `gorm:"-"`
}

func (Task) TableName() string { return "message_notification_task" }

type Target struct {
	ID         int64        `gorm:"column:id;primaryKey"`
	TaskID     int64        `gorm:"column:task_id"`
	TargetType AudienceType `gorm:"column:target_type"`
	TargetID   int64        `gorm:"column:target_id"`
	CreatedAt  time.Time    `gorm:"column:created_at"`
	UpdatedAt  time.Time    `gorm:"column:updated_at"`
	DeletedAt  *time.Time   `gorm:"column:deleted_at"`
}

func (Target) TableName() string { return "message_notification_task_target" }

type DispatchOutbox struct {
	ID          int64      `gorm:"column:id;primaryKey"`
	TaskID      int64      `gorm:"column:task_id"`
	BatchNo     int        `gorm:"column:batch_no"`
	Attempts    int        `gorm:"column:attempts"`
	AvailableAt time.Time  `gorm:"column:available_at"`
	LockedUntil *time.Time `gorm:"column:locked_until"`
	LockToken   *string    `gorm:"column:lock_token"`
	PublishedAt *time.Time `gorm:"column:published_at"`
	LastError   *string    `gorm:"column:last_error"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
}

func (DispatchOutbox) TableName() string { return "message_notification_dispatch_outbox" }
