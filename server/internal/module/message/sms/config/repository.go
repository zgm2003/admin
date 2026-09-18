package config

import (
	"context"
	"fmt"
	"time"

	cachegeneration "admin/server/internal/shared/cacheGeneration"
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

func (r *Repository) FindActive(ctx context.Context) (Model, error) {
	var value Model
	if err := r.db.WithContext(ctx).Where("deleted_at IS NULL").Order("id").First(&value).Error; err != nil {
		return Model{}, err
	}
	return value, nil
}

func (r *Repository) Create(ctx context.Context, value *Model, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	if value == nil {
		return cachegeneration.MutationResult{}, fmt.Errorf("sms config is required")
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

func (r *Repository) Update(ctx context.Context, value *Model, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	if value == nil || value.ID < 1 {
		return cachegeneration.MutationResult{}, fmt.Errorf("sms config is invalid")
	}
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		current, err := lockConfig(ctx, tx, value.ID)
		if err != nil {
			return false, err
		}
		if sameConfig(current, *value) {
			return false, nil
		}
		query := tx.WithContext(ctx).Model(&Model{}).Where("id = ? AND deleted_at IS NULL", value.ID).Updates(map[string]any{
			"secret_id_ciphertext":  value.SecretIDCiphertext,
			"secret_key_ciphertext": value.SecretKeyCiphertext,
			"secret_id_hint":        value.SecretIDHint,
			"secret_key_hint":       value.SecretKeyHint,
			"sms_sdk_app_id":        value.SDKAppID,
			"sign_name":             value.SignName,
			"region":                value.Region,
			"endpoint":              value.Endpoint,
			"ttl_minutes":           value.TTLMinutes,
			"is_enabled":            value.IsEnabled,
			"updated_at":            now,
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

func (r *Repository) UpdateTestResult(ctx context.Context, id int64, at time.Time, message string) error {
	query := r.db.WithContext(ctx).Model(&Model{}).Where("id = ? AND deleted_at IS NULL", id).Updates(map[string]any{
		"last_test_at": at, "last_test_error": message, "updated_at": at,
	})
	if query.Error != nil {
		return query.Error
	}
	if query.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id int64, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		if _, err := lockConfig(ctx, tx, id); err != nil {
			return false, err
		}
		query := tx.WithContext(ctx).Model(&Model{}).Where("id = ? AND deleted_at IS NULL", id).Updates(map[string]any{
			"deleted_at": now, "updated_at": now, "is_enabled": yesno.No,
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
		return cachegeneration.MutationResult{}, fmt.Errorf("sms config generation dependencies are not configured")
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

func lockConfig(ctx context.Context, tx *gorm.DB, id int64) (Model, error) {
	var value Model
	if err := tx.WithContext(ctx).Raw(`SELECT * FROM message_sms_config WHERE id = ? AND deleted_at IS NULL FOR UPDATE`, id).Scan(&value).Error; err != nil {
		return Model{}, err
	}
	if value.ID == 0 {
		return Model{}, gorm.ErrRecordNotFound
	}
	return value, nil
}

func sameConfig(left, right Model) bool {
	return left.SecretIDCiphertext == right.SecretIDCiphertext && left.SecretKeyCiphertext == right.SecretKeyCiphertext &&
		left.SecretIDHint == right.SecretIDHint && left.SecretKeyHint == right.SecretKeyHint &&
		left.SDKAppID == right.SDKAppID && left.SignName == right.SignName && left.Region == right.Region &&
		equalOptionalText(left.Endpoint, right.Endpoint) && left.TTLMinutes == right.TTLMinutes && left.IsEnabled == right.IsEnabled
}

func equalOptionalText(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
