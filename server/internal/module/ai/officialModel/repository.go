package officialmodel

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func modelFilter(db *gorm.DB, query ListQuery) *gorm.DB {
	if query.Vendor != "" {
		db = db.Where("vendor_code = ?", query.Vendor)
	}
	if query.Keyword != "" {
		like := "%" + query.Keyword + "%"
		db = db.Where("(display_name ILIKE ? OR model_key ILIKE ? OR vendor_name ILIKE ?)", like, like, like)
	}
	return db
}

func (r *Repository) Count(ctx context.Context, query ListQuery) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("official model database missing")
	}
	var total int64
	err := modelFilter(r.db.WithContext(ctx).Model(&Model{}), query).Count(&total).Error
	return total, err
}

func (r *Repository) List(ctx context.Context, query ListQuery) ([]Model, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("official model database missing")
	}
	var rows []Model
	err := modelFilter(r.db.WithContext(ctx).Model(&Model{}), query).
		Order("sort_order ASC, display_name ASC, id ASC").
		Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&rows).Error
	return rows, err
}

func (r *Repository) UpdatePrice(ctx context.Context, id int64, expectedVersion int64, inputCNY, outputCNY int64, now time.Time) error {
	if r == nil || r.db == nil {
		return errors.New("official model database missing")
	}
	result := r.db.WithContext(ctx).Model(&Model{}).
		Where("id = ? AND version = ?", id, expectedVersion).
		Updates(map[string]any{
			"input_cny_micros": inputCNY, "output_cny_micros": outputCNY,
			"version": gorm.Expr("version + 1"), "updated_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return r.classifyMiss(ctx, id)
	}
	return nil
}

func (r *Repository) ResetPrice(ctx context.Context, id int64, expectedVersion int64, now time.Time) error {
	if r == nil || r.db == nil {
		return errors.New("official model database missing")
	}
	result := r.db.WithContext(ctx).Model(&Model{}).
		Where("id = ? AND version = ?", id, expectedVersion).
		Updates(map[string]any{
			"input_cny_micros":  gorm.Expr("input_usd_micros"),
			"output_cny_micros": gorm.Expr("output_usd_micros"),
			"version":           gorm.Expr("version + 1"), "updated_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return r.classifyMiss(ctx, id)
	}
	return nil
}

func (r *Repository) classifyMiss(ctx context.Context, id int64) error {
	var found int64
	if err := r.db.WithContext(ctx).Model(&Model{}).Where("id = ?", id).Count(&found).Error; err != nil {
		return err
	}
	if found == 0 {
		return gorm.ErrRecordNotFound
	}
	return errVersionConflict
}

func (r *Repository) SyncCatalog(ctx context.Context, catalog Catalog, now time.Time) (SyncResult, error) {
	if r == nil || r.db == nil {
		return SyncResult{}, errors.New("official model database missing")
	}
	result := SyncResult{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, entry := range catalog.Models {
			var row Model
			found := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("vendor_code = ? AND model_key = ?", entry.VendorCode, entry.ModelKey).
				Limit(1).Find(&row)
			if found.Error != nil {
				return found.Error
			}
			switch {
			case found.RowsAffected == 0:
				row = Model{
					VendorCode: entry.VendorCode, VendorName: entry.VendorName,
					ModelKey: entry.ModelKey, DisplayName: entry.DisplayName,
					InputUSDMicros: entry.InputUSDMicros, OutputUSDMicros: entry.OutputUSDMicros,
					InputCNYMicros: entry.InputUSDMicros, OutputCNYMicros: entry.OutputUSDMicros,
					CatalogVersion: catalog.Version, SourceURL: catalog.SourceURL,
					VerifiedAt: catalog.VerifiedAt, SortOrder: entry.SortOrder,
					Version: 1, CreatedAt: now, UpdatedAt: now,
				}
				if err := tx.Create(&row).Error; err != nil {
					return err
				}
				result.Inserted++
			default:
				if row.InputUSDMicros == entry.InputUSDMicros &&
					row.OutputUSDMicros == entry.OutputUSDMicros &&
					row.VendorName == entry.VendorName &&
					row.DisplayName == entry.DisplayName &&
					row.CatalogVersion == catalog.Version &&
					row.SourceURL == catalog.SourceURL &&
					row.VerifiedAt.Equal(catalog.VerifiedAt) &&
					row.SortOrder == entry.SortOrder {
					result.Unchanged++
					continue
				}
				update := tx.Model(&Model{}).Where("id = ? AND version = ?", row.ID, row.Version).Updates(map[string]any{
					"vendor_name": entry.VendorName, "display_name": entry.DisplayName,
					"input_usd_micros": entry.InputUSDMicros, "output_usd_micros": entry.OutputUSDMicros,
					"catalog_version": catalog.Version, "source_url": catalog.SourceURL,
					"verified_at": catalog.VerifiedAt, "sort_order": entry.SortOrder,
					"version": gorm.Expr("version + 1"), "updated_at": now,
				})
				if update.Error != nil {
					return update.Error
				}
				if update.RowsAffected != 1 {
					return errVersionConflict
				}
				result.Updated++
			}
		}
		return nil
	})
	if err != nil {
		return SyncResult{}, err
	}
	return result, nil
}
