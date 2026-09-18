package template

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/yesno"

	"gorm.io/gorm"
)

type Repository struct {
	db          *gorm.DB
	generations *cachegeneration.Repository
	scope       cachegeneration.Scope
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) SetGenerations(repository *cachegeneration.Repository, scope cachegeneration.Scope) {
	r.generations = repository
	r.scope = scope
}

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

func (r *Repository) Update(ctx context.Context, id int64, expectedScene string, values map[string]any, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		row, err := lockTemplate(ctx, tx, id)
		if err != nil {
			return false, err
		}
		if row.Scene != expectedScene {
			return false, fmt.Errorf("template scene cannot be changed")
		}
		desired := row
		applyTemplateValues(&desired, values)
		if desired.IsEnabled == yesno.Yes {
			if desired.TencentTemplateID == nil || *desired.TencentTemplateID < 1 {
				return false, fmt.Errorf("enabled template requires a provider template id")
			}
			if err := validateTemplateFromModel(desired); err != nil {
				return false, err
			}
		}
		if sameTemplate(row, desired) {
			return false, nil
		}
		values["updated_at"] = now
		query := tx.WithContext(ctx).Model(&Model{}).Where("id = ?", id).Updates(values)
		if query.Error != nil {
			return false, query.Error
		}
		if query.RowsAffected != 1 {
			return false, gorm.ErrRecordNotFound
		}
		return true, nil
	})
}

func (r *Repository) UpdateStatus(ctx context.Context, id int64, enabled yesno.Value, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		row, err := lockTemplate(ctx, tx, id)
		if err != nil {
			return false, err
		}
		if enabled == yesno.Yes {
			if row.TencentTemplateID == nil || *row.TencentTemplateID < 1 {
				return false, fmt.Errorf("enabled template requires a provider template id")
			}
			if err := validateTemplateFromModel(row); err != nil {
				return false, err
			}
		}
		if row.IsEnabled == enabled {
			return false, nil
		}
		query := tx.WithContext(ctx).Model(&Model{}).Where("id = ?", id).Updates(map[string]any{"is_enabled": enabled, "updated_at": now})
		if query.Error != nil {
			return false, query.Error
		}
		if query.RowsAffected != 1 {
			return false, gorm.ErrRecordNotFound
		}
		return true, nil
	})
}

func (r *Repository) mutate(ctx context.Context, expected int64, now time.Time, apply func(*gorm.DB) (bool, error)) (cachegeneration.MutationResult, error) {
	if r == nil || r.db == nil || r.generations == nil {
		return cachegeneration.MutationResult{}, fmt.Errorf("mail template generation dependencies are not configured")
	}
	if err := r.scope.Validate(); err != nil {
		return cachegeneration.MutationResult{}, err
	}
	result := cachegeneration.MutationResult{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		changed, err := apply(tx)
		if err != nil {
			return fmt.Errorf("%w: %w", cachegeneration.ErrMutationRolledBack, err)
		}
		if !changed {
			return nil
		}
		event, err := r.generations.AdvanceTx(ctx, tx, r.scope, expected, now)
		if err != nil {
			return fmt.Errorf("%w: %w", cachegeneration.ErrMutationRolledBack, err)
		}
		result = cachegeneration.MutationResult{Changed: true, Generation: event.Generation, OutboxID: event.ID}
		return nil
	})
	if err != nil {
		return cachegeneration.MutationResult{}, err
	}
	return result, nil
}

func lockTemplate(ctx context.Context, tx *gorm.DB, id int64) (Model, error) {
	var row Model
	if err := tx.WithContext(ctx).Raw(`SELECT * FROM message_mail_template WHERE id = ? AND deleted_at IS NULL FOR UPDATE`, id).Scan(&row).Error; err != nil {
		return Model{}, err
	}
	if row.ID == 0 {
		return Model{}, gorm.ErrRecordNotFound
	}
	return row, nil
}

func applyTemplateValues(row *Model, values map[string]any) {
	for key, value := range values {
		switch key {
		case "name":
			row.Name = value.(string)
		case "subject":
			row.Subject = value.(string)
		case "content":
			row.Content = value.(string)
		case "tencent_template_id":
			if value == nil {
				row.TencentTemplateID = nil
			} else {
				row.TencentTemplateID = value.(*int)
			}
		case "variable_keys":
			row.VariableKeys = append([]byte(nil), value.([]byte)...)
		case "example_variables":
			row.ExampleVariables = append([]byte(nil), value.([]byte)...)
		}
	}
}

func sameTemplate(left, right Model) bool {
	return left.Name == right.Name && left.Subject == right.Subject && left.Content == right.Content &&
		equalTemplateID(left.TencentTemplateID, right.TencentTemplateID) &&
		bytes.Equal(left.VariableKeys, right.VariableKeys) && bytes.Equal(left.ExampleVariables, right.ExampleVariables)
}

func equalTemplateID(left, right *int) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
