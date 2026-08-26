package access

import (
	"context"
	"fmt"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type MenuType string

const (
	MenuDirectory MenuType = "directory"
	MenuPage      MenuType = "page"
	MenuAction    MenuType = "action"

	superAdminRoleCode = "super_admin"
)

type SourceMenu struct {
	ID            int64
	ParentID      *int64
	MenuType      MenuType
	Code          string
	I18nKey       *string
	Path          *string
	ComponentPath *string
	Icon          *string
	SortOrder     int
	IsEnabled     yesno.Value
	IsHidden      yesno.Value
}

type Source struct {
	Version        int64
	RoleCodes      []string
	Menus          []SourceMenu
	GrantedMenuIDs []int64
	SuperAdmin     bool
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindSourceWithVersion(ctx context.Context, userID int64) (Source, error) {
	db := r.db.WithContext(ctx)
	var version int64
	result := db.Raw(`
		SELECT access_version.version
		FROM user_account AS app_user
		JOIN rbac_access_version AS access_version ON access_version.user_id = app_user.id
		WHERE app_user.id = ?
		  AND app_user.is_enabled = ?
		  AND app_user.deleted_at IS NULL`, userID, yesno.Yes).Scan(&version)
	if result.Error != nil {
		return Source{}, fmt.Errorf("find access source version: %w", result.Error)
	}
	if result.RowsAffected != 1 || version < 1 {
		return Source{}, fmt.Errorf("find access source version: %w", gorm.ErrRecordNotFound)
	}

	type activeRole struct {
		ID   int64
		Code string
	}
	activeRoles := make([]activeRole, 0)
	if err := db.Raw(`
		SELECT DISTINCT app_role.id, app_role.code
		FROM rbac_user_role AS user_role
		JOIN rbac_role AS app_role
		  ON app_role.id = user_role.role_id
		 AND app_role.is_enabled = ?
		 AND app_role.deleted_at IS NULL
		WHERE user_role.user_id = ?
		  AND user_role.deleted_at IS NULL
		ORDER BY app_role.code, app_role.id`, yesno.Yes, userID).Scan(&activeRoles).Error; err != nil {
		return Source{}, fmt.Errorf("find access source roles: %w", err)
	}

	roleIDs := make([]int64, 0, len(activeRoles))
	roleCodes := make([]string, 0, len(activeRoles))
	superAdmin := false
	for _, active := range activeRoles {
		roleIDs = append(roleIDs, active.ID)
		roleCodes = append(roleCodes, active.Code)
		if active.Code == superAdminRoleCode {
			superAdmin = true
		}
	}

	menus := make([]SourceMenu, 0)
	if err := db.Raw(`
		SELECT id, parent_id, menu_type, code, i18n_key, path,
		       component_path, icon, sort_order, is_enabled, is_hidden
		FROM rbac_menu
		WHERE is_enabled = ? AND deleted_at IS NULL
		ORDER BY sort_order, code, id`, yesno.Yes).Scan(&menus).Error; err != nil {
		return Source{}, fmt.Errorf("find access source menus: %w", err)
	}

	grantedMenuIDs := make([]int64, 0)
	if len(roleIDs) > 0 && !superAdmin {
		if err := db.Raw(`
			SELECT DISTINCT role_menu.menu_id
			FROM rbac_role_menu AS role_menu
			JOIN rbac_menu AS app_menu
			  ON app_menu.id = role_menu.menu_id
			 AND app_menu.is_enabled = ?
			 AND app_menu.deleted_at IS NULL
			WHERE role_menu.role_id IN ?
			  AND role_menu.deleted_at IS NULL
			ORDER BY role_menu.menu_id`, yesno.Yes, roleIDs).Scan(&grantedMenuIDs).Error; err != nil {
			return Source{}, fmt.Errorf("find access source direct grants: %w", err)
		}
	}

	return Source{
		Version: version, RoleCodes: roleCodes, Menus: menus, GrantedMenuIDs: grantedMenuIDs, SuperAdmin: superAdmin,
	}, nil
}
