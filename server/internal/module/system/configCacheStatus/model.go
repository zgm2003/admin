package cachegeneration

import "time"

type generationModel struct {
	Namespace  string    `gorm:"column:namespace;type:varchar(128);not null"`
	ScopeKey   string    `gorm:"column:scope_key;type:varchar(128);not null"`
	Generation int64     `gorm:"column:generation;not null"`
	CreatedAt  time.Time `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt  time.Time `gorm:"column:updated_at;type:timestamptz;not null"`
}

func (generationModel) TableName() string { return "system_config_cache_generation" }

type outboxModel struct {
	ID          int64      `gorm:"column:id;primaryKey"`
	Namespace   string     `gorm:"column:namespace;type:varchar(128);not null"`
	ScopeKey    string     `gorm:"column:scope_key;type:varchar(128);not null"`
	Generation  int64      `gorm:"column:generation;not null"`
	Attempts    int        `gorm:"column:attempts;not null"`
	AvailableAt time.Time  `gorm:"column:available_at;type:timestamptz;not null"`
	LockedUntil *time.Time `gorm:"column:locked_until;type:timestamptz"`
	LockToken   *string    `gorm:"column:lock_token;type:varchar(64)"`
	LastError   string     `gorm:"column:last_error;type:varchar(512);not null"`
	PublishedAt *time.Time `gorm:"column:published_at;type:timestamptz"`
	CreatedAt   time.Time  `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;type:timestamptz;not null"`
}

func (outboxModel) TableName() string { return "system_config_cache_outbox" }
