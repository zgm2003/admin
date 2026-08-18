package role

import (
	"context"
	"errors"
	"fmt"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	CodeSuperAdmin     = "super_admin"
	CodeRegisteredUser = "registered_user"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) EnsureSystemRoles(ctx context.Context) error {
	systemRoles := []Role{
		{Code: CodeSuperAdmin, Name: "超级管理员", IsDefault: yesno.No, IsEnabled: yesno.Yes},
		{Code: CodeRegisteredUser, Name: "普通用户", IsDefault: yesno.Yes, IsEnabled: yesno.Yes},
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, expected := range systemRoles {
			var stored Role
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", expected.Code).Take(&stored).Error
			switch {
			case errors.Is(err, gorm.ErrRecordNotFound):
				if err := tx.Create(&expected).Error; err != nil {
					return fmt.Errorf("create system role %s: %w", expected.Code, err)
				}
			case err != nil:
				return fmt.Errorf("find system role %s: %w", expected.Code, err)
			case stored.Name != expected.Name || stored.IsDefault != expected.IsDefault || stored.IsEnabled != expected.IsEnabled:
				return fmt.Errorf("system role %s has unexpected name or status", expected.Code)
			}
		}
		return nil
	})
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
			FROM sys_user_role AS user_role
			JOIN sys_user AS app_user
			  ON app_user.id = user_role.user_id
			 AND app_user.deleted_at IS NULL
			 AND app_user.is_enabled = ?
			JOIN sys_role AS app_role
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
