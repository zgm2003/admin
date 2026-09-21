package notification

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ErrInvalidFacts means the permission graph is validly stored but cannot be
// used as a notification audience (for example a disabled role or an
// incomplete notification menu capability).
var ErrInvalidFacts = errors.New("notification permission facts are invalid")

type Option struct {
	ID    int64  `json:"id"`
	Label string `json:"label"`
}

// Reader is the intentionally narrow permission read surface consumed by the
// notification-task module. The implementation owns all permission tables;
// callers never compose permission SQL themselves.
type Reader interface {
	PlatformNames(context.Context, []int64) (map[int64]string, error)
	ValidatePlatformNotification(context.Context, int64) error
	ValidateUserTargets(context.Context, int64, []int64, time.Time) error
	ValidateRoleTargets(context.Context, int64, []int64, time.Time) error
	Options(context.Context, string, int64, string, int64, int) ([]Option, error)
	BatchUsers(context.Context, int64, string, []int64, time.Time, int64, int64, int) ([]int64, error)
}

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) PlatformNames(ctx context.Context, ids []int64) (map[int64]string, error) {
	result := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		ID   int64
		Name string
	}
	if err := r.db.WithContext(ctx).Table("permission_auth_platform").Select("id,name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID] = row.Name
	}
	return result, nil
}

func (r *Repository) ValidatePlatformNotification(ctx context.Context, platformID int64) error {
	var enabled int64
	if err := r.db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_auth_platform WHERE id=? AND is_enabled=1 AND deleted_at IS NULL`, platformID).Scan(&enabled).Error; err != nil {
		return err
	}
	if enabled != 1 {
		return fmt.Errorf("%w: platform is unavailable", ErrInvalidFacts)
	}
	type node struct {
		ID       int64
		ParentID *int64
		MenuType string
		Code     string
		IsHidden int16
	}
	var nodes []node
	if err := r.db.WithContext(ctx).Raw(`SELECT id,parent_id,menu_type,code,is_hidden FROM permission_menu WHERE platform_id=? AND code IN ('message:notification:view','message:notification:list','message:notification:read','message:notification:delete') AND is_enabled=1 AND deleted_at IS NULL`, platformID).Scan(&nodes).Error; err != nil {
		return err
	}
	if len(nodes) != 4 {
		return fmt.Errorf("%w: notification capability is incomplete", ErrInvalidFacts)
	}
	var pageID int64
	seen := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		if seen[node.Code] {
			return fmt.Errorf("%w: notification capability is duplicated", ErrInvalidFacts)
		}
		seen[node.Code] = true
		if node.Code == "message:notification:view" {
			if node.MenuType != "page" || node.IsHidden != 1 || node.ParentID != nil {
				return fmt.Errorf("%w: notification page fact is invalid", ErrInvalidFacts)
			}
			pageID = node.ID
		}
	}
	if pageID == 0 {
		return fmt.Errorf("%w: notification page fact is missing", ErrInvalidFacts)
	}
	for _, node := range nodes {
		if node.Code == "message:notification:view" {
			continue
		}
		if node.MenuType != "action" || node.ParentID == nil || *node.ParentID != pageID {
			return fmt.Errorf("%w: notification action fact is invalid", ErrInvalidFacts)
		}
	}
	return nil
}

func (r *Repository) ValidateUserTargets(ctx context.Context, platformID int64, targetIDs []int64, at time.Time) error {
	var count int64
	err := r.db.WithContext(ctx).Raw(`
SELECT count(*)
FROM user_account app_user
WHERE app_user.id IN ? AND app_user.is_enabled=1 AND app_user.deleted_at IS NULL
  AND EXISTS (
    SELECT 1 FROM permission_user_role user_role
    JOIN permission_role app_role ON app_role.id=user_role.role_id
      AND app_role.is_enabled=1 AND app_role.deleted_at IS NULL
    WHERE user_role.user_id=app_user.id
      AND user_role.created_at<=? AND (user_role.deleted_at IS NULL OR user_role.deleted_at>?)
      AND (app_role.code='super_admin' OR EXISTS (
        SELECT 1 FROM permission_role_menu role_menu
        JOIN permission_menu app_menu ON app_menu.id=role_menu.menu_id
          AND app_menu.platform_id=? AND app_menu.is_enabled=1 AND app_menu.deleted_at IS NULL
        WHERE role_menu.role_id=app_role.id
          AND role_menu.created_at<=? AND (role_menu.deleted_at IS NULL OR role_menu.deleted_at>?)
      ))
  )`, targetIDs, at, at, platformID, at, at).Scan(&count).Error
	if err != nil {
		return err
	}
	if count != int64(len(targetIDs)) {
		return fmt.Errorf("%w: user target is unavailable", ErrInvalidFacts)
	}
	return nil
}

func (r *Repository) ValidateRoleTargets(ctx context.Context, platformID int64, targetIDs []int64, at time.Time) error {
	var count int64
	err := r.db.WithContext(ctx).Raw(`
SELECT count(*) FROM permission_role app_role
WHERE app_role.id IN ? AND app_role.is_enabled=1 AND app_role.deleted_at IS NULL
  AND (app_role.code='super_admin' OR EXISTS (
    SELECT 1 FROM permission_role_menu role_menu
    JOIN permission_menu app_menu ON app_menu.id=role_menu.menu_id
      AND app_menu.platform_id=? AND app_menu.is_enabled=1 AND app_menu.deleted_at IS NULL
    WHERE role_menu.role_id=app_role.id
      AND role_menu.created_at<=? AND (role_menu.deleted_at IS NULL OR role_menu.deleted_at>?)
  ))`, targetIDs, platformID, at, at).Scan(&count).Error
	if err != nil {
		return err
	}
	if count != int64(len(targetIDs)) {
		return fmt.Errorf("%w: role target is unavailable", ErrInvalidFacts)
	}
	return nil
}

func (r *Repository) Options(ctx context.Context, kind string, platformID int64, keyword string, after int64, limit int) ([]Option, error) {
	pattern := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(keyword) + "%"
	var rows []Option
	switch kind {
	case "platform":
		if err := r.ValidatePlatformOptions(ctx); err != nil {
			return nil, err
		}
		err := r.db.WithContext(ctx).Raw(`
SELECT platform.id, platform.name AS label
FROM permission_auth_platform platform
JOIN permission_menu page ON page.platform_id=platform.id AND page.code='message:notification:view' AND page.menu_type='page' AND page.parent_id IS NULL AND page.is_hidden=1 AND page.is_enabled=1 AND page.deleted_at IS NULL
JOIN permission_menu action ON action.platform_id=platform.id AND action.parent_id=page.id AND action.code IN ('message:notification:list','message:notification:read','message:notification:delete') AND action.menu_type='action' AND action.is_enabled=1 AND action.deleted_at IS NULL
WHERE platform.id>? AND platform.is_enabled=1 AND platform.deleted_at IS NULL AND (?='' OR platform.name ILIKE ? ESCAPE '\')
GROUP BY platform.id,platform.name HAVING count(DISTINCT action.code)=3 ORDER BY platform.id LIMIT ?`, after, keyword, pattern, limit).Scan(&rows).Error
		return rows, err
	case "role":
		err := r.db.WithContext(ctx).Raw(`
SELECT DISTINCT app_role.id, app_role.name AS label
FROM permission_role app_role
JOIN permission_role_menu role_menu ON role_menu.role_id=app_role.id AND role_menu.deleted_at IS NULL
JOIN permission_menu app_menu ON app_menu.id=role_menu.menu_id AND app_menu.platform_id=? AND app_menu.is_enabled=1 AND app_menu.deleted_at IS NULL
WHERE app_role.id>? AND app_role.is_enabled=1 AND app_role.deleted_at IS NULL AND (?='' OR app_role.name ILIKE ? ESCAPE '\')
ORDER BY app_role.id LIMIT ?`, platformID, after, keyword, pattern, limit).Scan(&rows).Error
		return rows, err
	case "user":
		err := r.db.WithContext(ctx).Raw(`
SELECT app_user.id, app_user.username AS label
FROM user_account app_user
WHERE app_user.id>? AND app_user.is_enabled=1 AND app_user.deleted_at IS NULL AND (?='' OR app_user.username ILIKE ? ESCAPE '\')
  AND EXISTS (SELECT 1 FROM permission_user_role user_role JOIN permission_role app_role ON app_role.id=user_role.role_id AND app_role.is_enabled=1 AND app_role.deleted_at IS NULL
    WHERE user_role.user_id=app_user.id AND user_role.deleted_at IS NULL AND (app_role.code='super_admin' OR EXISTS (
      SELECT 1 FROM permission_role_menu role_menu JOIN permission_menu app_menu ON app_menu.id=role_menu.menu_id AND app_menu.platform_id=? AND app_menu.is_enabled=1 AND app_menu.deleted_at IS NULL
      WHERE role_menu.role_id=app_role.id AND role_menu.deleted_at IS NULL)))
ORDER BY app_user.id LIMIT ?`, after, keyword, pattern, platformID, limit).Scan(&rows).Error
		return rows, err
	default:
		return nil, fmt.Errorf("invalid notification option kind")
	}
}

func (r *Repository) ValidatePlatformOptions(ctx context.Context) error {
	var ids []int64
	if err := r.db.WithContext(ctx).Raw(`SELECT DISTINCT platform.id FROM permission_auth_platform platform JOIN permission_menu menu ON menu.platform_id=platform.id AND menu.code IN ('message:notification:view','message:notification:list','message:notification:read','message:notification:delete') AND menu.is_enabled=1 AND menu.deleted_at IS NULL WHERE platform.is_enabled=1 AND platform.deleted_at IS NULL ORDER BY platform.id`).Scan(&ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := r.ValidatePlatformNotification(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) BatchUsers(ctx context.Context, platformID int64, audience string, targetIDs []int64, submittedAt time.Time, nextUser, maxUser int64, limit int) ([]int64, error) {
	var ids []int64
	if audience == "user" {
		err := r.db.WithContext(ctx).Raw(`
WITH eligible_roles AS MATERIALIZED (SELECT app_role.id FROM permission_role app_role WHERE app_role.is_enabled=1 AND app_role.deleted_at IS NULL AND (app_role.code='super_admin' OR EXISTS (
 SELECT 1 FROM permission_role_menu role_menu JOIN permission_menu app_menu ON app_menu.id=role_menu.menu_id AND app_menu.platform_id=? AND app_menu.is_enabled=1 AND app_menu.deleted_at IS NULL
 WHERE role_menu.role_id=app_role.id AND role_menu.created_at<=? AND (role_menu.deleted_at IS NULL OR role_menu.deleted_at>?))))
SELECT app_user.id FROM user_account app_user
WHERE app_user.id IN ? AND app_user.is_enabled=1 AND app_user.deleted_at IS NULL AND app_user.id> ? AND app_user.id<=?
AND EXISTS (SELECT 1 FROM permission_user_role user_role JOIN eligible_roles ON eligible_roles.id=user_role.role_id WHERE user_role.user_id=app_user.id AND user_role.created_at<=? AND (user_role.deleted_at IS NULL OR user_role.deleted_at>?))
ORDER BY app_user.id LIMIT ?`, platformID, submittedAt, submittedAt, targetIDs, nextUser, maxUser, submittedAt, submittedAt, limit).Scan(&ids).Error
		return ids, err
	}
	err := r.db.WithContext(ctx).Raw(`
WITH eligible_roles AS MATERIALIZED (SELECT app_role.id FROM permission_role app_role WHERE app_role.is_enabled=1 AND app_role.deleted_at IS NULL AND (app_role.code='super_admin' OR EXISTS (
 SELECT 1 FROM permission_role_menu role_menu JOIN permission_menu app_menu ON app_menu.id=role_menu.menu_id AND app_menu.platform_id=? AND app_menu.is_enabled=1 AND app_menu.deleted_at IS NULL
 WHERE role_menu.role_id=app_role.id AND role_menu.created_at<=? AND (role_menu.deleted_at IS NULL OR role_menu.deleted_at>?))))
SELECT DISTINCT user_role.user_id FROM permission_user_role user_role JOIN user_account app_user ON app_user.id=user_role.user_id AND app_user.is_enabled=1 AND app_user.deleted_at IS NULL JOIN eligible_roles ON eligible_roles.id=user_role.role_id
WHERE user_role.role_id IN ? AND user_role.created_at<=? AND (user_role.deleted_at IS NULL OR user_role.deleted_at>?) AND user_role.user_id>? AND user_role.user_id<=?
ORDER BY user_role.user_id LIMIT ?`, platformID, submittedAt, submittedAt, targetIDs, submittedAt, submittedAt, nextUser, maxUser, limit).Scan(&ids).Error
	return ids, err
}
