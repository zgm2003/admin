package template

import (
	"bytes"
	"context"
	"fmt"
	"time"

	configcache "admin/server/internal/shared/configCache"

	"gorm.io/gorm"
)

type Repository struct {
	db          *gorm.DB
	generations *configcache.Repository
	scope       configcache.Scope
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) SetGenerations(repository *configcache.Repository, scope configcache.Scope) {
	r.generations = repository
	r.scope = scope
}

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

func (r *Repository) Update(ctx context.Context, value *Model, expected int64, now time.Time) (configcache.MutationResult, error) {
	if value == nil || value.ID < 1 {
		return configcache.MutationResult{}, fmt.Errorf("sms template is invalid")
	}
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		current, err := lockTemplate(ctx, tx, value.ID)
		if err != nil {
			return false, err
		}
		if current.Scene != value.Scene {
			return false, fmt.Errorf("sms template scene cannot be changed")
		}
		if sameTemplate(current, *value) {
			return false, nil
		}
		query := tx.WithContext(ctx).Model(&Model{}).Where("id = ?", value.ID).Updates(map[string]any{
			"name": value.Name, "tencent_template_id": value.TencentTemplateID,
			"content": value.Content, "variable_keys": value.VariableKeys,
			"example_variables": value.ExampleVariables, "updated_at": now,
		})
		if query.Error != nil {
			return false, query.Error
		}
		if query.RowsAffected != 1 {
			return false, gorm.ErrRecordNotFound
		}
		return true, nil
	})
}

func (r *Repository) UpdateStatus(ctx context.Context, id int64, status int16, expected int64, now time.Time) (configcache.MutationResult, error) {
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		current, err := lockTemplate(ctx, tx, id)
		if err != nil {
			return false, err
		}
		if int16(current.IsEnabled) == status {
			return false, nil
		}
		query := tx.WithContext(ctx).Model(&Model{}).Where("id = ?", id).Updates(map[string]any{
			"is_enabled": status, "updated_at": now,
		})
		if query.Error != nil {
			return false, query.Error
		}
		if query.RowsAffected != 1 {
			return false, gorm.ErrRecordNotFound
		}
		return true, nil
	})
}

func (r *Repository) mutate(ctx context.Context, expected int64, now time.Time, apply func(*gorm.DB) (bool, error)) (configcache.MutationResult, error) {
	if r == nil || r.db == nil || r.generations == nil {
		return configcache.MutationResult{}, fmt.Errorf("sms template generation dependencies are not configured")
	}
	if err := r.scope.Validate(); err != nil {
		return configcache.MutationResult{}, err
	}
	result := configcache.MutationResult{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		changed, err := apply(tx)
		if err != nil {
			return fmt.Errorf("%w: %w", configcache.ErrMutationRolledBack, err)
		}
		if !changed {
			return nil
		}
		event, err := r.generations.AdvanceTx(ctx, tx, r.scope, expected, now)
		if err != nil {
			return fmt.Errorf("%w: %w", configcache.ErrMutationRolledBack, err)
		}
		result = configcache.MutationResult{Changed: true, Generation: event.Generation, OutboxID: event.ID}
		return nil
	})
	if err != nil {
		return configcache.MutationResult{}, err
	}
	return result, nil
}

func lockTemplate(ctx context.Context, tx *gorm.DB, id int64) (Model, error) {
	var value Model
	if err := tx.WithContext(ctx).Raw(`SELECT * FROM message_sms_template WHERE id = ? FOR UPDATE`, id).Scan(&value).Error; err != nil {
		return Model{}, err
	}
	if value.ID == 0 {
		return Model{}, gorm.ErrRecordNotFound
	}
	return value, nil
}

func sameTemplate(left, right Model) bool {
	return left.Name == right.Name && left.TencentTemplateID == right.TencentTemplateID && left.Content == right.Content &&
		bytes.Equal(left.VariableKeys, right.VariableKeys) && bytes.Equal(left.ExampleVariables, right.ExampleVariables)
}
