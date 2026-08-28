package menu

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"admin/server/internal/module/accessstate"
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
	ParentID      *int64
	MenuType      Type
	Name          string
	I18nKey       *string
	Path          *string
	ComponentPath *string
	Icon          *string
	SortOrder     int
	IsHidden      yesno.Value
}

type PlatformOption struct {
	ID        int64
	Code      string
	Name      string
	IsEnabled yesno.Value
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

func (r *Repository) Create(ctx context.Context, value *Menu) error {
	var created struct {
		ID        int64
		CreatedAt time.Time
		UpdatedAt time.Time
	}
	result := r.db.WithContext(ctx).Raw(`
		INSERT INTO rbac_menu (
			platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, icon, sort_order, is_enabled, is_hidden
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, created_at, updated_at`,
		value.PlatformID, value.ParentID, value.MenuType, value.Name, value.Code, value.I18nKey, value.Path,
		value.ComponentPath, value.Icon, value.SortOrder, value.IsEnabled, value.IsHidden,
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

func (r *Repository) FindPlatformOptions(ctx context.Context) ([]PlatformOption, error) {
	rows := make([]PlatformOption, 0)
	if err := r.db.WithContext(ctx).Table("auth_platform").
		Select("id, code, name, is_enabled").
		Where("deleted_at IS NULL").
		Order("is_builtin DESC, code ASC, id ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("find menu platform options: %w", err)
	}
	return rows, nil
}

func (r *Repository) FindPlatform(ctx context.Context, id int64) (PlatformOption, error) {
	var found PlatformOption
	if err := r.db.WithContext(ctx).Table("auth_platform").
		Select("id, code, name, is_enabled").
		Where("id = ? AND deleted_at IS NULL", id).
		Take(&found).Error; err != nil {
		return PlatformOption{}, fmt.Errorf("find menu platform: %w", err)
	}
	return found, nil
}

func (r *Repository) LockPlatform(ctx context.Context, id int64) (PlatformOption, error) {
	var found PlatformOption
	if err := r.db.WithContext(ctx).Table("auth_platform").
		Select("id, code, name, is_enabled").
		Clauses(clause.Locking{Strength: "SHARE"}).
		Where("id = ? AND deleted_at IS NULL", id).
		Take(&found).Error; err != nil {
		return PlatformOption{}, fmt.Errorf("lock menu platform: %w", err)
	}
	return found, nil
}

func (r *Repository) FindActiveMenus(ctx context.Context, platformID *int64) ([]Menu, error) {
	var rows []Menu
	query := r.db.WithContext(ctx)
	if platformID != nil {
		query = query.Where("platform_id = ?", *platformID)
	}
	if err := query.Order("sort_order, code, id").Find(&rows).Error; err != nil {
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

func (r *Repository) CountAllMenusForPlatform(ctx context.Context, platformID int64) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Unscoped().Model(&Menu{}).Where("platform_id = ?", platformID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count all platform menus: %w", err)
	}
	return count, nil
}

func (r *Repository) LockMenusByCodesUnscoped(ctx context.Context, platformID int64, codes []string) ([]Menu, error) {
	rows := make([]Menu, 0, len(codes))
	if len(codes) == 0 {
		return rows, nil
	}
	if err := r.db.WithContext(ctx).Unscoped().
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("platform_id = ? AND code IN ?", platformID, codes).
		Order("code, id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("lock menus by codes: %w", err)
	}
	return rows, nil
}

func (r *Repository) UpdateFoundationStructure(ctx context.Context, id int64, value Menu, updatedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&Menu{}).Where("id = ?", id).Updates(map[string]any{
		"parent_id": value.ParentID, "menu_type": value.MenuType, "path": value.Path,
		"component_path": value.ComponentPath, "is_enabled": value.IsEnabled,
		"is_hidden": value.IsHidden, "updated_at": updatedAt.UTC(),
	})
	if result.Error != nil {
		return fmt.Errorf("update foundation menu structure: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("update foundation menu structure %d: %w", id, gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *Repository) RestoreFoundationMenu(ctx context.Context, id int64, value Menu, updatedAt time.Time) error {
	result := r.db.WithContext(ctx).Unscoped().Model(&Menu{}).Where("id = ?", id).Updates(map[string]any{
		"parent_id": value.ParentID, "menu_type": value.MenuType, "name": value.Name, "code": value.Code,
		"i18n_key": value.I18nKey, "path": value.Path, "component_path": value.ComponentPath,
		"icon": value.Icon, "sort_order": value.SortOrder, "is_enabled": value.IsEnabled,
		"is_hidden": value.IsHidden, "updated_at": updatedAt.UTC(), "deleted_at": nil,
	})
	if result.Error != nil {
		return mapMenuWriteError("restore foundation menu", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("restore foundation menu %d: %w", id, gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *Repository) FindActiveAccessVersions(ctx context.Context) ([]accessstate.Version, error) {
	versions := make([]accessstate.Version, 0)
	if err := r.db.WithContext(ctx).Raw(`
		SELECT app_user.id AS user_id, COALESCE(access_version.version, 0) AS version
		FROM user_account AS app_user
		LEFT JOIN rbac_access_version AS access_version
		  ON access_version.user_id = app_user.id
		WHERE app_user.deleted_at IS NULL
		  AND app_user.is_enabled = ?
		ORDER BY app_user.id ASC`, yesno.Yes).Scan(&versions).Error; err != nil {
		return nil, fmt.Errorf("find active menu access versions: %w", err)
	}
	if err := validateMenuAccessVersions(versions); err != nil {
		return nil, fmt.Errorf("find active menu access versions: %w", err)
	}
	return versions, nil
}

func (r *Repository) LockUserMutationTables(ctx context.Context) error {
	if err := r.db.WithContext(ctx).Exec("LOCK TABLE user_account IN SHARE ROW EXCLUSIVE MODE").Error; err != nil {
		return fmt.Errorf("lock user table for menu mutation: %w", err)
	}
	return nil
}

func (r *Repository) LockActiveAccessVersions(ctx context.Context) ([]accessstate.Version, error) {
	versions := make([]accessstate.Version, 0)
	if err := r.db.WithContext(ctx).Raw(`
		SELECT app_user.id AS user_id, access_version.version
		FROM user_account AS app_user
		JOIN rbac_access_version AS access_version
		  ON access_version.user_id = app_user.id
		WHERE app_user.deleted_at IS NULL
		  AND app_user.is_enabled = ?
		ORDER BY app_user.id ASC
		FOR UPDATE OF app_user, access_version`, yesno.Yes).Scan(&versions).Error; err != nil {
		return nil, fmt.Errorf("lock active menu access versions: %w", err)
	}
	if err := validateMenuAccessVersions(versions); err != nil {
		return nil, fmt.Errorf("lock active menu access versions: %w", err)
	}
	return versions, nil
}

func (r *Repository) IncrementAccessVersions(ctx context.Context, userIDs []int64, now time.Time) (map[int64]int64, error) {
	userIDs, err := normalizeMenuAccessUserIDs(userIDs)
	if err != nil {
		return nil, err
	}
	if len(userIDs) == 0 {
		return map[int64]int64{}, nil
	}
	advanced := make([]accessstate.Version, 0, len(userIDs))
	if err := r.db.WithContext(ctx).Raw(`
		UPDATE rbac_access_version
		SET version = version + 1, updated_at = ?
		WHERE user_id IN ?
		RETURNING user_id, version`, now.UTC(), userIDs).Scan(&advanced).Error; err != nil {
		return nil, fmt.Errorf("increment menu access versions: %w", err)
	}
	sortMenuAccessVersions(advanced)
	if err := validateMenuAccessVersions(advanced); err != nil {
		return nil, fmt.Errorf("increment menu access versions: %w", err)
	}
	result := make(map[int64]int64, len(advanced))
	for _, version := range advanced {
		result[version.UserID] = version.Version
	}
	if len(result) != len(userIDs) {
		return nil, fmt.Errorf("increment menu access versions returned %d users, want %d", len(result), len(userIDs))
	}
	for _, userID := range userIDs {
		if result[userID] < 2 {
			return nil, fmt.Errorf("increment menu access version for user %d is missing", userID)
		}
	}
	return result, nil
}

func sortMenuAccessVersions(versions []accessstate.Version) {
	sort.Slice(versions, func(left, right int) bool {
		return versions[left].UserID < versions[right].UserID
	})
}

func validateMenuAccessVersions(versions []accessstate.Version) error {
	previousUserID := int64(0)
	for index, version := range versions {
		if version.UserID < 1 || version.Version < 1 {
			return fmt.Errorf("menu access version at index %d is invalid", index)
		}
		if index > 0 && version.UserID <= previousUserID {
			return fmt.Errorf("menu access versions are not unique and sorted")
		}
		previousUserID = version.UserID
	}
	return nil
}

func normalizeMenuAccessUserIDs(userIDs []int64) ([]int64, error) {
	normalized := append([]int64(nil), userIDs...)
	sort.Slice(normalized, func(left, right int) bool { return normalized[left] < normalized[right] })
	result := normalized[:0]
	for _, userID := range normalized {
		if userID < 1 {
			return nil, fmt.Errorf("menu access user id is invalid")
		}
		if len(result) == 0 || result[len(result)-1] != userID {
			result = append(result, userID)
		}
	}
	return result, nil
}

func (r *Repository) UpdateMenu(ctx context.Context, id int64, values UpdateValues, updatedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&Menu{}).Where("id = ?", id).Updates(map[string]any{
		"parent_id":      values.ParentID,
		"menu_type":      values.MenuType,
		"name":           values.Name,
		"i18n_key":       values.I18nKey,
		"path":           values.Path,
		"component_path": values.ComponentPath,
		"icon":           values.Icon,
		"sort_order":     values.SortOrder,
		"is_hidden":      values.IsHidden,
		"updated_at":     updatedAt.UTC(),
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
		case "ux_rbac_menu_code_active":
			return fmt.Errorf("%s: %w: %w", operation, ErrMenuCodeConflict, err)
		case "ux_rbac_menu_page_path_active":
			return fmt.Errorf("%s: %w: %w", operation, ErrMenuPathConflict, err)
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}
