package dictionary

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

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

func (r *Repository) Create(ctx context.Context, value *Dictionary) error {
	err := r.db.WithContext(ctx).Create(&value).Error
	return mapRepositoryError(err)
}
func (r *Repository) Update(ctx context.Context, id int64, input UpdateInput, now time.Time) error {
	return r.db.WithContext(ctx).Model(&Dictionary{}).Where("id = ?", id).Updates(map[string]interface{}{"name_zh": input.NameZH, "name_en": input.NameEN, "description": input.Description, "updated_at": now}).Error
}
func (r *Repository) UpdateStatus(ctx context.Context, id int64, status int16, now time.Time) error {
	return r.db.WithContext(ctx).Model(&Dictionary{}).Where("id = ?", id).Updates(map[string]interface{}{"is_enabled": status, "updated_at": now}).Error
}
func (r *Repository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&Dictionary{}, id).Error
}
func (r *Repository) CreateItem(ctx context.Context, value Item) error {
	err := r.db.WithContext(ctx).Create(&value).Error
	return mapRepositoryError(err)
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

func mapRepositoryError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}
func (r *Repository) UpdateItem(ctx context.Context, dictionaryID, itemID int64, input UpdateItemInput, now time.Time) error {
	return r.db.WithContext(ctx).Model(&Item{}).Where("dictionary_id = ? AND id = ?", dictionaryID, itemID).Updates(map[string]interface{}{"label_zh": input.LabelZH, "label_en": input.LabelEN, "sort": input.Sort, "updated_at": now}).Error
}
func (r *Repository) UpdateItemStatus(ctx context.Context, dictionaryID, itemID int64, status int16, now time.Time) error {
	return r.db.WithContext(ctx).Model(&Item{}).Where("dictionary_id = ? AND id = ?", dictionaryID, itemID).Updates(map[string]interface{}{"is_enabled": status, "updated_at": now}).Error
}
func (r *Repository) DeleteItem(ctx context.Context, dictionaryID, itemID int64) error {
	return r.db.WithContext(ctx).Where("dictionary_id = ? AND id = ?", dictionaryID, itemID).Delete(&Item{}).Error
}
func (r *Repository) CountItems(ctx context.Context, dictionaryID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Item{}).Where("dictionary_id = ?", dictionaryID).Count(&count).Error
	return count, err
}
