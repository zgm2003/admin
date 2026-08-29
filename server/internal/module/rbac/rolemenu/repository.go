package rolemenu

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"time"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) FindByRole(ctx context.Context, roleID int64) ([]RoleMenu, error) {
	rows := make([]RoleMenu, 0)
	if err := r.db.WithContext(ctx).Where("role_id = ?", roleID).Order("menu_id, id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("find role menus: %w", err)
	}
	return rows, nil
}
func (r *Repository) HasMenuGrant(ctx context.Context, menuID int64) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&RoleMenu{}).Where("menu_id = ?", menuID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("find menu grants: %w", err)
	}
	return count > 0, nil
}
func (r *Repository) Create(ctx context.Context, values []RoleMenu) error {
	if len(values) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(&values).Error; err != nil {
		return fmt.Errorf("create role menus: %w", err)
	}
	return nil
}
func (r *Repository) SoftDeleteIDs(ctx context.Context, ids []int64, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Model(&RoleMenu{}).Where("id IN ?", ids).Updates(map[string]any{"updated_at": now.UTC(), "deleted_at": now.UTC()}).Error; err != nil {
		return fmt.Errorf("delete role menus: %w", err)
	}
	return nil
}
func (r *Repository) SoftDeleteByRole(ctx context.Context, roleID int64, now time.Time) error {
	if err := r.db.WithContext(ctx).Model(&RoleMenu{}).Where("role_id = ?", roleID).Updates(map[string]any{"updated_at": now.UTC(), "deleted_at": now.UTC()}).Error; err != nil {
		return fmt.Errorf("delete role menu grants: %w", err)
	}
	return nil
}
func (r *Repository) SoftDeleteByMenus(ctx context.Context, menuIDs []int64, now time.Time) error {
	if len(menuIDs) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Model(&RoleMenu{}).Where("menu_id IN ?", menuIDs).Updates(map[string]any{"updated_at": now.UTC(), "deleted_at": now.UTC()}).Error; err != nil {
		return fmt.Errorf("delete menu grants: %w", err)
	}
	return nil
}
