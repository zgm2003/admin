package config

import (
	"context"
	"errors"
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Find(ctx context.Context) (Model, error) {
	var value Model
	err := r.db.WithContext(ctx).Where("deleted_at IS NULL").Take(&value).Error
	return value, err
}

func (r *Repository) Save(ctx context.Context, values map[string]any) (Model, error) {
	var row Model
	query := r.db.WithContext(ctx).Where("deleted_at IS NULL").First(&row)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		for key, value := range values {
			setField(&row, key, value)
		}
		if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
			return Model{}, err
		}
		return row, nil
	}
	if query.Error != nil {
		return Model{}, query.Error
	}
	if err := r.db.WithContext(ctx).Model(&row).Updates(values).Error; err != nil {
		return Model{}, err
	}
	return r.Find(ctx)
}

func (r *Repository) Delete(ctx context.Context) error {
	now := time.Now().UTC()
	query := r.db.WithContext(ctx).Model(&Model{}).Where("deleted_at IS NULL").Updates(map[string]any{
		"deleted_at": now, "updated_at": now, "is_enabled": yesno.No,
	})
	if query.Error != nil {
		return query.Error
	}
	if query.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
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
	if text, ok := value.(*string); ok {
		return text
	}
	if text, ok := value.(string); ok {
		return &text
	}
	return nil
}
