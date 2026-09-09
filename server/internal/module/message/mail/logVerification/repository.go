package logverification

import (
	"context"

	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Create(ctx context.Context, value *Model) error {
	return r.db.WithContext(ctx).Create(value).Error
}

func (r *Repository) FindByLog(ctx context.Context, platformID, logID int64) (Model, error) {
	var value Model
	err := r.db.WithContext(ctx).
		Where("platform_id = ? AND mail_log_id = ? AND deleted_at IS NULL", platformID, logID).
		Take(&value).Error
	return value, err
}
