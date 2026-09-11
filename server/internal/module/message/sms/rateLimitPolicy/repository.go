package rateLimitPolicy

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

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

func (r *Repository) UpdatePolicy(ctx context.Context, platformID int64, key string, limit, windowSeconds int, now time.Time) (Model, error) {
	result := r.db.WithContext(ctx).Model(&Model{}).
		Where("platform_id = ? AND policy_key = ?", platformID, key).
		Updates(map[string]any{
			"limit_count":    limit,
			"window_seconds": windowSeconds,
			"revision":       gorm.Expr("revision + 1"),
			"updated_at":     now,
		})
	if result.Error != nil {
		return Model{}, result.Error
	}
	if result.RowsAffected == 0 {
		return Model{}, gorm.ErrRecordNotFound
	}
	var value Model
	if err := r.db.WithContext(ctx).Where("platform_id = ? AND policy_key = ?", platformID, key).First(&value).Error; err != nil {
		return Model{}, err
	}
	return value, nil
}

// ProvisionDefaults inserts the two fixed policies for a platform; it is
// idempotent and is also used by the platform lifecycle transaction.
func (r *Repository) ProvisionDefaults(ctx context.Context, platformID int64, now time.Time) error {
	for _, fixed := range FixedPolicies() {
		if err := r.db.WithContext(ctx).Exec(`
INSERT INTO message_sms_rate_limit_policy
  (platform_id, policy_key, mode, dimension, limit_count, window_seconds, revision, created_at, updated_at)
VALUES (?,?,?,?,?,?,1,?,?)
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
	return rows, nil
}
