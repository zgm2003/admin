package ratelimitpolicy

import (
	"context"
	"fmt"
	"time"

	cachegeneration "admin/server/internal/shared/cacheGeneration"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db          *gorm.DB
	generations *cachegeneration.Repository
	scope       cachegeneration.Scope
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) SetGenerations(repository *cachegeneration.Repository, scope cachegeneration.Scope) {
	r.generations = repository
	r.scope = scope
}

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
			Limit: spec.Limit, WindowSeconds: spec.WindowSeconds,
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

func (r *Repository) Update(ctx context.Context, platformID int64, input Input, expected int64, now time.Time) (Catalog, cachegeneration.MutationResult, error) {
	if r == nil || r.db == nil || r.generations == nil {
		return Catalog{}, cachegeneration.MutationResult{}, fmt.Errorf("mail rate limit generation dependencies are not configured")
	}
	if err := r.scope.Validate(); err != nil {
		return Catalog{}, cachegeneration.MutationResult{}, err
	}
	var catalog Catalog
	mutation := cachegeneration.MutationResult{}
	err := r.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var rows []Model
		if err := transaction.WithContext(ctx).Where("platform_id = ?", platformID).Clauses(clause.Locking{Strength: "UPDATE"}).Order("policy_key ASC").Find(&rows).Error; err != nil {
			return fmt.Errorf("%w: %w", cachegeneration.ErrMutationRolledBack, err)
		}
		current, err := buildCatalog(platformID, rows)
		if err != nil {
			return fmt.Errorf("%w: %w", cachegeneration.ErrMutationRolledBack, err)
		}
		policyIndex := -1
		for index := range current.Policies {
			if current.Policies[index].Key == input.Key {
				policyIndex = index
				break
			}
		}
		if policyIndex < 0 {
			return fmt.Errorf("%w: %w", cachegeneration.ErrMutationRolledBack, gorm.ErrRecordNotFound)
		}
		policy := current.Policies[policyIndex]
		if policy.Limit == input.Limit && policy.WindowSeconds == input.WindowSeconds {
			catalog = current
			return nil
		}
		query := transaction.WithContext(ctx).Model(&Model{}).Where("platform_id = ? AND policy_key = ?", platformID, input.Key).Updates(map[string]any{
			"limit_count": input.Limit, "window_seconds": input.WindowSeconds,
			"updated_at": now,
		})
		if query.Error != nil {
			return fmt.Errorf("%w: %w", cachegeneration.ErrMutationRolledBack, query.Error)
		}
		if query.RowsAffected != 1 {
			return fmt.Errorf("%w: %w", cachegeneration.ErrMutationRolledBack, gorm.ErrRecordNotFound)
		}
		current.Policies[policyIndex].Limit = input.Limit
		current.Policies[policyIndex].WindowSeconds = input.WindowSeconds
		current.Policies[policyIndex].UpdatedAt = now
		event, err := r.generations.AdvanceTx(ctx, transaction, r.scope, expected, now)
		if err != nil {
			return fmt.Errorf("%w: %w", cachegeneration.ErrMutationRolledBack, err)
		}
		catalog = current
		mutation = cachegeneration.MutationResult{Changed: true, Generation: event.Generation, OutboxID: event.ID}
		return nil
	})
	if err != nil {
		return Catalog{}, cachegeneration.MutationResult{}, err
	}
	return catalog, mutation, nil
}

func (r *Repository) ListRateLimitPolicies(ctx context.Context, platformID int64) (Catalog, error) {
	return r.List(ctx, platformID)
}

func buildCatalog(platformID int64, rows []Model) (Catalog, error) {
	if platformID < 1 {
		return Catalog{}, fmt.Errorf("rate limit policy platform is invalid")
	}
	if len(rows) != len(fixedRateLimitPolicyKeys) {
		return Catalog{}, fmt.Errorf("rate limit policy table must contain exactly %d rows, got %d", len(fixedRateLimitPolicyKeys), len(rows))
	}
	byKey := make(map[string]Model, len(rows))
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
		if row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() {
			return Catalog{}, fmt.Errorf("rate limit policy %q has an invalid timestamp", row.Key)
		}
		byKey[row.Key] = row
	}
	ordered := make([]Model, 0, len(fixedRateLimitPolicyKeys))
	for _, key := range fixedRateLimitPolicyKeys {
		row, found := byKey[key]
		if !found {
			return Catalog{}, fmt.Errorf("rate limit policy %q is missing", key)
		}
		ordered = append(ordered, row)
	}
	return Catalog{PlatformID: platformID, Policies: ordered}, nil
}

func BuildCatalog(platformID int64, rows []Model) (Catalog, error) {
	return buildCatalog(platformID, rows)
}
