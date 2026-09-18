package rateLimitPolicy

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

type platformRow struct {
	ID   int64
	Code string
	Name string
}

func (r *Repository) ListPlatforms(ctx context.Context) ([]Catalog, error) {
	var platforms []platformRow
	if err := r.db.WithContext(ctx).Raw(`
SELECT id, code, name FROM permission_auth_platform
 WHERE deleted_at IS NULL ORDER BY code, id`).Scan(&platforms).Error; err != nil {
		return nil, err
	}
	catalogs := make([]Catalog, 0, len(platforms))
	for _, platform := range platforms {
		policyRows, err := r.policiesOf(ctx, platform.ID)
		if err != nil {
			return nil, err
		}
		catalogs = append(catalogs, Catalog{
			PlatformID: platform.ID, PlatformCode: platform.Code,
			PlatformName: platform.Name, Policies: policyRows,
		})
	}
	return catalogs, nil
}

func (r *Repository) FindPlatform(ctx context.Context, platformID int64) (Catalog, error) {
	var platform platformRow
	if err := r.db.WithContext(ctx).Raw(`
SELECT id, code, name FROM permission_auth_platform
 WHERE id = ? AND deleted_at IS NULL`, platformID).Scan(&platform).Error; err != nil {
		return Catalog{}, err
	}
	if platform.ID == 0 {
		return Catalog{}, ErrPlatformNotFound
	}
	policyRows, err := r.policiesOf(ctx, platform.ID)
	if err != nil {
		return Catalog{}, err
	}
	return Catalog{
		PlatformID: platform.ID, PlatformCode: platform.Code,
		PlatformName: platform.Name, Policies: policyRows,
	}, nil
}

func (r *Repository) UpdatePolicy(ctx context.Context, platformID int64, key string, limit, windowSeconds int, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	if r == nil || r.db == nil || r.generations == nil {
		return cachegeneration.MutationResult{}, fmt.Errorf("sms rate limit generation dependencies are not configured")
	}
	if err := r.scope.Validate(); err != nil {
		return cachegeneration.MutationResult{}, err
	}
	mutation := cachegeneration.MutationResult{}
	err := r.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var rows []Model
		if err := transaction.WithContext(ctx).Where("platform_id = ?", platformID).
			Clauses(clause.Locking{Strength: "UPDATE"}).Order("policy_key").Find(&rows).Error; err != nil {
			return fmt.Errorf("%w: %w", cachegeneration.ErrMutationRolledBack, err)
		}
		policies, err := validatePolicyRows(platformID, rows)
		if err != nil {
			return fmt.Errorf("%w: %w", cachegeneration.ErrMutationRolledBack, err)
		}
		var current Model
		for _, policy := range policies {
			if policy.Key == key {
				current = policy
				break
			}
		}
		if current.Key == "" {
			return fmt.Errorf("%w: %w", cachegeneration.ErrMutationRolledBack, gorm.ErrRecordNotFound)
		}
		if current.Limit == limit && current.WindowSeconds == windowSeconds {
			return nil
		}
		result := transaction.WithContext(ctx).Model(&Model{}).
			Where("platform_id = ? AND policy_key = ?", platformID, key).
			Updates(map[string]any{"limit_count": limit, "window_seconds": windowSeconds, "updated_at": now})
		if result.Error != nil {
			return fmt.Errorf("%w: %w", cachegeneration.ErrMutationRolledBack, result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("%w: %w", cachegeneration.ErrMutationRolledBack, gorm.ErrRecordNotFound)
		}
		event, err := r.generations.AdvanceTx(ctx, transaction, r.scope, expected, now)
		if err != nil {
			return fmt.Errorf("%w: %w", cachegeneration.ErrMutationRolledBack, err)
		}
		mutation = cachegeneration.MutationResult{Changed: true, Generation: event.Generation, OutboxID: event.ID}
		return nil
	})
	if err != nil {
		return cachegeneration.MutationResult{}, err
	}
	return mutation, nil
}

// ProvisionDefaults inserts the two fixed policies for a platform; it is
// idempotent and is also used by the platform lifecycle transaction.
func (r *Repository) ProvisionDefaults(ctx context.Context, platformID int64, now time.Time) error {
	for _, fixed := range FixedPolicies() {
		if err := r.db.WithContext(ctx).Exec(`
INSERT INTO message_sms_rate_limit_policy
  (platform_id, policy_key, mode, dimension, limit_count, window_seconds, created_at, updated_at)
VALUES (?,?,?,?,?,?,?,?)
ON CONFLICT (platform_id, policy_key) DO NOTHING`,
			platformID, fixed.Key, fixed.Mode, fixed.Dimension,
			fixed.Limit, fixed.WindowSeconds, now, now).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) DeleteForPlatform(ctx context.Context, platformID int64) error {
	return r.db.WithContext(ctx).Where("platform_id = ?", platformID).Delete(&Model{}).Error
}

func (r *Repository) policiesOf(ctx context.Context, platformID int64) ([]Model, error) {
	var rows []Model
	if err := r.db.WithContext(ctx).
		Where("platform_id = ?", platformID).
		Order("policy_key").Find(&rows).Error; err != nil {
		return nil, err
	}
	return validatePolicyRows(platformID, rows)
}

func validatePolicyRows(platformID int64, rows []Model) ([]Model, error) {
	if platformID < 1 || len(rows) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	if len(rows) != len(FixedPolicies()) {
		return nil, fmt.Errorf("sms rate limit policy catalog is incomplete")
	}
	byKey := make(map[string]Model, len(rows))
	for _, row := range rows {
		fixed, found := FixedPolicyByKey(row.Key)
		if !found || row.PlatformID != platformID || row.Mode != fixed.Mode || row.Dimension != fixed.Dimension ||
			row.Limit < minLimit || row.Limit > maxLimit || row.WindowSeconds < minWindowSeconds || row.WindowSeconds > maxWindowSeconds ||
			row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() {
			return nil, fmt.Errorf("sms rate limit policy %q is invalid", row.Key)
		}
		if _, exists := byKey[row.Key]; exists {
			return nil, fmt.Errorf("sms rate limit policy %q is duplicated", row.Key)
		}
		byKey[row.Key] = row
	}
	ordered := make([]Model, 0, len(rows))
	for _, fixed := range FixedPolicies() {
		row, found := byKey[fixed.Key]
		if !found {
			return nil, fmt.Errorf("sms rate limit policy %q is missing", fixed.Key)
		}
		ordered = append(ordered, row)
	}
	return ordered, nil
}
