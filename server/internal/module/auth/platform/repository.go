package authplatform

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UpdateValues struct {
	Name                   string
	AccessTTLSeconds       int
	RefreshTTLSeconds      int
	SessionCacheTTLSeconds int
	AccessCacheTTLSeconds  int
	BindDevice             yesno.Value
	BindIP                 yesno.Value
	MaxSessions            int16
	AllowRegister          yesno.Value
}

type Repository struct {
	db *gorm.DB
}

type SessionRef struct {
	UserID    int64
	SessionID int64
}

type platformSession struct {
	ID        int64
	UserID    int64
	CreatedAt time.Time
}

func (platformSession) TableName() string { return "user_session" }

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindPolicy(ctx context.Context, code string) (Platform, error) {
	var found Platform
	if err := r.db.WithContext(ctx).Where("code = ?", code).Take(&found).Error; err != nil {
		return Platform{}, fmt.Errorf("find authentication platform policy: %w", err)
	}
	return found, nil
}

func (r *Repository) Count(ctx context.Context, query ListQuery) (int64, error) {
	var total int64
	if err := applyListFilter(r.db.WithContext(ctx), query).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count authentication platforms: %w", err)
	}
	return total, nil
}

func (r *Repository) List(ctx context.Context, query ListQuery) ([]Platform, error) {
	rows := make([]Platform, 0)
	if err := applyListFilter(r.db.WithContext(ctx), query).
		Order("updated_at DESC, id DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list authentication platforms: %w", err)
	}
	return rows, nil
}

func applyListFilter(db *gorm.DB, query ListQuery) *gorm.DB {
	db = db.Model(&Platform{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("code ILIKE ? OR name ILIKE ?", like, like)
	}
	if query.IsEnabled != nil {
		db = db.Where("is_enabled = ?", *query.IsEnabled)
	}
	return db
}

func (r *Repository) Transaction(ctx context.Context, fn func(*Repository) error) error {
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return fn(NewRepository(tx)) }); err != nil {
		return fmt.Errorf("authentication platform transaction: %w", err)
	}
	return nil
}

func (r *Repository) LockByID(ctx context.Context, id int64) (Platform, error) {
	var found Platform
	if err := r.db.WithContext(ctx).Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&found).Error; err != nil {
		return Platform{}, fmt.Errorf("lock authentication platform: %w", err)
	}
	return found, nil
}

func (r *Repository) LockByCodeUnscoped(ctx context.Context, code string) ([]Platform, error) {
	rows := make([]Platform, 0, 2)
	if err := r.db.WithContext(ctx).Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", code).Order("id ASC").Limit(2).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("lock authentication platform code: %w", err)
	}
	return rows, nil
}

func (r *Repository) FindActiveSessionUsers(ctx context.Context, platform string) ([]int64, error) {
	userIDs := make([]int64, 0)
	if err := r.db.WithContext(ctx).Table("user_session AS session").Distinct("session.user_id").
		Joins("JOIN auth_platform AS platform_ref ON platform_ref.id = session.platform_id").
		Where("platform_ref.code = ? AND session.revoked_at IS NULL", platform).Order("session.user_id ASC").Pluck("session.user_id", &userIDs).Error; err != nil {
		return nil, fmt.Errorf("find authentication platform session users: %w", err)
	}
	return userIDs, nil
}

func (r *Repository) HasActiveMenus(ctx context.Context, platformID int64) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Table("rbac_menu").
		Where("platform_id = ? AND deleted_at IS NULL", platformID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("inspect authentication platform active menus: %w", err)
	}
	return count > 0, nil
}

func (r *Repository) FindSessionLimitCandidates(ctx context.Context, platform string, maxSessions int16) ([]int64, error) {
	if maxSessions < 1 {
		return []int64{}, nil
	}
	userIDs := make([]int64, 0)
	if err := r.db.WithContext(ctx).Raw(`
		SELECT user_id
		FROM user_session AS session
		JOIN auth_platform AS platform_ref ON platform_ref.id = session.platform_id
		WHERE platform_ref.code = ? AND session.revoked_at IS NULL
		GROUP BY user_id
		HAVING count(*) > ?
		ORDER BY user_id ASC`, platform, maxSessions).Scan(&userIDs).Error; err != nil {
		return nil, fmt.Errorf("find authentication platform session limit candidates: %w", err)
	}
	return userIDs, nil
}

func (r *Repository) LockActiveSessionUsers(ctx context.Context, platform string) ([]int64, error) {
	userIDs := make([]int64, 0)
	if err := r.db.WithContext(ctx).Raw(`
		SELECT app_user.id
		FROM user_account AS app_user
		WHERE app_user.id IN (
			SELECT session.user_id FROM user_session AS session JOIN auth_platform AS platform_ref ON platform_ref.id = session.platform_id WHERE platform_ref.code = ? AND session.revoked_at IS NULL
		)
		ORDER BY app_user.id ASC
		FOR UPDATE OF app_user`, platform).Scan(&userIDs).Error; err != nil {
		return nil, fmt.Errorf("lock authentication platform session users: %w", err)
	}
	return userIDs, nil
}

func (r *Repository) RevokePlatformSessions(ctx context.Context, platform string, now time.Time) ([]SessionRef, error) {
	rows := make([]platformSession, 0)
	if err := r.db.WithContext(ctx).Table("user_session AS session").Joins("JOIN auth_platform AS platform_ref ON platform_ref.id = session.platform_id").Clauses(clause.Locking{Strength: "UPDATE"}).Where("platform_ref.code = ? AND session.revoked_at IS NULL", platform).
		Order("user_id ASC, created_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("find authentication platform sessions for revocation: %w", err)
	}
	return r.revokeSessions(ctx, rows, now)
}

func (r *Repository) EnforcePlatformLimit(ctx context.Context, platform string, maxSessions int16, now time.Time) ([]SessionRef, error) {
	if maxSessions < 0 || maxSessions > MaximumSessions {
		return nil, fmt.Errorf("authentication platform session limit is invalid")
	}
	if maxSessions == 0 {
		return []SessionRef{}, nil
	}
	rows := make([]platformSession, 0)
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Table("user_session AS session").Joins("JOIN auth_platform AS platform_ref ON platform_ref.id = session.platform_id").Where("platform_ref.code = ? AND session.revoked_at IS NULL", platform).
		Order("user_id ASC, created_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("lock authentication platform sessions for limit: %w", err)
	}
	excess := make([]platformSession, 0)
	currentUserID := int64(0)
	kept := int16(0)
	for _, row := range rows {
		if row.UserID != currentUserID {
			currentUserID = row.UserID
			kept = 0
		}
		if kept < maxSessions {
			kept++
			continue
		}
		excess = append(excess, row)
	}
	return r.revokeSessions(ctx, excess, now)
}

func (r *Repository) revokeSessions(ctx context.Context, sessions []platformSession, now time.Time) ([]SessionRef, error) {
	if len(sessions) == 0 {
		return []SessionRef{}, nil
	}
	ids := make([]int64, len(sessions))
	refs := make([]SessionRef, len(sessions))
	for index, session := range sessions {
		if session.ID < 1 || session.UserID < 1 {
			return nil, fmt.Errorf("authentication platform session identity is invalid")
		}
		ids[index] = session.ID
		refs[index] = SessionRef{UserID: session.UserID, SessionID: session.ID}
	}
	result := r.db.WithContext(ctx).Table("user_session").Where("id IN ? AND revoked_at IS NULL", ids).Updates(map[string]any{
		"revoked_at": now.UTC(), "updated_at": now.UTC(),
	})
	if result.Error != nil {
		return nil, fmt.Errorf("revoke authentication platform sessions: %w", result.Error)
	}
	if result.RowsAffected != int64(len(ids)) {
		return nil, fmt.Errorf("revoke authentication platform sessions affected %d rows, want %d", result.RowsAffected, len(ids))
	}
	sort.Slice(refs, func(left, right int) bool {
		if refs[left].UserID != refs[right].UserID {
			return refs[left].UserID < refs[right].UserID
		}
		return refs[left].SessionID < refs[right].SessionID
	})
	return refs, nil
}

func (r *Repository) Create(ctx context.Context, value *Platform) error {
	if err := r.db.WithContext(ctx).Create(value).Error; err != nil {
		return mapWriteError("create authentication platform", err)
	}
	return nil
}

func (r *Repository) UpdatePolicy(ctx context.Context, id int64, values UpdateValues, updatedAt time.Time) (int64, error) {
	var version int64
	result := r.db.WithContext(ctx).Raw(`
		UPDATE auth_platform
		SET name = ?, access_ttl_seconds = ?, refresh_ttl_seconds = ?,
			session_cache_ttl_seconds = ?, access_cache_ttl_seconds = ?,
			bind_device = ?, bind_ip = ?, max_sessions = ?, allow_register = ?,
			policy_version = policy_version + 1, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
		RETURNING policy_version`, values.Name, values.AccessTTLSeconds, values.RefreshTTLSeconds,
		values.SessionCacheTTLSeconds, values.AccessCacheTTLSeconds, values.BindDevice, values.BindIP,
		values.MaxSessions, values.AllowRegister, updatedAt.UTC(), id).Scan(&version)
	if result.Error != nil {
		return 0, mapWriteError("update authentication platform policy", result.Error)
	}
	if result.RowsAffected != 1 {
		return 0, fmt.Errorf("update authentication platform policy: %w", gorm.ErrRecordNotFound)
	}
	return version, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id int64, value yesno.Value, updatedAt time.Time) (int64, error) {
	var version int64
	result := r.db.WithContext(ctx).Raw(`
		UPDATE auth_platform
		SET is_enabled = ?, policy_version = policy_version + 1, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
		RETURNING policy_version`, value, updatedAt.UTC(), id).Scan(&version)
	if result.Error != nil {
		return 0, fmt.Errorf("update authentication platform status: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return 0, fmt.Errorf("update authentication platform status: %w", gorm.ErrRecordNotFound)
	}
	return version, nil
}

func (r *Repository) SoftDelete(ctx context.Context, id int64, deletedAt time.Time) (int64, error) {
	var version int64
	result := r.db.WithContext(ctx).Raw(`
		UPDATE auth_platform
		SET policy_version = policy_version + 1, updated_at = ?, deleted_at = ?
		WHERE id = ? AND deleted_at IS NULL
		RETURNING policy_version`, deletedAt.UTC(), deletedAt.UTC(), id).Scan(&version)
	if result.Error != nil {
		return 0, fmt.Errorf("delete authentication platform: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return 0, fmt.Errorf("delete authentication platform: %w", gorm.ErrRecordNotFound)
	}
	return version, nil
}

func mapWriteError(operation string, err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" && postgresError.ConstraintName == "ux_auth_platform_code_active" {
		return fmt.Errorf("%s: %w", operation, ErrCodeConflict)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
