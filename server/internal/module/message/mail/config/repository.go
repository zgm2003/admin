package config

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

func (r *Repository) Find(ctx context.Context) (Model, error) {
	var value Model
	err := r.db.WithContext(ctx).Where("deleted_at IS NULL").Take(&value).Error
	return value, err
}

func (r *Repository) Save(ctx context.Context, values map[string]any, expected int64, now time.Time) (Model, cachegeneration.MutationResult, error) {
	var saved Model
	result, err := r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		row, found, err := lockActiveConfig(ctx, tx)
		if err != nil {
			return false, err
		}
		if !found {
			for key, value := range values {
				setField(&row, key, value)
			}
			row.CreatedAt = now
			row.UpdatedAt = now
			if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
				return false, err
			}
			saved = row
			return true, nil
		}
		desired := row
		for key, value := range values {
			setField(&desired, key, value)
		}
		if sameConfig(row, desired) {
			saved = row
			return false, nil
		}
		values["updated_at"] = now
		query := tx.WithContext(ctx).Model(&Model{}).Where("id = ?", row.ID).Updates(values)
		if query.Error != nil {
			return false, query.Error
		}
		if query.RowsAffected != 1 {
			return false, gorm.ErrRecordNotFound
		}
		desired.UpdatedAt = now
		saved = desired
		return true, nil
	})
	if err != nil {
		return Model{}, cachegeneration.MutationResult{}, err
	}
	return saved, result, nil
}

func (r *Repository) Delete(ctx context.Context, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		row, found, err := lockActiveConfig(ctx, tx)
		if err != nil {
			return false, err
		}
		if !found {
			return false, gorm.ErrRecordNotFound
		}
		query := tx.WithContext(ctx).Model(&Model{}).Where("id = ?", row.ID).Updates(map[string]any{
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

func (r *Repository) RecordTestResult(ctx context.Context, at time.Time, summary string) error {
	query := r.db.WithContext(ctx).Model(&Model{}).Where("deleted_at IS NULL").Updates(map[string]any{
		"last_test_at": at, "last_test_error": summary, "updated_at": at,
	})
	if query.Error != nil {
		return query.Error
	}
	if query.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) mutate(ctx context.Context, expected int64, now time.Time, apply func(*gorm.DB) (bool, error)) (cachegeneration.MutationResult, error) {
	if r == nil || r.db == nil || r.generations == nil {
		return cachegeneration.MutationResult{}, fmt.Errorf("mail config generation dependencies are not configured")
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

func lockActiveConfig(ctx context.Context, tx *gorm.DB) (Model, bool, error) {
	var row Model
	err := tx.WithContext(ctx).Raw(`SELECT * FROM message_mail_config WHERE deleted_at IS NULL FOR UPDATE`).Scan(&row).Error
	if err != nil {
		return Model{}, false, err
	}
	return row, row.ID != 0, nil
}

func sameConfig(left, right Model) bool {
	return left.SecretIDCiphertext == right.SecretIDCiphertext &&
		left.SecretKeyCiphertext == right.SecretKeyCiphertext &&
		left.SecretIDHint == right.SecretIDHint && left.SecretKeyHint == right.SecretKeyHint &&
		left.Region == right.Region && equalText(left.Endpoint, right.Endpoint) &&
		left.FromEmail == right.FromEmail && left.FromName == right.FromName &&
		equalText(left.ReplyTo, right.ReplyTo) && left.TTLMinutes == right.TTLMinutes &&
		left.IsEnabled == right.IsEnabled
}

func equalText(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func setField(row *Model, key string, value any) {
	switch key {
	case "secret_id_ciphertext":
		row.SecretIDCiphertext = value.(string)
	case "secret_key_ciphertext":
		row.SecretKeyCiphertext = value.(string)
	case "secret_id_hint":
		row.SecretIDHint = value.(string)
	case "secret_key_hint":
		row.SecretKeyHint = value.(string)
	case "region":
		row.Region = value.(string)
	case "from_email":
		row.FromEmail = value.(string)
	case "from_name":
		row.FromName = value.(string)
	case "endpoint":
		row.Endpoint = textPointer(value)
	case "reply_to":
		row.ReplyTo = textPointer(value)
	case "ttl_minutes":
		row.TTLMinutes = int16(value.(int))
	case "is_enabled":
		switch enabled := value.(type) {
		case yesno.Value:
			row.IsEnabled = enabled
		case int16:
			row.IsEnabled = yesno.Value(enabled)
		case int:
			row.IsEnabled = yesno.Value(enabled)
		}
	}
}

func textPointer(value any) *string {
	if value == nil {
		return nil
	}
	if text, ok := value.(*string); ok {
		return text
	}
	if text, ok := value.(string); ok {
		return &text
	}
	return nil
}
