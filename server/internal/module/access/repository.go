package access

import (
	"context"
	"fmt"

	"admin/server/internal/module/menu"
	"admin/server/internal/module/role"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type Source struct {
	RoleCodes      []string
	Menus          []menu.Menu
	GrantedMenuIDs []int64
	SuperAdmin     bool
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindSource(ctx context.Context, userID int64) (Source, error) {
	db := r.db.WithContext(ctx)
	var activeUserID int64
	result := db.Raw(`
		SELECT id
		FROM sys_user
		WHERE id = ?
		  AND is_enabled = ?
		  AND deleted_at IS NULL`, userID, yesno.Yes).Scan(&activeUserID)
	if result.Error != nil {
		return Source{}, fmt.Errorf("find access source user: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return Source{}, fmt.Errorf("find access source user: %w", gorm.ErrRecordNotFound)
	}

	type activeRole struct {
		ID   int64
		Code string
	}
	activeRoles := make([]activeRole, 0)
	if err := db.Raw(`
		SELECT app_role.id, app_role.code
		FROM sys_user_role AS user_role
		JOIN sys_role AS app_role
		  ON app_role.id = user_role.role_id
		 AND app_role.is_enabled = ?
		 AND app_role.deleted_at IS NULL
		WHERE user_role.user_id = ?
		  AND user_role.deleted_at IS NULL
		ORDER BY app_role.code, app_role.id`, yesno.Yes, userID).Scan(&activeRoles).Error; err != nil {
		return Source{}, fmt.Errorf("find access source roles: %w", err)
	}
	if len(activeRoles) == 0 {
		return Source{}, fmt.Errorf("find access source roles: %w", gorm.ErrRecordNotFound)
	}

	roleIDs := make([]int64, 0, len(activeRoles))
	roleCodes := make([]string, 0, len(activeRoles))
	superAdmin := false
	for _, active := range activeRoles {
		roleIDs = append(roleIDs, active.ID)
		roleCodes = append(roleCodes, active.Code)
		if active.Code == role.CodeSuperAdmin {
			superAdmin = true
		}
	}

	menus := make([]menu.Menu, 0)
	if err := db.Where("is_enabled = ?", yesno.Yes).
		Order("sort_order, code, id").
		Find(&menus).Error; err != nil {
		return Source{}, fmt.Errorf("find access source menus: %w", err)
	}

	grantedMenuIDs := make([]int64, 0)
	if err := db.Model(&menu.RoleMenu{}).
		Distinct("menu_id").
		Where("role_id IN ?", roleIDs).
		Order("menu_id").
		Pluck("menu_id", &grantedMenuIDs).Error; err != nil {
		return Source{}, fmt.Errorf("find access source direct grants: %w", err)
	}

	return Source{
		RoleCodes:      roleCodes,
		Menus:          menus,
		GrantedMenuIDs: grantedMenuIDs,
		SuperAdmin:     superAdmin,
	}, nil
}

func (r *Repository) HasPermission(ctx context.Context, userID int64, code string) (bool, error) {
	var allowed bool
	if err := r.db.WithContext(ctx).Raw(`
		WITH RECURSIVE permission_nodes AS (
			SELECT app_menu.id
			FROM sys_menu AS app_menu
			WHERE app_menu.code = ?
			  AND app_menu.menu_type IN ('page', 'action')
			  AND app_menu.is_enabled = ?
			  AND app_menu.deleted_at IS NULL
			UNION
			SELECT child.id
			FROM sys_menu AS child
			JOIN permission_nodes AS parent ON child.parent_id = parent.id
			WHERE child.is_enabled = ?
			  AND child.deleted_at IS NULL
		)
		SELECT EXISTS (
			SELECT 1
			FROM sys_user AS app_user
			JOIN sys_user_role AS user_role
			  ON user_role.user_id = app_user.id
			 AND user_role.deleted_at IS NULL
			JOIN sys_role AS app_role
			  ON app_role.id = user_role.role_id
			 AND app_role.is_enabled = ?
			 AND app_role.deleted_at IS NULL
			WHERE app_user.id = ?
			  AND app_user.is_enabled = ?
			  AND app_user.deleted_at IS NULL
			  AND EXISTS (SELECT 1 FROM permission_nodes)
			  AND (
				app_role.code = ?
				OR EXISTS (
					SELECT 1
					FROM sys_role_menu AS role_menu
					JOIN permission_nodes ON permission_nodes.id = role_menu.menu_id
					WHERE role_menu.role_id = app_role.id
					  AND role_menu.deleted_at IS NULL
				)
			  )
		)`, code, yesno.Yes, yesno.Yes, yesno.Yes, userID, yesno.Yes, role.CodeSuperAdmin).Scan(&allowed).Error; err != nil {
		return false, fmt.Errorf("check permission %q for user %d: %w", code, userID, err)
	}
	return allowed, nil
}
