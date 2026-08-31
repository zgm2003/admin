package userrole

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"time"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) FindByUser(ctx context.Context, userID int64) ([]UserRole, error) {
	rows := make([]UserRole, 0)
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("role_id, id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("find user roles: %w", err)
	}
	return rows, nil
}

func (r *Repository) Create(ctx context.Context, values []UserRole) error {
	if len(values) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(&values).Error; err != nil {
		return fmt.Errorf("create user roles: %w", err)
	}
	return nil
}

func (r *Repository) SoftDeleteIDs(ctx context.Context, ids []int64, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Model(&UserRole{}).Where("id IN ?", ids).Updates(map[string]any{"updated_at": now.UTC(), "deleted_at": now.UTC()}).Error; err != nil {
		return fmt.Errorf("delete user roles: %w", err)
	}
	return nil
}
