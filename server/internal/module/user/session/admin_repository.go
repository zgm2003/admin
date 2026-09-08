package session

import (
	authclient "admin/server/internal/module/auth/client"
	"admin/server/internal/shared/apperror"
	"context"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type sessionAdminRow struct {
	ID               int64
	UserID           int64
	Username         string
	Platform         string
	DeviceID         string
	ClientIP         string
	UserAgent        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	RefreshExpiresAt time.Time
	RevokedAt        *time.Time
	Status           string
}

type sessionPlatformRow struct {
	Session
	Platform string
}

func (r *Repository) ListAdmin(ctx context.Context, query AdminSessionQuery, now time.Time) ([]AdminSession, int64, error) {
	db := r.db.WithContext(ctx).Table("user_session AS session").
		Select("session.id, session.user_id, app_user.username, platform.code AS platform, session.device_id, "+
			"session.client_ip, session.user_agent, session.created_at, session.updated_at, "+
			"session.refresh_expires_at, session.revoked_at, "+
			"CASE WHEN session.revoked_at IS NOT NULL THEN 'revoked' "+
			"WHEN session.refresh_expires_at <= ? THEN 'expired' ELSE 'active' END AS status", now.UTC()).
		Joins("JOIN user_account AS app_user ON app_user.id = session.user_id").
		Joins("JOIN permission_auth_platform AS platform ON platform.id = session.platform_id").
		Where("app_user.deleted_at IS NULL")
	if query.Username != "" {
		db = db.Where("app_user.username LIKE ? ESCAPE '\\'", adminPrefixPattern(query.Username))
	}
	if query.Platform != "" {
		if err := authclient.ValidatePlatform(query.Platform); err != nil {
			return nil, 0, apperror.InvalidRequest(err)
		}
		db = db.Where("platform.code = ?", query.Platform)
	}
	switch query.Status {
	case SessionStatusActive:
		db = db.Where("session.revoked_at IS NULL AND session.refresh_expires_at > ?", now.UTC())
	case SessionStatusExpired:
		db = db.Where("session.revoked_at IS NULL AND session.refresh_expires_at <= ?", now.UTC())
	case SessionStatusRevoked:
		db = db.Where("session.revoked_at IS NOT NULL")
	case "":
	default:
		return nil, 0, apperror.InvalidRequest(fmt.Errorf("session status is invalid"))
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count sessions: %w", err)
	}
	rows := make([]sessionAdminRow, 0, query.PageSize)
	if err := db.Order("session.created_at DESC, session.id DESC").
		Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list sessions: %w", err)
	}
	items := make([]AdminSession, 0, len(rows))
	for _, row := range rows {
		items = append(items, AdminSession{
			ID: row.ID, UserID: row.UserID, Username: row.Username, Platform: row.Platform, DeviceID: row.DeviceID,
			ClientIP: row.ClientIP, UserAgent: row.UserAgent, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			RefreshExpiresAt: row.RefreshExpiresAt, RevokedAt: row.RevokedAt, Status: SessionStatus(row.Status),
		})
	}
	return items, total, nil
}

func (r *Repository) StatsAdmin(ctx context.Context, now time.Time) (AdminSessionStats, error) {
	stats := AdminSessionStats{Platforms: make(map[string]int64)}
	if err := r.db.WithContext(ctx).Table("user_session AS session").
		Joins("JOIN user_account AS app_user ON app_user.id = session.user_id").
		Where("app_user.deleted_at IS NULL AND session.revoked_at IS NULL AND session.refresh_expires_at > ?", now.UTC()).
		Count(&stats.ActiveTotal).Error; err != nil {
		return AdminSessionStats{}, fmt.Errorf("count active sessions: %w", err)
	}
	type platformCount struct {
		Platform string
		Count    int64
	}
	rows := make([]platformCount, 0)
	if err := r.db.WithContext(ctx).Table("user_session AS session").
		Select("platform.code AS platform, COUNT(*) AS count").
		Joins("JOIN user_account AS app_user ON app_user.id = session.user_id").
		Joins("JOIN permission_auth_platform AS platform ON platform.id = session.platform_id").
		Where("app_user.deleted_at IS NULL AND session.revoked_at IS NULL AND session.refresh_expires_at > ?", now.UTC()).
		Group("platform.code").Scan(&rows).Error; err != nil {
		return AdminSessionStats{}, fmt.Errorf("count active sessions by platform: %w", err)
	}
	for _, row := range rows {
		stats.Platforms[row.Platform] = row.Count
	}
	return stats, nil
}

func (r *Repository) RevokeAdmin(ctx context.Context, ids []int64, currentSessionID int64, now time.Time) (AdminRevokeResult, error) {
	result := AdminRevokeResult{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rows := make([]sessionPlatformRow, 0, len(ids))
		if err := tx.Unscoped().Table("user_session AS session").
			Select("session.*, platform.code AS platform").
			Joins("JOIN permission_auth_platform AS platform ON platform.id = session.platform_id").
			Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "session"}}).
			Where("session.id IN ?", ids).Scan(&rows).Error; err != nil {
			return fmt.Errorf("lock sessions for revoke: %w", err)
		}
		for _, row := range rows {
			session := row.Session
			session.Platform = row.Platform
			switch {
			case session.ID == currentSessionID:
				result.SkippedCurrent++
			case session.RevokedAt != nil:
				result.SkippedRevoked++
			default:
				result.Revoked = append(result.Revoked, session)
			}
		}
		if len(result.Revoked) == 0 {
			return nil
		}
		revokeIDs := make([]int64, len(result.Revoked))
		for index := range result.Revoked {
			revokeIDs[index] = result.Revoked[index].ID
		}
		if err := tx.Model(&Session{}).Where("id IN ? AND revoked_at IS NULL", revokeIDs).
			Updates(map[string]any{"revoked_at": now.UTC(), "updated_at": now.UTC()}).Error; err != nil {
			return fmt.Errorf("revoke admin sessions: %w", err)
		}
		for index := range result.Revoked {
			value := now.UTC()
			result.Revoked[index].RevokedAt = &value
			result.Revoked[index].UpdatedAt = value
		}
		return nil
	})
	if err != nil {
		return AdminRevokeResult{}, err
	}
	return result, nil
}

func (r *Repository) FindAdminRevokeTargets(ctx context.Context, ids []int64) ([]Record, error) {
	rows := make([]sessionPlatformRow, 0, len(ids))
	if err := r.db.WithContext(ctx).Unscoped().Table("user_session AS session").
		Select("session.*, platform.code AS platform").
		Joins("JOIN permission_auth_platform AS platform ON platform.id = session.platform_id").
		Where("session.id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("find sessions for admin revoke: %w", err)
	}
	result := make([]Record, 0, len(rows))
	for _, row := range rows {
		row.Session.Platform = row.Platform
		result = append(result, row.Session)
	}
	return result, nil
}
