package tool

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context) ([]Tool, error) {
	var rows []Tool
	err := r.db.WithContext(ctx).Where("deleted_at IS NULL").Order("sort_order, id").Find(&rows).Error
	return rows, err
}

func (r *Repository) Enabled(ctx context.Context) ([]Tool, error) {
	var rows []Tool
	err := r.db.WithContext(ctx).
		Where("status = ? AND deleted_at IS NULL", StatusEnabled).
		Order("sort_order, id").Find(&rows).Error
	return rows, err
}

func (r *Repository) FindByCode(ctx context.Context, code string) (Tool, error) {
	var row Tool
	err := r.db.WithContext(ctx).Where("code = ? AND deleted_at IS NULL", code).First(&row).Error
	return row, err
}

func (r *Repository) UpdateStatus(ctx context.Context, code string, status Status, now time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Model(&Tool{}).
		Where("code = ? AND deleted_at IS NULL", code).
		Updates(map[string]any{"status": status, "updated_at": now})
	return result.RowsAffected, result.Error
}
