package recipientRule

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

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

func (r *Repository) Create(ctx context.Context, value *Model) error {
	if err := r.db.WithContext(ctx).Create(&value).Error; err != nil {
		return mapWriteError(err)
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, value *Model, now time.Time) error {
	if err := r.db.WithContext(ctx).Model(&Model{}).Where("id = ?", value.ID).Updates(map[string]any{
		"scope":              value.Scope,
		"pattern_ciphertext": value.PatternCiphertext,
		"pattern_hint":       value.PatternHint,
		"pattern_hmac":       value.PatternHMAC,
		"action":             value.Action,
		"name":               value.Name,
		"remark":             value.Remark,
		"is_enabled":         value.IsEnabled,
		"updated_at":         now,
	}).Error; err != nil {
		return mapWriteError(err)
	}
	return nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id int64, status int16, now time.Time) error {
	return r.db.WithContext(ctx).Model(&Model{}).Where("id = ?", id).Updates(map[string]any{
		"is_enabled": status,
		"updated_at": now,
	}).Error
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&Model{}, id).Error
}

func mapWriteError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return ErrConflict
	}
	return err
}
