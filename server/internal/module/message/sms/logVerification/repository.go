package logVerification

import (
	"context"

	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// Create appends one verification audit row.
func (r *Repository) Create(ctx context.Context, value *Model) error {
	return r.db.WithContext(ctx).Create(&value).Error
}

// FindBySMSLog returns the latest verification row of one log entry.
func (r *Repository) FindBySMSLog(ctx context.Context, platformID, smsLogID int64) (Model, error) {
	var value Model
	if err := r.db.WithContext(ctx).
		Where("platform_id = ? AND sms_log_id = ?", platformID, smsLogID).
		Order("id DESC").First(&value).Error; err != nil {
		return Model{}, err
	}
	return value, nil
}
