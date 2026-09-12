package setting

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context, query ListQuery) ([]Record, int64, error) {
	db := r.db.WithContext(ctx).Model(&Model{})
	if query.Keyword != "" {
		pattern := "%" + strings.ReplaceAll(strings.ReplaceAll(query.Keyword, "%", `\%`), "_", `\_`) + "%"
		db = db.Where("(setting_key LIKE ? ESCAPE '\\' OR description LIKE ? ESCAPE '\\')", pattern, pattern)
	}
	if query.IsEnabled != nil {
		db = db.Where("is_enabled = ?", *query.IsEnabled)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count settings: %w", err)
	}
	var rows []Model
	if err := db.Order("setting_key ASC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list settings: %w", err)
	}
	items := make([]Record, 0, len(rows))
	for _, row := range rows {
		items = append(items, recordFromModel(row))
	}
	return items, total, nil
}

func (r *Repository) Find(ctx context.Context, key string) (Record, error) {
	var row Model
	if err := r.db.WithContext(ctx).Where("setting_key = ?", key).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Record{}, ErrNotFound
		}
		return Record{}, err
	}
	return recordFromModel(row), nil
}

func (r *Repository) Create(ctx context.Context, value *Record) error {
	row := Model{Key: value.Key, Value: value.Value, ValueType: value.ValueType, Description: value.Description, IsEnabled: value.IsEnabled, IsBuiltin: value.IsBuiltin, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return err
	}
	value.ID = row.ID
	return nil
}

func (r *Repository) Update(ctx context.Context, key string, value Record) error {
	return r.db.WithContext(ctx).Model(&Model{}).Where("setting_key = ?", key).Updates(map[string]any{"value": value.Value, "value_type": value.ValueType, "description": value.Description, "updated_at": value.UpdatedAt}).Error
}
func (r *Repository) UpdateStatus(ctx context.Context, key string, status yesno.Value, now time.Time) error {
	return r.db.WithContext(ctx).Model(&Model{}).Where("setting_key = ?", key).Updates(map[string]any{"is_enabled": status, "updated_at": now}).Error
}
func (r *Repository) Delete(ctx context.Context, key string) error {
	return r.db.WithContext(ctx).Where("setting_key = ?", key).Delete(&Model{}).Error
}
