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

func (r *Repository) List(ctx context.Context) (Catalog, error) {
	var rows []Model
	if err := r.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return Catalog{}, err
	}
	return buildCatalog(rows)
}

func (r *Repository) Update(ctx context.Context, input Input) (Catalog, error) {
	var catalog Catalog
	err := r.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var rows []Model
		if err := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).Order("policy_key ASC").Find(&rows).Error; err != nil {
			return err
		}
		current, err := buildCatalog(rows)
		if err != nil {
			return err
		}
		query := transaction.Model(&Model{}).Where("policy_key = ?", input.Key).Updates(map[string]any{
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
		if err := transaction.Find(&updated).Error; err != nil {
			return err
		}
		catalog, err = buildCatalog(updated)
		return err
	})
	return catalog, err
}

func (r *Repository) ListRateLimitPolicies(ctx context.Context) (Catalog, error) {
	return r.List(ctx)
}

func (r *Repository) UpdateRateLimitPolicy(ctx context.Context, input Input) (Catalog, error) {
	return r.Update(ctx, input)
}

func buildCatalog(rows []Model) (Catalog, error) {
	if len(rows) != len(fixedRateLimitPolicyKeys) {
		return Catalog{}, fmt.Errorf("rate limit policy table must contain exactly %d rows, got %d", len(fixedRateLimitPolicyKeys), len(rows))
	}
	byKey := make(map[string]Model, len(rows))
	var version int64
	for _, row := range rows {
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
	return Catalog{Version: version, Policies: ordered}, nil
}

func BuildCatalog(rows []Model) (Catalog, error) { return buildCatalog(rows) }
