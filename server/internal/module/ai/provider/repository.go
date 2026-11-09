package provider

import (
	"context"
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type UpdateValues struct {
	Name             string
	Vendor           Vendor
	Protocol         Protocol
	BaseURL          string
	APIKeyCiphertext *string
	EncryptionKeyID  *string
	TimeoutMS        int32
	SortOrder        int32
	Status           yesno.Value
	TokenRatePPM     *int64
	UpdatedAt        time.Time
}

func (r *Repository) Create(ctx context.Context, row Model) (int64, error) {
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

func (r *Repository) FindByID(ctx context.Context, id int64) (Model, error) {
	var row Model
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return Model{}, err
	}
	return row, nil
}

func (r *Repository) List(ctx context.Context, query ListQuery) ([]Model, int64, error) {
	countDB := modelFilter(r.db.WithContext(ctx).Model(&Model{}), query)
	var total int64
	if err := countDB.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []Model
	filtered := modelFilter(r.db.WithContext(ctx).Model(&Model{}), query)
	if err := filtered.Order("sort_order ASC, id DESC").
		Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *Repository) Update(ctx context.Context, id int64, values UpdateValues) error {
	updates := map[string]any{
		"name": values.Name, "vendor": values.Vendor, "protocol": values.Protocol,
		"base_url": values.BaseURL, "timeout_ms": values.TimeoutMS,
		"sort_order": values.SortOrder, "status": values.Status, "updated_at": values.UpdatedAt,
	}
	if values.TokenRatePPM != nil {
		updates["token_rate_multiplier_ppm"] = *values.TokenRatePPM
	}
	if values.APIKeyCiphertext != nil && values.EncryptionKeyID != nil {
		updates["api_key_ciphertext"] = *values.APIKeyCiphertext
		updates["encryption_key_id"] = *values.EncryptionKeyID
	}
	result := r.db.WithContext(ctx).Model(&Model{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id int64, status yesno.Value, now time.Time) error {
	result := r.db.WithContext(ctx).Model(&Model{}).Where("id = ?", id).
		Updates(map[string]any{"status": status, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&Model{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func modelFilter(db *gorm.DB, query ListQuery) *gorm.DB {
	if query.Keyword != "" {
		pattern := "%" + query.Keyword + "%"
		db = db.Where("name ILIKE ? OR base_url ILIKE ?", pattern, pattern)
	}
	if query.Vendor != 0 {
		db = db.Where("vendor = ?", query.Vendor)
	}
	if query.Protocol != 0 {
		db = db.Where("protocol = ?", query.Protocol)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	return db
}
