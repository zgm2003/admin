package notification

import "time"

type Notification struct {
	ID                int64        `gorm:"column:id;primaryKey"`
	PlatformID        int64        `gorm:"column:platform_id"`
	SourceTaskID      *int64       `gorm:"column:source_task_id"`
	SourceType        string       `gorm:"column:source_type"`
	SourceKey         string       `gorm:"column:source_key"`
	AudienceType      AudienceType `gorm:"column:audience_type"`
	AudienceMaxUserID int64        `gorm:"column:audience_max_user_id"`
	Title             string       `gorm:"column:title"`
	ContentHTML       string       `gorm:"column:content_html"`
	Summary           string       `gorm:"column:summary"`
	Variant           Variant      `gorm:"column:variant"`
	Priority          Priority     `gorm:"column:priority"`
	LinkType          LinkType     `gorm:"column:link_type"`
	Link              string       `gorm:"column:link"`
	PublishedAt       time.Time    `gorm:"column:published_at"`
	CreatedAt         time.Time    `gorm:"column:created_at"`
	UpdatedAt         time.Time    `gorm:"column:updated_at"`
}

func (Notification) TableName() string { return "message_notification" }

type Recipient struct {
	ID             int64      `gorm:"column:id;primaryKey"`
	NotificationID int64      `gorm:"column:notification_id"`
	PlatformID     int64      `gorm:"column:platform_id"`
	UserID         int64      `gorm:"column:user_id"`
	ReadAt         *time.Time `gorm:"column:read_at"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
	DeletedAt      *time.Time `gorm:"column:deleted_at"`
}

func (Recipient) TableName() string { return "message_notification_recipient" }

type BroadcastState struct {
	ID             int64      `gorm:"column:id;primaryKey"`
	NotificationID int64      `gorm:"column:notification_id"`
	PlatformID     int64      `gorm:"column:platform_id"`
	UserID         int64      `gorm:"column:user_id"`
	ReadAt         *time.Time `gorm:"column:read_at"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
	DeletedAt      *time.Time `gorm:"column:deleted_at"`
}

func (BroadcastState) TableName() string { return "message_notification_broadcast_state" }

type MailboxState struct {
	PlatformID                int64     `gorm:"column:platform_id;primaryKey"`
	UserID                    int64     `gorm:"column:user_id;primaryKey"`
	ReadThroughNotificationID int64     `gorm:"column:read_through_notification_id"`
	CreatedAt                 time.Time `gorm:"column:created_at"`
	UpdatedAt                 time.Time `gorm:"column:updated_at"`
}

func (MailboxState) TableName() string { return "message_notification_mailbox_state" }
