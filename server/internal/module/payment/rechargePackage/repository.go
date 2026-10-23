package rechargepackage

import (
	"context"
	"errors"
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func packageFilter(db *gorm.DB, query ListQuery) *gorm.DB {
	if query.Keyword != "" {
		like := "%" + query.Keyword + "%"
		db = db.Where("name ILIKE ?", like)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	return db
}

func (r *Repository) Count(ctx context.Context, query ListQuery) (int64, error) {
	var total int64
	if r == nil || r.db == nil {
		return 0, errors.New("recharge package database missing")
	}
	err := packageFilter(r.db.WithContext(ctx).Model(&Model{}), query).Count(&total).Error
	return total, err
}

func (r *Repository) List(ctx context.Context, query ListQuery) ([]Model, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("recharge package database missing")
	}
	var rows []Model
	err := packageFilter(r.db.WithContext(ctx).Where("deleted_at IS NULL"), query).
		Order("sort_order ASC, id ASC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&rows).Error
	return rows, err
}

func (r *Repository) FindByID(ctx context.Context, id int64) (Model, error) {
	var row Model
	if r == nil || r.db == nil {
		return row, errors.New("recharge package database missing")
	}
	err := r.db.WithContext(ctx).Where("deleted_at IS NULL").First(&row, id).Error
	return row, err
}

func (r *Repository) Create(ctx context.Context, row *Model) error {
	if r == nil || r.db == nil {
		return errors.New("recharge package database missing")
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return err
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, id int64, input UpdateInput, now time.Time) error {
	if r == nil || r.db == nil {
		return errors.New("recharge package database missing")
	}
	result := r.db.WithContext(ctx).Model(&Model{}).Where("id = ? AND deleted_at IS NULL", id).Updates(map[string]any{
		"name": input.Name, "amount_cents": input.AmountCents, "badge": input.Badge, "sort_order": input.SortOrder, "updated_at": now,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id int64, value yesno.Value, now time.Time) error {
	if r == nil || r.db == nil {
		return errors.New("recharge package database missing")
	}
	result := r.db.WithContext(ctx).Model(&Model{}).Where("id = ? AND deleted_at IS NULL", id).Updates(map[string]any{"status": value, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) MarkDeleted(ctx context.Context, id int64, now time.Time) error {
	if r == nil || r.db == nil {
		return errors.New("recharge package database missing")
	}
	result := r.db.WithContext(ctx).Model(&Model{}).Where("id = ? AND deleted_at IS NULL", id).Updates(map[string]any{"deleted_at": now, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) Catalog(ctx context.Context) ([]CatalogItem, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("recharge package database missing")
	}
	rows := make([]CatalogItem, 0)
	err := r.db.WithContext(ctx).Model(&Model{}).Select("id,name,amount_cents,badge,sort_order").Where("deleted_at IS NULL AND status = 1").Order("sort_order ASC, id ASC").Limit(MaxCatalogItems + 1).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) > MaxCatalogItems {
		return nil, errors.New("recharge package catalog too large")
	}
	return rows, nil
}
