package recipientrule

import (
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

func (r *Repository) Create(ctx context.Context, value *Model, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	if value == nil {
		return cachegeneration.MutationResult{}, fmt.Errorf("mail recipient rule is required")
	}
	originalID := value.ID
	result, err := r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		return true, tx.WithContext(ctx).Create(value).Error
	})
	if err != nil {
		value.ID = originalID
	}
	return result, err
}

func (r *Repository) Update(ctx context.Context, id int64, values map[string]any, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		row, err := lockRule(ctx, tx, id)
		if err != nil {
			return false, err
		}
		desired := row
		applyRuleValues(&desired, values)
		if sameRule(row, desired) {
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
		row, err := lockRule(ctx, tx, id)
		if err != nil {
			return false, err
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

func (r *Repository) Delete(ctx context.Context, id int64, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		if _, err := lockRule(ctx, tx, id); err != nil {
			return false, err
		}
		query := tx.WithContext(ctx).Model(&Model{}).Where("id = ?", id).Updates(map[string]any{"deleted_at": now, "updated_at": now})
		if query.Error != nil {
			return false, query.Error
		}
		if query.RowsAffected != 1 {
			return false, gorm.ErrRecordNotFound
		}
		return true, nil
	})
}

func (r *Repository) Lock(ctx context.Context, id int64) (Model, error) {
	return lockRule(ctx, r.db, id)
}

func (r *Repository) mutate(ctx context.Context, expected int64, now time.Time, apply func(*gorm.DB) (bool, error)) (cachegeneration.MutationResult, error) {
	if r == nil || r.db == nil || r.generations == nil {
		return cachegeneration.MutationResult{}, fmt.Errorf("mail recipient rule generation dependencies are not configured")
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

func lockRule(ctx context.Context, tx *gorm.DB, id int64) (Model, error) {
	var value Model
	if err := tx.WithContext(ctx).Raw(`SELECT * FROM message_mail_recipient_rule WHERE id = ? AND deleted_at IS NULL FOR UPDATE`, id).Scan(&value).Error; err != nil {
		return Model{}, err
	}
	if value.ID == 0 {
		return Model{}, gorm.ErrRecordNotFound
	}
	return value, nil
}

func applyRuleValues(row *Model, values map[string]any) {
	for key, value := range values {
		switch key {
		case "scope":
			row.Scope = value.(string)
		case "pattern":
			row.Pattern = value.(string)
		case "action":
			row.Action = value.(string)
		case "name":
			row.Name = value.(string)
		case "remark":
			row.Remark = value.(string)
		case "is_enabled":
			row.IsEnabled = value.(yesno.Value)
		}
	}
}

func sameRule(left, right Model) bool {
	return left.Scope == right.Scope && left.Pattern == right.Pattern && left.Action == right.Action &&
		left.Name == right.Name && left.Remark == right.Remark && left.IsEnabled == right.IsEnabled
}
