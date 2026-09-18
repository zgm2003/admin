package dictionary

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

var errDictionaryMutationRolledBack = errors.New("dictionary mutation transaction rolled back")

type optionFact struct {
	Code              string      `gorm:"column:code"`
	DictionaryEnabled yesno.Value `gorm:"column:dictionary_enabled"`
	ItemID            *int64      `gorm:"column:item_id"`
	Value             *string     `gorm:"column:value"`
	LabelZH           *string     `gorm:"column:label_zh"`
	LabelEN           *string     `gorm:"column:label_en"`
}

type Repository struct {
	db          *gorm.DB
	generations *cachegeneration.Repository
	scope       cachegeneration.Scope
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db, scope: dictionaryGenerationScope}
}

func (r *Repository) SetGenerations(repository *cachegeneration.Repository, scope cachegeneration.Scope) {
	r.generations = repository
	r.scope = scope
}

func (r *Repository) List(ctx context.Context, query ListQuery) ([]ListItem, int64, error) {
	db := r.db.WithContext(ctx).Model(&Dictionary{})
	if query.Keyword != "" {
		pattern := "%" + strings.ReplaceAll(strings.ReplaceAll(query.Keyword, "%", `\%`), "_", `\_`) + "%"
		db = db.Where("(code LIKE ? ESCAPE '\\' OR name_zh LIKE ? ESCAPE '\\' OR name_en LIKE ? ESCAPE '\\')", pattern, pattern, pattern)
	}
	if query.IsEnabled != nil {
		db = db.Where("is_enabled = ?", *query.IsEnabled)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count dictionaries: %w", err)
	}
	rows := make([]ListItem, 0, query.PageSize)
	if err := db.Select("system_dictionary.*, COALESCE(item_counts.item_count, 0) AS item_count").
		Joins("LEFT JOIN (SELECT dictionary_id, COUNT(*) AS item_count FROM system_dictionary_item WHERE deleted_at IS NULL GROUP BY dictionary_id) AS item_counts ON item_counts.dictionary_id = system_dictionary.id").
		Order("system_dictionary.code ASC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list dictionaries: %w", err)
	}
	return rows, total, nil
}

func (r *Repository) Find(ctx context.Context, id int64) (Dictionary, error) {
	var value Dictionary
	if err := r.db.WithContext(ctx).First(&value, id).Error; err != nil {
		return Dictionary{}, err
	}
	return value, nil
}

func (r *Repository) FindByCode(ctx context.Context, code string) (Dictionary, error) {
	var value Dictionary
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&value).Error; err != nil {
		return Dictionary{}, err
	}
	return value, nil
}

func (r *Repository) Items(ctx context.Context, dictionaryID int64, enabledOnly bool) ([]Item, error) {
	db := r.db.WithContext(ctx).Where("dictionary_id = ?", dictionaryID)
	if enabledOnly {
		db = db.Where("is_enabled = 1")
	}
	var values []Item
	if err := db.Order("sort ASC, id ASC").Find(&values).Error; err != nil {
		return nil, fmt.Errorf("list dictionary items: %w", err)
	}
	return values, nil
}

func (r *Repository) Options(ctx context.Context, codes []string) ([]optionFact, error) {
	var rows []optionFact
	if err := r.db.WithContext(ctx).Raw(`
SELECT d.code,
       d.is_enabled AS dictionary_enabled,
       i.id AS item_id,
       i.value,
       i.label_zh,
       i.label_en
FROM system_dictionary AS d
LEFT JOIN system_dictionary_item AS i
  ON i.dictionary_id = d.id
 AND i.deleted_at IS NULL
 AND i.is_enabled = 1
WHERE d.code IN ?
  AND d.deleted_at IS NULL
ORDER BY d.code ASC, i.sort ASC, i.id ASC`, codes).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load dictionary options: %w", err)
	}
	return rows, nil
}

func (r *Repository) Create(ctx context.Context, value *Dictionary, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	if value == nil {
		return cachegeneration.MutationResult{}, fmt.Errorf("dictionary is required")
	}
	originalID := value.ID
	result, err := r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		return true, tx.WithContext(ctx).Create(value).Error
	})
	if err != nil {
		value.ID = originalID
	}
	return result, err
}

func (r *Repository) Update(ctx context.Context, id int64, input UpdateInput, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		row, err := lockDictionary(ctx, tx, id)
		if err != nil {
			return false, err
		}
		if row.NameZH == input.NameZH && row.NameEN == input.NameEN && row.Description == input.Description {
			return false, nil
		}
		result := tx.WithContext(ctx).Model(&Dictionary{}).Where("id = ?", id).Updates(map[string]any{
			"name_zh": input.NameZH, "name_en": input.NameEN, "description": input.Description, "updated_at": now,
		})
		return result.RowsAffected == 1, result.Error
	})
}

func (r *Repository) UpdateStatus(ctx context.Context, id int64, status yesno.Value, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		row, err := lockDictionary(ctx, tx, id)
		if err != nil {
			return false, err
		}
		if row.IsEnabled == status {
			return false, nil
		}
		result := tx.WithContext(ctx).Model(&Dictionary{}).Where("id = ?", id).Updates(map[string]any{"is_enabled": status, "updated_at": now})
		return result.RowsAffected == 1, result.Error
	})
}

func (r *Repository) Delete(ctx context.Context, id int64, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		row, err := lockDictionary(ctx, tx, id)
		if err != nil {
			return false, err
		}
		if row.IsBuiltin == yesno.Yes {
			return false, ErrConflict
		}
		var count int64
		if err := tx.WithContext(ctx).Model(&Item{}).Where("dictionary_id = ?", id).Count(&count).Error; err != nil {
			return false, err
		}
		if count > 0 {
			return false, ErrConflict
		}
		result := tx.WithContext(ctx).Model(&Dictionary{}).Where("id = ?", id).Updates(map[string]any{"deleted_at": now, "updated_at": now})
		return result.RowsAffected == 1, result.Error
	})
}

func (r *Repository) CreateItem(ctx context.Context, value *Item, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	if value == nil {
		return cachegeneration.MutationResult{}, fmt.Errorf("dictionary item is required")
	}
	originalID := value.ID
	result, err := r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		parent, err := lockDictionary(ctx, tx, value.DictionaryID)
		if err != nil {
			return false, err
		}
		if parent.IsEnabled != yesno.Yes {
			return false, ErrConflict
		}
		return true, tx.WithContext(ctx).Create(value).Error
	})
	if err != nil {
		value.ID = originalID
	}
	return result, err
}

func (r *Repository) FindItemByValue(ctx context.Context, dictionaryID int64, value string) (Item, error) {
	var item Item
	err := r.db.WithContext(ctx).Where("dictionary_id = ? AND value = ?", dictionaryID, value).First(&item).Error
	return item, err
}

func (r *Repository) FindItem(ctx context.Context, dictionaryID, itemID int64) (Item, error) {
	var value Item
	if err := r.db.WithContext(ctx).Where("dictionary_id = ? AND id = ?", dictionaryID, itemID).First(&value).Error; err != nil {
		return Item{}, err
	}
	return value, nil
}

func (r *Repository) UpdateItem(ctx context.Context, dictionaryID, itemID int64, input UpdateItemInput, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		if _, err := lockDictionary(ctx, tx, dictionaryID); err != nil {
			return false, err
		}
		row, err := lockDictionaryItem(ctx, tx, dictionaryID, itemID)
		if err != nil {
			return false, err
		}
		if row.LabelZH == input.LabelZH && row.LabelEN == input.LabelEN && row.Sort == input.Sort {
			return false, nil
		}
		result := tx.WithContext(ctx).Model(&Item{}).Where("dictionary_id = ? AND id = ?", dictionaryID, itemID).Updates(map[string]any{
			"label_zh": input.LabelZH, "label_en": input.LabelEN, "sort": input.Sort, "updated_at": now,
		})
		return result.RowsAffected == 1, result.Error
	})
}

func (r *Repository) UpdateItemStatus(ctx context.Context, dictionaryID, itemID int64, status yesno.Value, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		if _, err := lockDictionary(ctx, tx, dictionaryID); err != nil {
			return false, err
		}
		row, err := lockDictionaryItem(ctx, tx, dictionaryID, itemID)
		if err != nil {
			return false, err
		}
		if row.IsEnabled == status {
			return false, nil
		}
		result := tx.WithContext(ctx).Model(&Item{}).Where("dictionary_id = ? AND id = ?", dictionaryID, itemID).Updates(map[string]any{
			"is_enabled": status, "updated_at": now,
		})
		return result.RowsAffected == 1, result.Error
	})
}

func (r *Repository) DeleteItem(ctx context.Context, dictionaryID, itemID int64, expected int64, now time.Time) (cachegeneration.MutationResult, error) {
	return r.mutate(ctx, expected, now, func(tx *gorm.DB) (bool, error) {
		if _, err := lockDictionary(ctx, tx, dictionaryID); err != nil {
			return false, err
		}
		row, err := lockDictionaryItem(ctx, tx, dictionaryID, itemID)
		if err != nil {
			return false, err
		}
		if row.IsBuiltin == yesno.Yes {
			return false, ErrConflict
		}
		result := tx.WithContext(ctx).Model(&Item{}).Where("dictionary_id = ? AND id = ?", dictionaryID, itemID).Updates(map[string]any{
			"deleted_at": now, "updated_at": now,
		})
		return result.RowsAffected == 1, result.Error
	})
}

func (r *Repository) CountItems(ctx context.Context, dictionaryID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Item{}).Where("dictionary_id = ?", dictionaryID).Count(&count).Error
	return count, err
}

func (r *Repository) mutate(ctx context.Context, expected int64, now time.Time, apply func(*gorm.DB) (bool, error)) (cachegeneration.MutationResult, error) {
	if r == nil || r.db == nil || r.generations == nil {
		return cachegeneration.MutationResult{}, fmt.Errorf("dictionary repository generation dependencies are not configured")
	}
	if err := r.scope.Validate(); err != nil {
		return cachegeneration.MutationResult{}, err
	}
	if expected < 1 {
		return cachegeneration.MutationResult{}, fmt.Errorf("dictionary mutation expected generation is invalid")
	}
	result := cachegeneration.MutationResult{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		changed, err := apply(tx)
		if err != nil {
			return fmt.Errorf("%w: %w", errDictionaryMutationRolledBack, mapRepositoryError(err))
		}
		if !changed {
			return nil
		}
		event, err := r.generations.AdvanceTx(ctx, tx, r.scope, expected, now)
		if err != nil {
			return fmt.Errorf("%w: %w", errDictionaryMutationRolledBack, err)
		}
		result = cachegeneration.MutationResult{Changed: true, Generation: event.Generation, OutboxID: event.ID}
		return nil
	})
	if err != nil {
		return cachegeneration.MutationResult{}, mapRepositoryError(err)
	}
	return result, nil
}

func lockDictionary(ctx context.Context, tx *gorm.DB, id int64) (Dictionary, error) {
	var row Dictionary
	if err := tx.WithContext(ctx).Raw(`SELECT * FROM system_dictionary WHERE id = ? AND deleted_at IS NULL FOR UPDATE`, id).Scan(&row).Error; err != nil {
		return Dictionary{}, err
	}
	if row.ID == 0 {
		return Dictionary{}, gorm.ErrRecordNotFound
	}
	return row, nil
}

func lockDictionaryItem(ctx context.Context, tx *gorm.DB, dictionaryID, itemID int64) (Item, error) {
	var row Item
	if err := tx.WithContext(ctx).Raw(`
SELECT * FROM system_dictionary_item
WHERE dictionary_id = ? AND id = ? AND deleted_at IS NULL
FOR UPDATE`, dictionaryID, itemID).Scan(&row).Error; err != nil {
		return Item{}, err
	}
	if row.ID == 0 {
		return Item{}, gorm.ErrRecordNotFound
	}
	return row, nil
}

func mapRepositoryError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return fmt.Errorf("%w: %v", ErrConflict, err)
	}
	return err
}
