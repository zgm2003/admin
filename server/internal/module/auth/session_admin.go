package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"admin/server/internal/module/authclient"
	"admin/server/internal/module/authstate"
	"admin/server/internal/shared/apperror"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SessionStatus string

const (
	SessionStatusActive  SessionStatus = "active"
	SessionStatusExpired SessionStatus = "expired"
	SessionStatusRevoked SessionStatus = "revoked"
)

type AdminSessionQuery struct {
	Page     int
	PageSize int
	Username string
	Platform string
	Status   SessionStatus
}

type AdminSession struct {
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
	Status           SessionStatus
}

type AdminSessionStats struct {
	ActiveTotal int64
	Platforms   map[string]int64
}

type AdminRevokeResult struct {
	Revoked        []Session
	SkippedCurrent int
	SkippedRevoked int
}

type adminSessionRepository interface {
	ListAdmin(context.Context, AdminSessionQuery, time.Time) ([]AdminSession, int64, error)
	StatsAdmin(context.Context, time.Time) (AdminSessionStats, error)
	FindAdminRevokeTargets(context.Context, []int64) ([]Session, error)
	RevokeAdmin(context.Context, []int64, int64, time.Time) (AdminRevokeResult, error)
}

func (s *Service) ListSessions(ctx context.Context, query AdminSessionQuery) ([]AdminSession, int64, error) {
	if err := validateAdminSessionQuery(query); err != nil {
		return nil, 0, err
	}
	if s.adminSessions == nil {
		return nil, 0, apperror.DependencyUnavailable(fmt.Errorf("session administration repository is unavailable"))
	}
	rows, total, err := s.adminSessions.ListAdmin(ctx, query, s.now().UTC())
	if err != nil {
		return nil, 0, apperror.DependencyUnavailable(err)
	}
	return rows, total, nil
}

func (s *Service) SessionStats(ctx context.Context) (AdminSessionStats, error) {
	if s.adminSessions == nil {
		return AdminSessionStats{}, apperror.DependencyUnavailable(fmt.Errorf("session administration repository is unavailable"))
	}
	stats, err := s.adminSessions.StatsAdmin(ctx, s.now().UTC())
	if err != nil {
		return AdminSessionStats{}, apperror.DependencyUnavailable(err)
	}
	return stats, nil
}

func (s *Service) RevokeSession(ctx context.Context, actor Identity, id int64) (AdminRevokeResult, error) {
	return s.revokeAdminSessions(ctx, actor, []int64{id}, true)
}

func (s *Service) RevokeSessions(ctx context.Context, actor Identity, ids []int64) (AdminRevokeResult, error) {
	return s.revokeAdminSessions(ctx, actor, ids, false)
}

func (s *Service) revokeAdminSessions(ctx context.Context, actor Identity, ids []int64, single bool) (AdminRevokeResult, error) {
	if actor.SessionID < 1 || actor.UserID < 1 {
		return AdminRevokeResult{}, apperror.Unauthorized(fmt.Errorf("authentication identity is missing"))
	}
	normalized := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id < 1 {
			return AdminRevokeResult{}, apperror.InvalidRequest(fmt.Errorf("session ids must be positive integers"))
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	if len(normalized) == 0 || len(normalized) > 100 {
		return AdminRevokeResult{}, apperror.InvalidRequest(fmt.Errorf("session ids must contain 1 to 100 items"))
	}
	if s.adminSessions == nil {
		return AdminRevokeResult{}, apperror.DependencyUnavailable(fmt.Errorf("session administration repository is unavailable"))
	}
	targets, err := s.adminSessions.FindAdminRevokeTargets(ctx, normalized)
	if err != nil {
		return AdminRevokeResult{}, apperror.DependencyUnavailable(err)
	}
	if single {
		if len(targets) == 0 {
			return AdminRevokeResult{}, sessionNotFound(fmt.Errorf("session %d does not exist", normalized[0]))
		}
		if targets[0].ID == actor.SessionID {
			return AdminRevokeResult{}, sessionCurrentProtected(fmt.Errorf("session %d is the current session", normalized[0]))
		}
	}

	mutationTargets := make([]Session, 0, len(targets))
	for _, target := range targets {
		if target.ID != actor.SessionID && target.RevokedAt == nil {
			mutationTargets = append(mutationTargets, target)
		}
	}
	if len(mutationTargets) == 0 {
		result, revokeErr := s.adminSessions.RevokeAdmin(ctx, normalized, actor.SessionID, s.now().UTC())
		if revokeErr != nil {
			return AdminRevokeResult{}, apperror.DependencyUnavailable(revokeErr)
		}
		return result, nil
	}
	if s.invalidator == nil || s.states == nil || s.sessionCache == nil {
		return AdminRevokeResult{}, apperror.DependencyUnavailable(fmt.Errorf("session invalidation dependencies are unavailable"))
	}

	groups := make(map[string]authstate.SessionsFact)
	for _, session := range mutationTargets {
		key := fmt.Sprintf("%d:%s", session.UserID, session.Platform)
		if _, exists := groups[key]; exists {
			continue
		}
		fact, factErr := s.ensureSessionsReady(ctx, session.Platform, session.UserID)
		if factErr != nil {
			return AdminRevokeResult{}, apperror.DependencyUnavailable(factErr)
		}
		groups[key] = fact
	}
	priorFacts := make([]authstate.SessionsFact, 0, len(groups))
	nextFacts := make([]authstate.SessionsFact, 0, len(groups))
	for _, fact := range groups {
		nextGeneration, generationErr := authstate.NewGeneration()
		if generationErr != nil {
			return AdminRevokeResult{}, apperror.Internal(generationErr)
		}
		priorFacts = append(priorFacts, fact)
		nextFacts = append(nextFacts, authstate.SessionsFact{Platform: fact.Platform, UserID: fact.UserID, Generation: nextGeneration})
	}
	lease, acquireErr := s.invalidator.Acquire(ctx, authstate.MutationFacts{Sessions: priorFacts})
	if acquireErr != nil {
		return AdminRevokeResult{}, apperror.DependencyUnavailable(acquireErr)
	}
	mutationCtx, stopRenewal := lease.StartRenewal(ctx)
	result, revokeErr := s.adminSessions.RevokeAdmin(mutationCtx, normalized, actor.SessionID, s.now().UTC())
	renewalCause := context.Cause(mutationCtx)
	stopRenewal()
	if revokeErr != nil || renewalCause != nil {
		return AdminRevokeResult{}, apperror.DependencyUnavailable(errors.Join(revokeErr, renewalCause, lease.Rollback(ctx)))
	}
	if len(result.Revoked) == 0 {
		if rollbackErr := lease.Rollback(ctx); rollbackErr != nil {
			return AdminRevokeResult{}, apperror.DependencyUnavailable(rollbackErr)
		}
		return result, nil
	}
	if commitErr := lease.Commit(ctx, authstate.MutationFacts{Sessions: nextFacts}); commitErr != nil {
		return AdminRevokeResult{}, apperror.DependencyUnavailable(commitErr)
	}
	if err := s.sessionCache.DeleteMany(ctx, result.Revoked); err != nil {
		return AdminRevokeResult{}, apperror.DependencyUnavailable(err)
	}
	return result, nil
}

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

func (r *SessionRepository) ListAdmin(ctx context.Context, query AdminSessionQuery, now time.Time) ([]AdminSession, int64, error) {
	db := r.db.WithContext(ctx).Table("sys_user_session AS session").
		Select("session.id, session.user_id, app_user.username, session.platform, session.device_id, "+
			"session.client_ip, session.user_agent, session.created_at, session.updated_at, "+
			"session.refresh_expires_at, session.revoked_at, "+
			"CASE WHEN session.revoked_at IS NOT NULL THEN 'revoked' "+
			"WHEN session.refresh_expires_at <= ? THEN 'expired' ELSE 'active' END AS status", now.UTC()).
		Joins("JOIN sys_user AS app_user ON app_user.id = session.user_id").
		Where("app_user.deleted_at IS NULL")
	if query.Username != "" {
		db = db.Where("app_user.username LIKE ? ESCAPE '\\'", adminPrefixPattern(query.Username))
	}
	if query.Platform != "" {
		if err := authclient.ValidatePlatform(query.Platform); err != nil {
			return nil, 0, apperror.InvalidRequest(err)
		}
		db = db.Where("session.platform = ?", query.Platform)
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

func (r *SessionRepository) StatsAdmin(ctx context.Context, now time.Time) (AdminSessionStats, error) {
	stats := AdminSessionStats{Platforms: make(map[string]int64)}
	if err := r.db.WithContext(ctx).Table("sys_user_session AS session").
		Joins("JOIN sys_user AS app_user ON app_user.id = session.user_id").
		Where("app_user.deleted_at IS NULL AND session.revoked_at IS NULL AND session.refresh_expires_at > ?", now.UTC()).
		Count(&stats.ActiveTotal).Error; err != nil {
		return AdminSessionStats{}, fmt.Errorf("count active sessions: %w", err)
	}
	type platformCount struct {
		Platform string
		Count    int64
	}
	rows := make([]platformCount, 0)
	if err := r.db.WithContext(ctx).Table("sys_user_session AS session").
		Select("session.platform, COUNT(*) AS count").
		Joins("JOIN sys_user AS app_user ON app_user.id = session.user_id").
		Where("app_user.deleted_at IS NULL AND session.revoked_at IS NULL AND session.refresh_expires_at > ?", now.UTC()).
		Group("session.platform").Scan(&rows).Error; err != nil {
		return AdminSessionStats{}, fmt.Errorf("count active sessions by platform: %w", err)
	}
	for _, row := range rows {
		stats.Platforms[row.Platform] = row.Count
	}
	return stats, nil
}

func (r *SessionRepository) RevokeAdmin(ctx context.Context, ids []int64, currentSessionID int64, now time.Time) (AdminRevokeResult, error) {
	result := AdminRevokeResult{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rows := make([]Session, 0, len(ids))
		if err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", ids).Find(&rows).Error; err != nil {
			return fmt.Errorf("lock sessions for revoke: %w", err)
		}
		for _, session := range rows {
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

func (r *SessionRepository) FindAdminRevokeTargets(ctx context.Context, ids []int64) ([]Session, error) {
	rows := make([]Session, 0, len(ids))
	if err := r.db.WithContext(ctx).Unscoped().Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("find sessions for admin revoke: %w", err)
	}
	return rows, nil
}

func adminPrefixPattern(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	value = strings.ReplaceAll(value, "_", "\\_")
	return value + "%"
}

func validateAdminSessionQuery(query AdminSessionQuery) error {
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 {
		return apperror.InvalidRequest(fmt.Errorf("pagination is invalid"))
	}
	if query.Platform != "" {
		if err := authclient.ValidatePlatform(query.Platform); err != nil {
			return apperror.InvalidRequest(err)
		}
	}
	if query.Status != "" && query.Status != SessionStatusActive && query.Status != SessionStatusExpired && query.Status != SessionStatusRevoked {
		return apperror.InvalidRequest(fmt.Errorf("session status is invalid"))
	}
	return nil
}
