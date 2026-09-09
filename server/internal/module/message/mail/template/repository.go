package template

import (
	"context"

	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context) ([]Model, error) {
	var values []Model
	err := r.db.WithContext(ctx).Where("deleted_at IS NULL").Order("id").Find(&values).Error
	return values, err
}

func (r *Repository) Find(ctx context.Context, id int64) (Model, error) {
	var value Model
	err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).Take(&value).Error
	return value, err
}

func (r *Repository) FindByScene(ctx context.Context, scene string) (Model, error) {
	var value Model
	err := r.db.WithContext(ctx).Where("scene = ? AND deleted_at IS NULL", scene).Take(&value).Error
	return value, err
}

func (r *Repository) Update(ctx context.Context, id int64, values map[string]any) error {
	query := r.db.WithContext(ctx).Model(&Model{}).Where("id = ? AND deleted_at IS NULL", id).Updates(values)
	if query.Error != nil {
		return query.Error
	}
	if query.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
