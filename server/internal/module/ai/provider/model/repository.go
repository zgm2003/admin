package model

import (
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type priceVersionRow struct {
	ID              int64 `gorm:"column:id"`
	ProviderModelID int64 `gorm:"column:provider_model_id"`
	Version         int64
	Status          int16
	EffectiveAt     *time.Time
	Currency        string
	SourceCurrency  string
	SourceDigest    string
	SnapshotDigest  string
	SourcePayload   []byte
	SyncedAt        time.Time
	CreatedBy       int64
	PublishedBy     *int64
	PublishedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (priceVersionRow) TableName() string { return "ai_provider_model_price_version" }

type priceRateRow struct {
	ID                    int64
	PriceVersionID        int64 `gorm:"column:price_version_id"`
	Operation             string
	UsageKey              string
	Selector              []byte
	SelectorHash          string
	Unit                  string
	UnitQuantity          string
	SourceUnitPrice       string
	BillingUnitPriceUnits *int64
}

func (priceRateRow) TableName() string { return "ai_provider_model_price_rate" }

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }
func (r *GormRepository) CurrentProviderVersionID(ctx context.Context, modelID int64) (int64, error) {
	var id int64
	if err := r.db.WithContext(ctx).Raw(`SELECT v.id FROM ai_provider_model m JOIN ai_provider p ON p.id=m.provider_id JOIN ai_provider_version v ON v.provider_id=p.id AND v.version=p.current_version WHERE m.id=? AND m.deleted_at IS NULL AND p.deleted_at IS NULL`, modelID).Scan(&id).Error; err != nil {
		return 0, err
	}
	if id < 1 {
		return 0, ErrPriceNotFound
	}
	return id, nil
}

// WithModel serializes sync, edits and publication across API instances.
func (r *GormRepository) WithModel(ctx context.Context, modelID int64, fn func(Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct{ ID int64 }
		if err := tx.Table("ai_provider_model").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND deleted_at IS NULL", modelID).Take(&row).Error; err != nil {
			return err
		}
		return fn(NewRepository(tx))
	})
}
func (r *GormRepository) CreatePreview(ctx context.Context, p *PriceVersion) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := priceVersionRow{ProviderModelID: p.ProviderModelID, Version: p.Version, Status: 0, Currency: p.Currency, SourceCurrency: p.SourceCurrency, SourceDigest: p.SourceDigest, SnapshotDigest: p.SnapshotDigest, SourcePayload: p.SourcePayload, SyncedAt: p.SyncedAt, CreatedBy: p.CreatedBy, EffectiveAt: p.EffectiveAt}
		if err := tx.Create(&row).Error; err != nil {
			return fmt.Errorf("create AI price preview: %w", err)
		}
		p.ID = row.ID
		for i := range p.Rates {
			rr := priceRateRow{PriceVersionID: row.ID, Operation: p.Rates[i].Operation, UsageKey: p.Rates[i].UsageKey, Selector: p.Rates[i].Selector, SelectorHash: p.Rates[i].SelectorHash, Unit: p.Rates[i].Unit, UnitQuantity: p.Rates[i].UnitQuantity, SourceUnitPrice: p.Rates[i].SourceUnitPrice, BillingUnitPriceUnits: p.Rates[i].BillingUnitPriceUnits}
			if err := tx.Create(&rr).Error; err != nil {
				return fmt.Errorf("create AI price rate: %w", err)
			}
			p.Rates[i].ID = rr.ID
			p.Rates[i].PriceVersionID = row.ID
		}
		return nil
	})
}
func (r *GormRepository) Latest(ctx context.Context, providerModelID int64) (*PriceVersion, error) {
	var row priceVersionRow
	err := r.db.WithContext(ctx).Where("provider_model_id = ?", providerModelID).Order("version DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.load(ctx, row)
}
func (r *GormRepository) Find(ctx context.Context, providerModelID, versionID int64) (*PriceVersion, error) {
	var row priceVersionRow
	err := r.db.WithContext(ctx).Where("provider_model_id=? AND id=?", providerModelID, versionID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPriceNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.load(ctx, row)
}
func (r *GormRepository) load(ctx context.Context, row priceVersionRow) (*PriceVersion, error) {
	var rates []priceRateRow
	if err := r.db.WithContext(ctx).Where("price_version_id=?", row.ID).Order("id").Find(&rates).Error; err != nil {
		return nil, err
	}
	p := &PriceVersion{ID: row.ID, ProviderModelID: row.ProviderModelID, Version: row.Version, Status: row.Status, EffectiveAt: row.EffectiveAt, Currency: row.Currency, SourceCurrency: row.SourceCurrency, SourceDigest: row.SourceDigest, SnapshotDigest: row.SnapshotDigest, SourcePayload: row.SourcePayload, SyncedAt: row.SyncedAt, CreatedBy: row.CreatedBy, PublishedBy: row.PublishedBy, PublishedAt: row.PublishedAt}
	for _, rr := range rates {
		p.Rates = append(p.Rates, PriceRate{ID: rr.ID, PriceVersionID: rr.PriceVersionID, Operation: rr.Operation, UsageKey: rr.UsageKey, Selector: rr.Selector, SelectorHash: rr.SelectorHash, Unit: rr.Unit, UnitQuantity: rr.UnitQuantity, SourceUnitPrice: rr.SourceUnitPrice, BillingUnitPriceUnits: rr.BillingUnitPriceUnits})
	}
	return p, nil
}
func (r *GormRepository) UpdatePreview(ctx context.Context, p *PriceVersion) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&priceVersionRow{}).Where("id=? AND provider_model_id=? AND status=0", p.ID, p.ProviderModelID).Updates(map[string]any{"snapshot_digest": p.SnapshotDigest, "effective_at": p.EffectiveAt, "updated_at": time.Now()})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrPublishedImmutable
		}
		for _, rate := range p.Rates {
			if err := tx.Model(&priceRateRow{}).Where("id=? AND price_version_id=?", rate.ID, p.ID).Update("billing_unit_price_units", rate.BillingUnitPriceUnits).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
func (r *GormRepository) Publish(ctx context.Context, providerModelID, versionID int64, digest string, actorID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		res := tx.Model(&priceVersionRow{}).Where("id=? AND provider_model_id=? AND status=0 AND snapshot_digest=?", versionID, providerModelID, digest).Updates(map[string]any{"status": 1, "published_by": actorID, "published_at": now, "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrPriceDigestMismatch
		}
		return nil
	})
}
func (r *GormRepository) Effective(ctx context.Context, providerModelID int64, at time.Time) (*PriceVersion, error) {
	var row priceVersionRow
	err := r.db.WithContext(ctx).Where("provider_model_id=? AND status=1 AND effective_at<=? AND published_at<=?", providerModelID, at, at).Order("effective_at DESC,id DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPriceNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.load(ctx, row)
}
