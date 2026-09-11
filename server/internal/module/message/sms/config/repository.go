package config

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) FindActive(ctx context.Context) (Model, error) {
	var value Model
	if err := r.db.WithContext(ctx).Order("id").First(&value).Error; err != nil {
		return Model{}, err
	}
	return value, nil
}

func (r *Repository) Create(ctx context.Context, value *Model) error {
	return r.db.WithContext(ctx).Create(&value).Error
}

func (r *Repository) Update(ctx context.Context, value *Model, now time.Time) error {
	return r.db.WithContext(ctx).Model(&Model{}).Where("id = ?", value.ID).Updates(map[string]any{
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
	}).Error
}

func (r *Repository) UpdateTestResult(ctx context.Context, id int64, at time.Time, message string) error {
	return r.db.WithContext(ctx).Model(&Model{}).Where("id = ?", id).Updates(map[string]any{
		"last_test_at":    at,
		"last_test_error": message,
		"updated_at":      at,
	}).Error
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&Model{}, id).Error
}
