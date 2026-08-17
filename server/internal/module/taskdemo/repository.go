package taskdemo

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, task *Task) error {
	if err := r.db.WithContext(ctx).Create(task).Error; err != nil {
		return fmt.Errorf("create foundation task: %w", err)
	}
	return nil
}

func (r *Repository) Find(ctx context.Context, taskID string) (Task, error) {
	var task Task
	if err := r.db.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return Task{}, fmt.Errorf("find foundation task: %w", err)
	}
	return task, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, taskID, status string) error {
	result := r.db.WithContext(ctx).Model(&Task{}).Where("id = ?", taskID).Update("status", status)
	if result.Error != nil {
		return fmt.Errorf("update foundation task status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("update foundation task status: %w", gorm.ErrRecordNotFound)
	}
	return nil
}
