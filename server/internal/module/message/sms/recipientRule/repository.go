package recipientRule

import (
	"context"
	"errors"
	"fmt"
	"time"

	cachegeneration "admin/server/internal/shared/cacheGeneration"

	"github.com/jackc/pgx/v5/pgconn"
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
	if err := r.db.WithContext(ctx).Where("deleted_at IS NULL").Order("id").Find(&values).Error; err != nil {
		return nil, err
	}
	return values, nil
}

func (r *Repository) FindByID(ctx context.Context, id int64) (Model, error) {
	var value Model
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&value).Error; err != nil {
		return Model{}, err
	}
	return value, nil
}

func (r *Repository) Create(ctx context.Context, value *Model, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	if value == nil {
		return cachegeneration.MutationResult{}, fmt.Errorf("sms recipient rule is required")
	}
	originalID := value.ID
	result, err := r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		return true, mapWriteError(tx.WithContext(ctx).Create(value).Error)
	})
	if err != nil {
		value.ID = originalID
	}
	return result, err
}

func (r *Repository) Update(ctx context.Context, value *Model, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	if value == nil || value.ID < 1 {
		return cachegeneration.MutationResult{}, fmt.Errorf("sms recipient rule is invalid")
	}
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		current, err := lockRule(ctx, tx, value.ID)
		if err != nil {
			return false, err
		}
		if sameRule(current, *value) {
			return false, nil
		}
		query := tx.WithContext(ctx).Model(&Model{}).Where("id = ? AND deleted_at IS NULL", value.ID).Updates(map[string]any{
			"scope": value.Scope, "pattern_ciphertext": value.PatternCiphertext,
			"pattern_hint": value.PatternHint, "pattern_hmac": value.PatternHMAC,
			"action": value.Action, "name": value.Name, "remark": value.Remark,
			"is_enabled": value.IsEnabled, "updated_at": now,
		})
		if query.Error != nil {
			return false, mapWriteError(query.Error)
		}
		if query.RowsAffected != 1 {
			return false, gorm.ErrRecordNotFound
		}
		return true, nil
	})
}

func (r *Repository) UpdateStatus(ctx context.Context, id int64, status int16, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		current, err := lockRule(ctx, tx, id)
		if err != nil {
			return false, err
		}
		if int16(current.IsEnabled) == status {
			return false, nil
		}
		query := tx.WithContext(ctx).Model(&Model{}).Where("id = ? AND deleted_at IS NULL", id).Updates(map[string]any{
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

func (r *Repository) Delete(ctx context.Context, id int64, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		if _, err := lockRule(ctx, tx, id); err != nil {
			return false, err
		}
		query := tx.WithContext(ctx).Model(&Model{}).Where("id = ? AND deleted_at IS NULL", id).Updates(map[string]any{
			"deleted_at": now, "updated_at": now,
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

func (r *Repository) mutate(ctx context.Context, expected int64, now time.Time, apply func(*gorm.DB) (bool, error)) (cachegeneration.MutationResult, error) {
	if r == nil || r.db == nil || r.generations == nil {
		return cachegeneration.MutationResult{}, fmt.Errorf("sms recipient rule generation dependencies are not configured")
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
	if err := tx.WithContext(ctx).Raw(`SELECT * FROM message_sms_recipient_rule WHERE id = ? AND deleted_at IS NULL FOR UPDATE`, id).Scan(&value).Error; err != nil {
		return Model{}, err
	}
	if value.ID == 0 {
		return Model{}, gorm.ErrRecordNotFound
	}
	return value, nil
}

func sameRule(left, right Model) bool {
	return left.Scope == right.Scope && left.PatternCiphertext == right.PatternCiphertext &&
		left.PatternHint == right.PatternHint && left.PatternHMAC == right.PatternHMAC &&
		left.Action == right.Action && left.Name == right.Name && left.Remark == right.Remark && left.IsEnabled == right.IsEnabled
}

func mapWriteError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return ErrConflict
	}
	return err
}
