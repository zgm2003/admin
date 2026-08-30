package profile

import "time"

type Profile struct {
	UserID    int64      `gorm:"column:user_id;primaryKey"`
	Avatar    string     `gorm:"column:avatar;type:varchar(512);not null;default:''"`
	Birthday  *time.Time `gorm:"column:birthday;type:date"`
	Gender    int16      `gorm:"column:gender;type:smallint;not null;default:0"`
	CreatedAt time.Time  `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time  `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
}

func (Profile) TableName() string { return "user_profile" }
