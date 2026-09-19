package realtime

import (
	"encoding/json"
	"time"
)

type Event struct {
	Sequence          int64           `gorm:"column:sequence;primaryKey"`
	EventID           string          `gorm:"column:event_id"`
	DedupKey          string          `gorm:"column:dedup_key"`
	PlatformID        int64           `gorm:"column:platform_id"`
	EventType         string          `gorm:"column:event_type"`
	TargetType        TargetType      `gorm:"column:target_type"`
	TargetUserID      *int64          `gorm:"column:target_user_id"`
	AudienceMaxUserID *int64          `gorm:"column:audience_max_user_id"`
	Payload           json.RawMessage `gorm:"column:payload;type:jsonb"`
	OccurredAt        time.Time       `gorm:"column:occurred_at"`
	CreatedAt         time.Time       `gorm:"column:created_at"`
	UpdatedAt         time.Time       `gorm:"column:updated_at"`
}

func (Event) TableName() string { return "realtime_event" }

type EventOutbox struct {
	ID            int64      `gorm:"column:id;primaryKey"`
	EventSequence int64      `gorm:"column:event_sequence"`
	Attempts      int        `gorm:"column:attempts"`
	AvailableAt   time.Time  `gorm:"column:available_at"`
	LockedUntil   *time.Time `gorm:"column:locked_until"`
	LockToken     *string    `gorm:"column:lock_token"`
	PublishedAt   *time.Time `gorm:"column:published_at"`
	LastError     *string    `gorm:"column:last_error"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at"`
}

func (EventOutbox) TableName() string { return "realtime_event_outbox" }

type RetentionState struct {
	PlatformID             int64     `gorm:"column:platform_id;primaryKey"`
	DeletedThroughSequence int64     `gorm:"column:deleted_through_sequence"`
	CreatedAt              time.Time `gorm:"column:created_at"`
	UpdatedAt              time.Time `gorm:"column:updated_at"`
}

func (RetentionState) TableName() string { return "realtime_retention_state" }

type ResumeWindow struct {
	Events                 []Event
	ThroughSequence        int64
	DeletedThroughSequence int64
	ResyncRequired         bool
	OverLimit              bool
}

type OutboxEvent struct {
	ID          int64
	Attempts    int
	AvailableAt time.Time
	LockedUntil time.Time
	LockToken   string
	Event       Event
}
