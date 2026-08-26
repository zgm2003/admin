package access

import "time"

type Version struct {
	UserID    int64     `gorm:"column:user_id;primaryKey"`
	Version   int64     `gorm:"column:version;not null;default:1"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
}

func (Version) TableName() string {
	return "rbac_access_version"
}
