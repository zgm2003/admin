package taskdemo

import "time"

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

type Task struct {
	ID        string    `gorm:"type:varchar(32);primaryKey"`
	Message   string    `gorm:"type:varchar(200);not null"`
	Status    string    `gorm:"type:varchar(16);not null;check:status IN ('pending','running','completed','failed')"`
	CreatedAt time.Time `gorm:"type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
}

func (Task) TableName() string {
	return "foundation_task"
}
