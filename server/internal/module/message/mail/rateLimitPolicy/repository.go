package ratelimitpolicy

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// ProvisionDefaults creates the fixed business mail policies for a newly
// created authentication platform. It is idempotent so a retried lifecycle
// callback cannot create duplicate rows or reset an existing catalog.
func (r *Repository) ProvisionDefaults(ctx context.Context, platformID int64) error {
	if platformID < 1 {
		return fmt.Errorf("rate limit policy platform is invalid")
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	rows := make([]Model, 0, len(fixedRateLimitSpecs))
	for _, spec := range fixedRateLimitSpecs {
		rows = append(rows, Model{
			PlatformID: platformID, Key: spec.Key, Mode: spec.Mode, Dimension: spec.Dimension,
			Limit: spec.Limit, WindowSeconds: spec.WindowSeconds, Revision: 1,
			CreatedAt: now, UpdatedAt: now,
		})
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
		return fmt.Errorf("provision mail rate limit policies: %w", err)
	}
	_, err := r.List(ctx, platformID)
	return err
}

// DeleteForPlatform removes all policy rows when an authentication platform
// is soft-deleted. Rate-limit policy rows have no independent business life.
func (r *Repository) DeleteForPlatform(ctx context.Context, platformID int64) error {
	if platformID < 1 {
		return fmt.Errorf("rate limit policy platform is invalid")
	}
	if err := r.db.WithContext(ctx).Where("platform_id = ?", platformID).Delete(&Model{}).Error; err != nil {
		return fmt.Errorf("delete mail rate limit policies: %w", err)
	}
	return nil
}

func (r *Repository) List(ctx context.Context, platformID int64) (Catalog, error) {
	var rows []Model
	if err := r.db.WithContext(ctx).Where("platform_id = ?", platformID).Find(&rows).Error; err != nil {
		return Catalog{}, err
	}
	return buildCatalog(platformID, rows)
}

// ListAll is the management view. Rate-limit data is platform-scoped, but the
// Admin console is the control plane and must be able to inspect every
// platform catalog in one request. Sending still uses List(platformID).
func (r *Repository) ListAll(ctx context.Context) ([]Catalog, error) {
	var rows []Model
	if err := r.db.WithContext(ctx).Order("platform_id ASC, policy_key ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	groups := make(map[int64][]Model)
	for _, row := range rows {
		groups[row.PlatformID] = append(groups[row.PlatformID], row)
	}
	var platforms []struct {
		ID   int64
		Code string
		Name string
	}
	if err := r.db.WithContext(ctx).Table("permission_auth_platform").Select("id, code, name").Where("deleted_at IS NULL").Order("id ASC").Find(&platforms).Error; err != nil {
		return nil, err
	}
	result := make([]Catalog, 0, len(platforms))
	for _, platform := range platforms {
		platformID := platform.ID
		catalog, err := buildCatalog(platformID, groups[platformID])
		if err != nil {
			return nil, err
		}
		catalog.PlatformCode = platform.Code
		catalog.PlatformName = platform.Name
		result = append(result, catalog)
	}
	return result, nil
}

func (r *Repository) Update(ctx context.Context, platformID int64, input Input) (Catalog, error) {
	var catalog Catalog
	err := r.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var rows []Model
		if err := transaction.Where("platform_id = ?", platformID).Clauses(clause.Locking{Strength: "UPDATE"}).Order("policy_key ASC").Find(&rows).Error; err != nil {
			return err
		}
		current, err := buildCatalog(platformID, rows)
		if err != nil {
			return err
		}
		query := transaction.Model(&Model{}).Where("platform_id = ? AND policy_key = ?", platformID, input.Key).Updates(map[string]any{
			"limit_count": input.Limit, "window_seconds": input.WindowSeconds,
			"revision": current.Version + 1, "updated_at": time.Now().UTC(),
		})
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		var updated []Model
		if err := transaction.Where("platform_id = ?", platformID).Find(&updated).Error; err != nil {
			return err
		}
		catalog, err = buildCatalog(platformID, updated)
		return err
	})
	return catalog, err
}

func (r *Repository) ListRateLimitPolicies(ctx context.Context, platformID int64) (Catalog, error) {
	return r.List(ctx, platformID)
}

func (r *Repository) UpdateRateLimitPolicy(ctx context.Context, platformID int64, input Input) (Catalog, error) {
	return r.Update(ctx, platformID, input)
}

func buildCatalog(platformID int64, rows []Model) (Catalog, error) {
	if platformID < 1 {
		return Catalog{}, fmt.Errorf("rate limit policy platform is invalid")
	}
	if len(rows) != len(fixedRateLimitPolicyKeys) {
		return Catalog{}, fmt.Errorf("rate limit policy table must contain exactly %d rows, got %d", len(fixedRateLimitPolicyKeys), len(rows))
	}
	byKey := make(map[string]Model, len(rows))
	var version int64
	for _, row := range rows {
		if row.PlatformID != platformID {
			return Catalog{}, fmt.Errorf("rate limit policy platform is inconsistent")
		}
		if _, exists := byKey[row.Key]; exists {
			return Catalog{}, fmt.Errorf("rate limit policy %q is duplicated", row.Key)
		}
		spec, found := fixedRateLimitSpecByKey(row.Key)
		if !found || row.Mode != spec.Mode || row.Dimension != spec.Dimension {
			return Catalog{}, fmt.Errorf("rate limit policy %q has invalid mode or dimension", row.Key)
		}
		if row.Limit < 1 || row.Limit > 100000 || row.WindowSeconds < 1 || row.WindowSeconds > 86400 {
			return Catalog{}, fmt.Errorf("rate limit policy %q values are out of range", row.Key)
		}
		if row.Revision < 1 || row.UpdatedAt.IsZero() {
			return Catalog{}, fmt.Errorf("rate limit policy %q has invalid revision or timestamp", row.Key)
		}
		byKey[row.Key] = row
		if row.Revision > version {
			version = row.Revision
		}
	}
	ordered := make([]Model, 0, len(fixedRateLimitPolicyKeys))
	for _, key := range fixedRateLimitPolicyKeys {
		row, found := byKey[key]
		if !found {
			return Catalog{}, fmt.Errorf("rate limit policy %q is missing", key)
		}
		ordered = append(ordered, row)
	}
	if version < 1 {
		return Catalog{}, fmt.Errorf("rate limit policy catalog version is invalid")
	}
	return Catalog{PlatformID: platformID, Version: version, Policies: ordered}, nil
}

func BuildCatalog(platformID int64, rows []Model) (Catalog, error) {
	return buildCatalog(platformID, rows)
}
