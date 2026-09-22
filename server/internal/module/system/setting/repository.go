package setting

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type Repository struct {
	db          *gorm.DB
	generations *cachegeneration.Repository
	scope       cachegeneration.Scope
}

// errMutationRolledBack marks errors returned from the transaction callback.
// GORM rolls these transactions back before returning, so the service must not
// reinterpret a later generation advance as an acknowledged write.
var errMutationRolledBack = errors.New("system setting mutation transaction rolled back")

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db, scope: settingGenerationScope}
}

// SetGenerations 由显式装配注入 generation/outbox 事实来源。
func (r *Repository) SetGenerations(generations *cachegeneration.Repository) {
	r.generations = generations
}

func (r *Repository) List(ctx context.Context, query ListQuery) ([]Record, int64, error) {
	db := r.db.WithContext(ctx).Model(&Model{}).Where("setting_key NOT IN ?", dedicatedSettingKeys)
	if query.Keyword != "" {
		pattern := "%" + strings.ReplaceAll(strings.ReplaceAll(query.Keyword, "%", `\%`), "_", `\_`) + "%"
		db = db.Where("(setting_key LIKE ? ESCAPE '\\' OR description LIKE ? ESCAPE '\\')", pattern, pattern)
	}
	if query.IsEnabled != nil {
		db = db.Where("is_enabled = ?", *query.IsEnabled)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count settings: %w", err)
	}
	var rows []Model
	if err := db.Order("setting_key ASC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list settings: %w", err)
	}
	items := make([]Record, 0, len(rows))
	for _, row := range rows {
		items = append(items, recordFromModel(row))
	}
	return items, total, nil
}

func (r *Repository) Find(ctx context.Context, key string) (Record, error) {
	var row Model
	if err := r.db.WithContext(ctx).Where("setting_key = ?", key).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Record{}, ErrNotFound
		}
		return Record{}, err
	}
	return recordFromModel(row), nil
}

// Create 插入业务行并在同一事务内推进 generation 与 outbox。
func (r *Repository) Create(ctx context.Context, value *Record, expected int64) (cachegeneration.MutationResult, error) {
	return r.mutate(ctx, expected, value.CreatedAt, func(tx *gorm.DB) (bool, error) {
		row := Model{
			Key: value.Key, Value: value.Value, ValueType: value.ValueType, Description: value.Description,
			IsEnabled: value.IsEnabled, IsBuiltin: value.IsBuiltin, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		}
		if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return false, ErrConflict
			}
			return false, err
		}
		value.ID = row.ID
		return true, nil
	})
}

// Update 锁定业务行；内容完全一致时不推进 generation。
func (r *Repository) Update(ctx context.Context, key string, value Record, expected int64) (cachegeneration.MutationResult, error) {
	return r.mutate(ctx, expected, value.UpdatedAt, func(tx *gorm.DB) (bool, error) {
		row, err := lockSettingRow(ctx, tx, key)
		if err != nil {
			return false, err
		}
		if row.Value == value.Value && row.ValueType == value.ValueType && row.Description == value.Description {
			return false, nil
		}
		result := tx.WithContext(ctx).Model(&Model{}).Where("id = ?", row.ID).Updates(map[string]any{
			"value": value.Value, "value_type": value.ValueType, "description": value.Description, "updated_at": value.UpdatedAt,
		})
		if result.Error != nil {
			return false, result.Error
		}
		if result.RowsAffected != 1 {
			return false, fmt.Errorf("update setting affected %d rows", result.RowsAffected)
		}
		return true, nil
	})
}

// UpdateStatus 锁定业务行；状态一致时不推进 generation。
func (r *Repository) UpdateStatus(ctx context.Context, key string, status yesno.Value, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		row, err := lockSettingRow(ctx, tx, key)
		if err != nil {
			return false, err
		}
		if row.IsEnabled == status {
			return false, nil
		}
		result := tx.WithContext(ctx).Model(&Model{}).Where("id = ?", row.ID).Updates(map[string]any{
			"is_enabled": status, "updated_at": now,
		})
		if result.Error != nil {
			return false, result.Error
		}
		if result.RowsAffected != 1 {
			return false, fmt.Errorf("update setting status affected %d rows", result.RowsAffected)
		}
		return true, nil
	})
}

// Delete 锁定业务行后软删并在同一事务内推进 generation 与 outbox。
func (r *Repository) Delete(ctx context.Context, key string, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		row, err := lockSettingRow(ctx, tx, key)
		if err != nil {
			return false, err
		}
		result := tx.WithContext(ctx).Model(&Model{}).Where("id = ?", row.ID).Updates(map[string]any{"deleted_at": now, "updated_at": now})
		if result.Error != nil {
			return false, result.Error
		}
		if result.RowsAffected != 1 {
			return false, fmt.Errorf("delete setting affected %d rows", result.RowsAffected)
		}
		return true, nil
	})
}

func (r *Repository) FindBrand(ctx context.Context) (BrandSettings, error) {
	var rows []Model
	keys := []string{BrandTitleZhCNKey, BrandTitleEnUSKey, BrandDefaultAvatarKey}
	if err := r.db.WithContext(ctx).Where("setting_key IN ? AND deleted_at IS NULL", keys).Find(&rows).Error; err != nil {
		return BrandSettings{}, err
	}
	values := make(map[string]string, len(rows))
	for _, row := range rows {
		if row.ValueType != ValueTypeString || row.IsEnabled != yesno.Yes {
			return BrandSettings{}, fmt.Errorf("brand setting unavailable")
		}
		values[row.Key] = row.Value
	}
	if len(values) != len(keys) {
		return BrandSettings{}, fmt.Errorf("brand setting unavailable")
	}
	return BrandSettings{
		TitleZhCN: values[BrandTitleZhCNKey], TitleEnUS: values[BrandTitleEnUSKey], DefaultAvatar: values[BrandDefaultAvatarKey],
	}, nil
}

// UpdateBrand 在同一事务内锁定并更新三行；任一行缺失整体回滚；
// 三行内容全部一致时不推进 generation。
func (r *Repository) UpdateBrand(ctx context.Context, brand BrandSettings, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		var rows []Model
		if err := tx.WithContext(ctx).Raw(
			`SELECT * FROM system_setting
			 WHERE setting_key IN (?, ?, ?) AND deleted_at IS NULL
			 ORDER BY setting_key
			 FOR UPDATE`,
			BrandDefaultAvatarKey, BrandTitleEnUSKey, BrandTitleZhCNKey).Scan(&rows).Error; err != nil {
			return false, err
		}
		if len(rows) != 3 {
			return false, ErrNotFound
		}
		values := map[string]string{
			BrandTitleZhCNKey: brand.TitleZhCN, BrandTitleEnUSKey: brand.TitleEnUS, BrandDefaultAvatarKey: brand.DefaultAvatar,
		}
		changed := false
		for _, row := range rows {
			if row.Value != values[row.Key] || row.ValueType != ValueTypeString || row.IsEnabled != yesno.Yes || row.IsBuiltin != yesno.Yes {
				changed = true
				break
			}
		}
		if !changed {
			return false, nil
		}
		for _, row := range rows {
			result := tx.WithContext(ctx).Model(&Model{}).Where("id = ?", row.ID).Updates(map[string]any{
				"value": values[row.Key], "value_type": ValueTypeString, "is_enabled": yesno.Yes, "is_builtin": yesno.Yes, "updated_at": now,
			})
			if result.Error != nil {
				return false, result.Error
			}
			if result.RowsAffected != 1 {
				return false, ErrNotFound
			}
		}
		return true, nil
	})
}

// mutate 是私有的业务事务 helper：只执行显式业务变更，并在真实变化时
// 于同一事务内校验 lease base generation 并推进 generation/outbox。
func (r *Repository) mutate(ctx context.Context, expected int64, now time.Time, apply func(tx *gorm.DB) (bool, error)) (cachegeneration.MutationResult, error) {
	if r == nil || r.db == nil {
		return cachegeneration.MutationResult{}, fmt.Errorf("setting repository is not configured")
	}
	if r.generations == nil {
		return cachegeneration.MutationResult{}, fmt.Errorf("setting repository requires cache generations")
	}
	if expected < 1 {
		return cachegeneration.MutationResult{}, fmt.Errorf("setting mutation requires an expected generation")
	}
	result := cachegeneration.MutationResult{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		changed, err := apply(tx)
		if err != nil {
			return fmt.Errorf("%w: %w", errMutationRolledBack, err)
		}
		if !changed {
			return nil
		}
		event, err := r.generations.AdvanceTx(ctx, tx, r.scope, expected, now)
		if err != nil {
			return fmt.Errorf("%w: %w", errMutationRolledBack, err)
		}
		result.Generation = event.Generation
		result.OutboxID = event.ID
		result.Changed = true
		return nil
	})
	if err != nil {
		return cachegeneration.MutationResult{}, err
	}
	return result, nil
}

func lockSettingRow(ctx context.Context, tx *gorm.DB, key string) (Model, error) {
	var row Model
	if err := tx.WithContext(ctx).Raw(
		`SELECT * FROM system_setting WHERE setting_key = ? AND deleted_at IS NULL FOR UPDATE`, key).Scan(&row).Error; err != nil {
		return Model{}, err
	}
	if row.ID == 0 {
		return Model{}, ErrNotFound
	}
	return row, nil
}
