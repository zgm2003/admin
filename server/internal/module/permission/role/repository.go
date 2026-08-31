package role

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"admin/server/internal/module/permission/menu"
	"admin/server/internal/module/permission/state"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

var (
	ErrRoleCodeConflict = errors.New("active role code conflicts")
	ErrRoleNameConflict = errors.New("active role name conflicts")
)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Transaction(ctx context.Context, fn func(*Repository) error) error {
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(NewRepository(tx))
	}); err != nil {
		return fmt.Errorf("role transaction: %w", err)
	}
	return nil
}

func (r *Repository) LockRoleTable(ctx context.Context) error {
	if err := r.db.WithContext(ctx).Exec("LOCK TABLE permission_role IN SHARE ROW EXCLUSIVE MODE").Error; err != nil {
		return fmt.Errorf("lock role table: %w", err)
	}
	return nil
}

func (r *Repository) FindSystemRoleRecords(ctx context.Context) ([]Role, error) {
	var records []Role
	if err := r.db.WithContext(ctx).Unscoped().
		Where("code IN ?", []string{CodeSuperAdmin, CodeRegisteredUser}).
		Order("id ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("find system role records: %w", err)
	}
	return records, nil
}

func (r *Repository) LockActiveRoles(ctx context.Context) ([]Role, error) {
	var records []Role
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Order("id ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("lock active roles: %w", err)
	}
	return records, nil
}

func (r *Repository) Create(ctx context.Context, value *Role) error {
	if err := r.db.WithContext(ctx).Create(value).Error; err != nil {
		return mapRoleWriteError("create role", err)
	}
	return nil
}

func (r *Repository) LockActiveRole(ctx context.Context, id int64) (Role, error) {
	var found Role
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&found).Error; err != nil {
		return Role{}, fmt.Errorf("lock active role %d: %w", id, err)
	}
	return found, nil
}

func (r *Repository) FindActiveRole(ctx context.Context, id int64) (Role, error) {
	var found Role
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&found).Error; err != nil {
		return Role{}, fmt.Errorf("find active role %d: %w", id, err)
	}
	return found, nil
}

func (r *Repository) LockActiveMenus(ctx context.Context) ([]menu.Menu, error) {
	rows := make([]menu.Menu, 0)
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Order("sort_order ASC, code ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("lock active permission menus: %w", err)
	}
	return rows, nil
}

func (r *Repository) FindPermissionPlatforms(ctx context.Context) ([]PermissionPlatform, error) {
	type permissionPlatformRow struct {
		ID        int64
		Code      string
		Name      string
		IsEnabled yesno.Value
	}
	rows := make([]permissionPlatformRow, 0)
	if err := r.db.WithContext(ctx).Table("auth_platform").
		Select("id, code, name, is_enabled").
		Where("deleted_at IS NULL").
		Order("is_builtin DESC, code ASC, id ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("find role permission platforms: %w", err)
	}
	platforms := make([]PermissionPlatform, 0, len(rows))
	for _, row := range rows {
		platforms = append(platforms, PermissionPlatform{
			ID: row.ID, Code: row.Code, Name: row.Name, IsEnabled: row.IsEnabled,
		})
	}
	return platforms, nil
}

func (r *Repository) FindActiveRoleMenus(ctx context.Context, roleID int64) ([]menu.RoleMenu, error) {
	rows := make([]menu.RoleMenu, 0)
	if err := r.db.WithContext(ctx).Where("role_id = ?", roleID).Order("menu_id ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("find active role menus: %w", err)
	}
	return rows, nil
}

func (r *Repository) SoftDeleteRoleMenuIDs(ctx context.Context, ids []int64, deletedAt time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Table("permission_role_menu").Where("id IN ? AND deleted_at IS NULL", ids).Updates(map[string]any{
		"updated_at": deletedAt.UTC(), "deleted_at": deletedAt.UTC(),
	}).Error; err != nil {
		return fmt.Errorf("soft delete selected role menus: %w", err)
	}
	return nil
}

func (r *Repository) CreateRoleMenus(ctx context.Context, values []menu.RoleMenu) error {
	if len(values) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(&values).Error; err != nil {
		return fmt.Errorf("create role menus: %w", err)
	}
	return nil
}

func (r *Repository) TouchRole(ctx context.Context, roleID int64, updatedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&Role{}).Where("id = ?", roleID).Update("updated_at", updatedAt.UTC())
	if result.Error != nil {
		return fmt.Errorf("touch role: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("touch role %d: %w", roleID, gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *Repository) UpdateName(ctx context.Context, id int64, name string, updatedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&Role{}).Where("id = ?", id).Updates(map[string]any{
		"name": name, "updated_at": updatedAt.UTC(),
	})
	if result.Error != nil {
		return mapRoleWriteError("update role name", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("update role name %d: %w", id, gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id int64, value yesno.Value, updatedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&Role{}).Where("id = ?", id).Updates(map[string]any{
		"is_enabled": value, "updated_at": updatedAt.UTC(),
	})
	if result.Error != nil {
		return fmt.Errorf("update role status: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("update role status %d: %w", id, gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *Repository) ClearDefault(ctx context.Context, id int64, updatedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&Role{}).Where("id = ? AND is_default = ?", id, yesno.Yes).Updates(map[string]any{
		"is_default": yesno.No, "updated_at": updatedAt.UTC(),
	})
	if result.Error != nil {
		return fmt.Errorf("clear default role: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("clear default role %d: %w", id, gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *Repository) SetDefault(ctx context.Context, id int64, updatedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&Role{}).Where("id = ? AND is_default = ?", id, yesno.No).Updates(map[string]any{
		"is_default": yesno.Yes, "updated_at": updatedAt.UTC(),
	})
	if result.Error != nil {
		return fmt.Errorf("set default role: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("set default role %d: %w", id, gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *Repository) CountEffectiveUsers(ctx context.Context, roleID int64) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Table("permission_user_role AS user_role").
		Joins("JOIN user_account AS app_user ON app_user.id = user_role.user_id AND app_user.deleted_at IS NULL").
		Where("user_role.role_id = ? AND user_role.deleted_at IS NULL", roleID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count effective role users: %w", err)
	}
	return count, nil
}

func (r *Repository) FindEffectiveAccessVersionsByRole(ctx context.Context, roleID int64) ([]permissionstate.Version, error) {
	versions := make([]permissionstate.Version, 0)
	if err := r.db.WithContext(ctx).Raw(`
		SELECT app_user.id AS user_id, COALESCE(access_version.version, 0) AS version
		FROM permission_user_role AS user_role
		JOIN user_account AS app_user
		  ON app_user.id = user_role.user_id
		 AND app_user.deleted_at IS NULL
		 AND app_user.is_enabled = ?
		LEFT JOIN permission_access_version AS access_version
		  ON access_version.user_id = app_user.id
		WHERE user_role.role_id = ?
		  AND user_role.deleted_at IS NULL
		ORDER BY app_user.id ASC`, yesno.Yes, roleID).Scan(&versions).Error; err != nil {
		return nil, fmt.Errorf("find effective role access versions: %w", err)
	}
	if err := validateAccessVersions(versions); err != nil {
		return nil, fmt.Errorf("find effective role access versions: %w", err)
	}
	return versions, nil
}

func (r *Repository) LockEffectiveAccessVersionsByRole(ctx context.Context, roleID int64) ([]permissionstate.Version, error) {
	versions := make([]permissionstate.Version, 0)
	if err := r.db.WithContext(ctx).Raw(`
		SELECT app_user.id AS user_id, access_version.version
		FROM permission_user_role AS user_role
		JOIN user_account AS app_user
		  ON app_user.id = user_role.user_id
		 AND app_user.deleted_at IS NULL
		 AND app_user.is_enabled = ?
		JOIN permission_access_version AS access_version
		  ON access_version.user_id = app_user.id
		WHERE user_role.role_id = ?
		  AND user_role.deleted_at IS NULL
		ORDER BY app_user.id ASC
		FOR UPDATE OF app_user, access_version`, yesno.Yes, roleID).Scan(&versions).Error; err != nil {
		return nil, fmt.Errorf("lock effective role access versions: %w", err)
	}
	if err := validateAccessVersions(versions); err != nil {
		return nil, fmt.Errorf("lock effective role access versions: %w", err)
	}
	return versions, nil
}

func (r *Repository) IncrementAccessVersions(ctx context.Context, userIDs []int64, now time.Time) (map[int64]int64, error) {
	userIDs, err := normalizeAccessUserIDs(userIDs)
	if err != nil {
		return nil, err
	}
	if len(userIDs) == 0 {
		return map[int64]int64{}, nil
	}
	advanced := make([]permissionstate.Version, 0, len(userIDs))
	if err := r.db.WithContext(ctx).Raw(`
		UPDATE permission_access_version
		SET version = version + 1, updated_at = ?
		WHERE user_id IN ?
		RETURNING user_id, version`, now.UTC(), userIDs).Scan(&advanced).Error; err != nil {
		return nil, fmt.Errorf("increment role access versions: %w", err)
	}
	if err := validateAccessVersions(advanced); err != nil {
		return nil, fmt.Errorf("increment role access versions: %w", err)
	}
	result := make(map[int64]int64, len(advanced))
	for _, version := range advanced {
		result[version.UserID] = version.Version
	}
	if len(result) != len(userIDs) {
		return nil, fmt.Errorf("increment role access versions returned %d users, want %d", len(result), len(userIDs))
	}
	for _, userID := range userIDs {
		if result[userID] < 2 {
			return nil, fmt.Errorf("increment role access version for user %d is missing", userID)
		}
	}
	return result, nil
}

func validateAccessVersions(versions []permissionstate.Version) error {
	previousUserID := int64(0)
	for index, version := range versions {
		if version.UserID < 1 || version.Version < 1 {
			return fmt.Errorf("role access version at index %d is invalid", index)
		}
		if index > 0 && version.UserID <= previousUserID {
			return fmt.Errorf("role access versions are not unique and sorted")
		}
		previousUserID = version.UserID
	}
	return nil
}

func normalizeAccessUserIDs(userIDs []int64) ([]int64, error) {
	normalized := append([]int64(nil), userIDs...)
	sort.Slice(normalized, func(left, right int) bool { return normalized[left] < normalized[right] })
	result := normalized[:0]
	for _, userID := range normalized {
		if userID < 1 {
			return nil, fmt.Errorf("role access user id is invalid")
		}
		if len(result) == 0 || result[len(result)-1] != userID {
			result = append(result, userID)
		}
	}
	return result, nil
}

func (r *Repository) SoftDeleteRoleMenus(ctx context.Context, roleID int64, deletedAt time.Time) error {
	if err := r.db.WithContext(ctx).Table("permission_role_menu").Where("role_id = ? AND deleted_at IS NULL", roleID).Updates(map[string]any{
		"updated_at": deletedAt.UTC(), "deleted_at": deletedAt.UTC(),
	}).Error; err != nil {
		return fmt.Errorf("soft delete role menu grants: %w", err)
	}
	return nil
}

func (r *Repository) SoftDeleteRole(ctx context.Context, roleID int64, deletedAt time.Time) error {
	result := r.db.WithContext(ctx).Table("permission_role").Where("id = ? AND deleted_at IS NULL", roleID).Updates(map[string]any{
		"updated_at": deletedAt.UTC(), "deleted_at": deletedAt.UTC(),
	})
	if result.Error != nil {
		return fmt.Errorf("soft delete role: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("soft delete role %d: %w", roleID, gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *Repository) Count(ctx context.Context, query ListQuery) (int64, error) {
	var total int64
	if err := applyRoleListFilter(r.db.WithContext(ctx), query).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count roles: %w", err)
	}
	return total, nil
}

func (r *Repository) List(ctx context.Context, query ListQuery) ([]ListItem, error) {
	items := make([]ListItem, 0)
	db := applyRoleListFilter(r.db.WithContext(ctx), query).
		Select(`
			app_role.id,
			app_role.code,
			app_role.name,
			app_role.is_default,
			app_role.is_enabled,
			app_role.created_at,
			app_role.updated_at,
			(
				SELECT count(*)
				FROM permission_user_role AS user_role
				JOIN user_account AS app_user
				  ON app_user.id = user_role.user_id
				 AND app_user.deleted_at IS NULL
				WHERE user_role.role_id = app_role.id
				  AND user_role.deleted_at IS NULL
			) AS user_count,
			(
				SELECT count(*)
				FROM permission_role_menu AS role_menu
				JOIN permission_menu AS app_menu
				  ON app_menu.id = role_menu.menu_id
				 AND app_menu.deleted_at IS NULL
				 AND app_menu.menu_type IN ('page', 'action')
				WHERE role_menu.role_id = app_role.id
				  AND role_menu.deleted_at IS NULL
			) AS permission_count`).
		Order("app_role.created_at ASC, app_role.id ASC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Scan(&items)
	if db.Error != nil {
		return nil, fmt.Errorf("list roles: %w", db.Error)
	}
	return items, nil
}

func applyRoleListFilter(db *gorm.DB, query ListQuery) *gorm.DB {
	db = db.Table("permission_role AS app_role").Where("app_role.deleted_at IS NULL")
	if query.Keyword != "" {
		pattern := "%" + escapeRoleLike(query.Keyword) + "%"
		db = db.Where(`(app_role.code ILIKE ? ESCAPE E'\\' OR app_role.name ILIKE ? ESCAPE E'\\')`, pattern, pattern)
	}
	if query.IsEnabled != nil {
		db = db.Where("app_role.is_enabled = ?", *query.IsEnabled)
	}
	return db
}

func escapeRoleLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

func mapRoleWriteError(operation string, err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.ConstraintName {
		case "ux_permission_role_code_active":
			return fmt.Errorf("%s: %w", operation, ErrRoleCodeConflict)
		case "ux_permission_role_name_active":
			return fmt.Errorf("%s: %w", operation, ErrRoleNameConflict)
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func (r *Repository) FindDefault(ctx context.Context) (Role, error) {
	var roles []Role
	if err := r.db.WithContext(ctx).
		Where("is_default = ? AND is_enabled = ?", yesno.Yes, yesno.Yes).
		Limit(2).
		Find(&roles).Error; err != nil {
		return Role{}, fmt.Errorf("find default role: %w", err)
	}
	if len(roles) != 1 {
		return Role{}, fmt.Errorf("expected exactly one enabled default role, found %d", len(roles))
	}
	return roles[0], nil
}

func (r *Repository) FindByCode(ctx context.Context, code string) (Role, error) {
	var found Role
	if err := r.db.WithContext(ctx).
		Where("code = ? AND is_enabled = ?", code, yesno.Yes).
		Take(&found).Error; err != nil {
		return Role{}, fmt.Errorf("find enabled role %s: %w", code, err)
	}
	return found, nil
}

func (r *Repository) HasActiveUserWithRole(ctx context.Context, roleID int64) (bool, error) {
	var exists bool
	if err := r.db.WithContext(ctx).Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM permission_user_role AS user_role
			JOIN user_account AS app_user
			  ON app_user.id = user_role.user_id
			 AND app_user.deleted_at IS NULL
			 AND app_user.is_enabled = ?
			JOIN permission_role AS app_role
			  ON app_role.id = user_role.role_id
			 AND app_role.deleted_at IS NULL
			 AND app_role.is_enabled = ?
			WHERE user_role.role_id = ?
			  AND user_role.deleted_at IS NULL
		)`, yesno.Yes, yesno.Yes, roleID).Scan(&exists).Error; err != nil {
		return false, fmt.Errorf("check active user with role: %w", err)
	}
	return exists, nil
}
