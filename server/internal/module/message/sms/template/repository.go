package template

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// List returns the four seeded rows in their seeded order, which matches the
// fixed catalog order used by the scene pickers.
func (r *Repository) List(ctx context.Context) ([]Model, error) {
	var values []Model
	if err := r.db.WithContext(ctx).Order("id").Find(&values).Error; err != nil {
		return nil, err
	}
	return values, nil
}

func (r *Repository) FindByID(ctx context.Context, id int64) (Model, error) {
	var value Model
	if err := r.db.WithContext(ctx).First(&value, id).Error; err != nil {
		return Model{}, err
	}
	return value, nil
}

func (r *Repository) Update(ctx context.Context, value *Model, now time.Time) error {
	return r.db.WithContext(ctx).Model(&Model{}).Where("id = ?", value.ID).Updates(map[string]any{
		"name":                value.Name,
		"tencent_template_id": value.TencentTemplateID,
		"parameter_keys":      value.ParameterKeys,
		"example_variables":   value.ExampleVariables,
		"updated_at":          now,
	}).Error
}

func (r *Repository) UpdateStatus(ctx context.Context, id int64, status int16, now time.Time) error {
	return r.db.WithContext(ctx).Model(&Model{}).Where("id = ?", id).Updates(map[string]any{
		"is_enabled": status,
		"updated_at": now,
	}).Error
}
