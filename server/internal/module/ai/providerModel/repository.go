package providermodel

import (
	"context"
	"errors"
	"time"

	"admin/server/internal/module/ai/pricing"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type SyncItem struct {
	UpstreamModel   string
	ModelKey        string
	DisplayName     string
	ModelKind       pricing.ModelKind
	PricingMode     pricing.PricingMode
	PriceUnit       pricing.PriceUnit
	OfficialModelID *int64
	Status          yesno.Value
}

type SyncCounts struct {
	Inserted    int64
	Updated     int64
	Deactivated int64
}

type UpdateValues struct {
	DisplayName     string
	ModelKind       pricing.ModelKind
	PricingMode     pricing.PricingMode
	PriceUnit       pricing.PriceUnit
	OfficialModelID *int64
	Status          yesno.Value
	Prices          []Price
	UpdatedAt       time.Time
}

func (r *Repository) List(ctx context.Context, providerID int64, query ListQuery) ([]Model, int64, error) {
	countDB := listFilter(r.db.WithContext(ctx).Model(&Model{}).Where("provider_id = ?", providerID), query)
	var total int64
	if err := countDB.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []Model
	db := listFilter(r.db.WithContext(ctx).Model(&Model{}).Where("provider_id = ?", providerID), query)
	if err := db.Order("model_kind ASC, id ASC").
		Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *Repository) FindByID(ctx context.Context, providerID, id int64) (Model, error) {
	var row Model
	if err := r.db.WithContext(ctx).
		Where("provider_id = ? AND id = ?", providerID, id).First(&row).Error; err != nil {
		return Model{}, err
	}
	return row, nil
}

func (r *Repository) PricesByModelIDs(ctx context.Context, ids []int64) (map[int64][]Price, error) {
	result := make(map[int64][]Price, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []Price
	if err := r.db.WithContext(ctx).Where("provider_model_id IN ?", ids).
		Order("provider_model_id ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ProviderModelID] = append(result[row.ProviderModelID], row)
	}
	return result, nil
}

func (r *Repository) OfficialByIDs(ctx context.Context, ids []int64) (map[int64]Official, error) {
	result := make(map[int64]Official, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []Official
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID] = row
	}
	return result, nil
}

func (r *Repository) OfficialByVendor(ctx context.Context, vendor int16) ([]Official, error) {
	var rows []Official
	if err := r.db.WithContext(ctx).Where("vendor = ?", vendor).
		Order("sort_order ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) Sync(ctx context.Context, providerID int64, items []SyncItem, seenAt time.Time) (SyncCounts, error) {
	var counts SyncCounts
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			var row Model
			err := tx.Where("provider_id = ? AND upstream_model = ?", providerID, item.UpstreamModel).
				First(&row).Error
			switch {
			case err == nil:
				updates := map[string]any{
					"display_name": item.DisplayName,
					"last_seen_at": seenAt,
					"updated_at":   seenAt,
				}
				if row.OfficialModelID == nil && item.OfficialModelID != nil {
					updates["official_model_id"] = *item.OfficialModelID
				}
				if err := tx.Model(&Model{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
					return err
				}
				counts.Updated++
			case errors.Is(err, gorm.ErrRecordNotFound):
				now := seenAt
				model := Model{
					ProviderID: providerID, UpstreamModel: item.UpstreamModel,
					ModelKey: item.ModelKey, DisplayName: item.DisplayName,
					ModelKind: item.ModelKind, PricingMode: item.PricingMode,
					PriceUnit: item.PriceUnit, OfficialModelID: item.OfficialModelID,
					Source: SourceSync, Status: item.Status, LastSeenAt: &now, Version: 1,
					CreatedAt: seenAt, UpdatedAt: seenAt,
				}
				if err := tx.Create(&model).Error; err != nil {
					return err
				}
				counts.Inserted++
			default:
				return err
			}
		}

		result := tx.Model(&Model{}).
			Where("provider_id = ? AND source = ? AND deleted_at IS NULL AND (last_seen_at IS NULL OR last_seen_at < ?)",
				providerID, SourceSync, seenAt).
			Update("status", yesno.No)
		if result.Error != nil {
			return result.Error
		}
		counts.Deactivated = result.RowsAffected
		return nil
	})
	if err != nil {
		return SyncCounts{}, err
	}
	return counts, nil
}

func (r *Repository) UpdateConfig(ctx context.Context, providerID, id int64, values UpdateValues) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"display_name":      values.DisplayName,
			"model_kind":        values.ModelKind,
			"pricing_mode":      values.PricingMode,
			"price_unit":        values.PriceUnit,
			"official_model_id": values.OfficialModelID,
			"status":            values.Status,
			"updated_at":        values.UpdatedAt,
			"version":           gorm.Expr("version + 1"),
		}
		result := tx.Model(&Model{}).Where("provider_id = ? AND id = ?", providerID, id).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Where("provider_model_id = ? AND deleted_at IS NULL", id).Delete(&Price{}).Error; err != nil {
			return err
		}
		for _, price := range values.Prices {
			price.ID = 0
			price.ProviderModelID = id
			price.Status = yesno.Yes
			price.Version = 1
			price.CreatedAt = values.UpdatedAt
			price.UpdatedAt = values.UpdatedAt
			if err := tx.Create(&price).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func listFilter(db *gorm.DB, query ListQuery) *gorm.DB {
	if query.Keyword != "" {
		pattern := "%" + query.Keyword + "%"
		db = db.Where("display_name ILIKE ? OR upstream_model ILIKE ? OR model_key ILIKE ?", pattern, pattern, pattern)
	}
	if query.ModelKind != 0 {
		db = db.Where("model_kind = ?", query.ModelKind)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	return db
}
