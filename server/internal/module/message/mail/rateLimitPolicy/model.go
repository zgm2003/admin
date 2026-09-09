package ratelimitpolicy

import "time"

const Table = "message_mail_rate_limit_policy"

type Model struct {
	PlatformID    int64     `gorm:"column:platform_id;primaryKey" json:"platformId"`
	Key           string    `gorm:"column:policy_key;primaryKey" json:"key"`
	Mode          string    `gorm:"column:mode;not null" json:"mode"`
	Dimension     string    `gorm:"column:dimension;not null" json:"dimension"`
	Limit         int       `gorm:"column:limit_count;not null" json:"limit"`
	WindowSeconds int       `gorm:"column:window_seconds;not null" json:"windowSeconds"`
	Revision      int64     `gorm:"column:revision;not null" json:"-"`
	CreatedAt     time.Time `gorm:"column:created_at;type:timestamptz;not null" json:"-"`
	UpdatedAt     time.Time `gorm:"column:updated_at;type:timestamptz;not null" json:"updatedAt"`
}

func (Model) TableName() string { return "message_mail_rate_limit_policy" }
