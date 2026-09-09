package cosconfig

import (
	"admin/server/internal/shared/yesno"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"time"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }
func (r *Repository) Count(ctx context.Context, q ListQuery) (int64, error) {
	var n int64
	db := r.db.WithContext(ctx).Model(&Model{}).Where("deleted_at IS NULL")
	if q.Keyword != "" {
		p := "%" + q.Keyword + "%"
		db = db.Where("name ILIKE ? OR bucket ILIKE ?", p, p)
	}
	if q.IsEnabled != nil {
		db = db.Where("is_enabled = ?", *q.IsEnabled)
	}
	if err := db.Count(&n).Error; err != nil {
		return 0, fmt.Errorf("count configs: %w", err)
	}
	return n, nil
}
func (r *Repository) List(ctx context.Context, q ListQuery) ([]Model, error) {
	rows := []Model{}
	db := r.db.WithContext(ctx).Where("deleted_at IS NULL")
	if q.Keyword != "" {
		p := "%" + q.Keyword + "%"
		db = db.Where("name ILIKE ? OR bucket ILIKE ?", p, p)
	}
	if q.IsEnabled != nil {
		db = db.Where("is_enabled = ?", *q.IsEnabled)
	}
	if err := db.Order("created_at DESC,id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
func (r *Repository) FindByID(ctx context.Context, id int64) (Model, error) {
	var m Model
	err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).Take(&m).Error
	return m, err
}
func (r *Repository) LockByID(ctx context.Context, id int64) (Model, error) {
	var m Model
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND deleted_at IS NULL", id).Take(&m).Error
	return m, err
}
func (r *Repository) Create(ctx context.Context, m *Model) error {
	if err := r.quietDB(ctx).Create(m).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "ux_storage_cos_config_name_active" {
			return fmt.Errorf("create COS config: %w", ErrNameConflict)
		}
		return err
	}
	return nil
}
func (r *Repository) Update(ctx context.Context, id int64, values map[string]any) error {
	result := r.quietDB(ctx).Model(&Model{}).Where("id = ? AND deleted_at IS NULL", id).Updates(values)
	if result.Error != nil {
		var pgErr *pgconn.PgError
		if errors.As(result.Error, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "ux_storage_cos_config_name_active" {
			return fmt.Errorf("update COS config: %w", ErrNameConflict)
		}
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) quietDB(ctx context.Context) *gorm.DB {
	return r.db.Session(&gorm.Session{Logger: r.db.Logger.LogMode(logger.Silent)}).WithContext(ctx)
}
func (r *Repository) MarkDeleted(ctx context.Context, id int64, now time.Time) error {
	result := r.db.WithContext(ctx).Model(&Model{}).Where("id = ? AND deleted_at IS NULL", id).Updates(map[string]any{"is_enabled": yesno.No, "deleted_at": now, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *Repository) CountEnabledRules(ctx context.Context, id int64) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("storage_upload_rule").Where("cos_config_id = ? AND is_enabled = 1 AND deleted_at IS NULL", id).Count(&n).Error
	return n, err
}
func (r *Repository) Transaction(ctx context.Context, fn func(*Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return fn(NewRepository(tx)) })
}
