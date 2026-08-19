package menu

import (
	"context"
	"errors"
	"fmt"
	"time"

	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrMenuCodeConflict = errors.New("active menu code conflicts")
	ErrMenuPathConflict = errors.New("active menu page path conflicts")
)

type UpdateValues struct {
	ParentID  *int64
	MenuType  Type
	I18nKey   string
	Path      *string
	ViewKey   *string
	Icon      *string
	SortOrder int
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Transaction(ctx context.Context, fn func(*Repository) error) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("menu transaction requires a database")
	}
	if fn == nil {
		return fmt.Errorf("menu transaction requires a callback")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(NewRepository(tx))
	})
}

func (r *Repository) LockMenuTableForBuiltin(ctx context.Context) error {
	if err := r.db.WithContext(ctx).Exec("LOCK TABLE sys_menu IN SHARE ROW EXCLUSIVE MODE").Error; err != nil {
		return fmt.Errorf("lock menu table for builtin initialization: %w", err)
	}
	return nil
}

func (r *Repository) FindBuiltinRecords(ctx context.Context, codes []string) ([]Menu, error) {
	var rows []Menu
	if err := r.db.WithContext(ctx).Unscoped().Where("code IN ?", codes).Order("code, id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("find builtin menu records: %w", err)
	}
	return rows, nil
}

func (r *Repository) Create(ctx context.Context, value *Menu) error {
	var created struct {
		ID        int64
		CreatedAt time.Time
		UpdatedAt time.Time
	}
	result := r.db.WithContext(ctx).Raw(`
		INSERT INTO sys_menu (
			parent_id, menu_type, code, i18n_key, path, view_key, icon, sort_order, is_enabled
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, created_at, updated_at`,
		value.ParentID, value.MenuType, value.Code, value.I18nKey, value.Path,
		value.ViewKey, value.Icon, value.SortOrder, value.IsEnabled,
	).Scan(&created)
	if result.Error != nil {
		return mapMenuWriteError("create menu", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("create menu: expected one inserted row, got %d", result.RowsAffected)
	}
	value.ID = created.ID
	value.CreatedAt = created.CreatedAt
	value.UpdatedAt = created.UpdatedAt
	return nil
}

func (r *Repository) FindActiveMenus(ctx context.Context) ([]Menu, error) {
	var rows []Menu
	if err := r.db.WithContext(ctx).Order("sort_order, code, id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("find active menus: %w", err)
	}
	return rows, nil
}

func (r *Repository) LockActiveMenus(ctx context.Context) ([]Menu, error) {
	var rows []Menu
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Order("sort_order, code, id").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("lock active menus: %w", err)
	}
	return rows, nil
}

func (r *Repository) UpdateMenu(ctx context.Context, id int64, values UpdateValues, updatedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&Menu{}).Where("id = ?", id).Updates(map[string]any{
		"parent_id":  values.ParentID,
		"menu_type":  values.MenuType,
		"i18n_key":   values.I18nKey,
		"path":       values.Path,
		"view_key":   values.ViewKey,
		"icon":       values.Icon,
		"sort_order": values.SortOrder,
		"updated_at": updatedAt,
	})
	if result.Error != nil {
		return mapMenuWriteError("update menu", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("update menu %d: %w", id, gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *Repository) UpdateMenuStatus(ctx context.Context, ids []int64, value yesno.Value, updatedAt time.Time) error {
	if len(ids) == 0 {
		return fmt.Errorf("update menu status requires at least one id")
	}
	if err := r.db.WithContext(ctx).Model(&Menu{}).Where("id IN ?", ids).Updates(map[string]any{
		"is_enabled": value,
		"updated_at": updatedAt,
	}).Error; err != nil {
		return fmt.Errorf("update menu status: %w", err)
	}
	return nil
}

func (r *Repository) HasActiveDirectGrant(ctx context.Context, menuID int64) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&RoleMenu{}).Where("menu_id = ?", menuID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("inspect active direct menu grant: %w", err)
	}
	return count > 0, nil
}

func (r *Repository) SoftDeleteRoleMenus(ctx context.Context, menuIDs []int64, deletedAt time.Time) error {
	if len(menuIDs) == 0 {
		return fmt.Errorf("soft delete role menus requires at least one menu id")
	}
	if err := r.db.WithContext(ctx).Model(&RoleMenu{}).Where("menu_id IN ?", menuIDs).Updates(map[string]any{
		"updated_at": deletedAt.UTC(),
		"deleted_at": deletedAt.UTC(),
	}).Error; err != nil {
		return fmt.Errorf("soft delete role menus: %w", err)
	}
	return nil
}

func (r *Repository) SoftDeleteMenus(ctx context.Context, menuIDs []int64, deletedAt time.Time) error {
	if len(menuIDs) == 0 {
		return fmt.Errorf("soft delete menus requires at least one id")
	}
	if err := r.db.WithContext(ctx).Model(&Menu{}).Where("id IN ?", menuIDs).Updates(map[string]any{
		"updated_at": deletedAt.UTC(),
		"deleted_at": deletedAt.UTC(),
	}).Error; err != nil {
		return fmt.Errorf("soft delete menus: %w", err)
	}
	return nil
}

func mapMenuWriteError(operation string, err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.ConstraintName {
		case "ux_sys_menu_code_active":
			return fmt.Errorf("%s: %w: %w", operation, ErrMenuCodeConflict, err)
		case "ux_sys_menu_page_path_active":
			return fmt.Errorf("%s: %w: %w", operation, ErrMenuPathConflict, err)
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}
