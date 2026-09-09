package recipientrule

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context) ([]Model, error) {
	var values []Model
	err := r.db.WithContext(ctx).Where("deleted_at IS NULL").Order("id").Find(&values).Error
	return values, err
}

func (r *Repository) Create(ctx context.Context, value *Model) error {
	return r.db.WithContext(ctx).Create(value).Error
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

func (r *Repository) Delete(ctx context.Context, id int64) error {
	query := r.db.WithContext(ctx).Model(&Model{}).Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", time.Now().UTC())
	if query.Error != nil {
		return query.Error
	}
	if query.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) Lock(ctx context.Context, id int64) (Model, error) {
	var value Model
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND deleted_at IS NULL", id).Take(&value).Error
	return value, err
}
