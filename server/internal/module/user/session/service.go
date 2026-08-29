package session

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
	Revoked        []Record
	SkippedCurrent int
	SkippedRevoked int
}

type Actor struct {
	UserID    int64
	SessionID int64
}

type cacheDeleter interface {
	DeleteMany(context.Context, []Record) error
}

type adminRepository interface {
	ListAdmin(context.Context, AdminSessionQuery, time.Time) ([]AdminSession, int64, error)
	StatsAdmin(context.Context, time.Time) (AdminSessionStats, error)
	FindAdminRevokeTargets(context.Context, []int64) ([]Record, error)
	RevokeAdmin(context.Context, []int64, int64, time.Time) (AdminRevokeResult, error)
}

type Service struct {
	repository  adminRepository
	states      *authstate.Store
	invalidator *authstate.Invalidator
	cache       cacheDeleter
	now         func() time.Time
}

func NewService(repository adminRepository, states *authstate.Store, invalidator *authstate.Invalidator, cache cacheDeleter) *Service {
	return &Service{repository: repository, states: states, invalidator: invalidator, cache: cache, now: time.Now}
}

func (s *Service) ListSessions(ctx context.Context, query AdminSessionQuery) ([]AdminSession, int64, error) {
	if err := validateAdminSessionQuery(query); err != nil {
		return nil, 0, err
	}
	if s == nil || s.repository == nil {
		return nil, 0, apperror.DependencyUnavailable(fmt.Errorf("session administration repository is unavailable"))
	}
	rows, total, err := s.repository.ListAdmin(ctx, query, s.now().UTC())
	if err != nil {
		return nil, 0, apperror.DependencyUnavailable(err)
	}
	return rows, total, nil
}

func (s *Service) SessionStats(ctx context.Context) (AdminSessionStats, error) {
	if s == nil || s.repository == nil {
		return AdminSessionStats{}, apperror.DependencyUnavailable(fmt.Errorf("session administration repository is unavailable"))
	}
	stats, err := s.repository.StatsAdmin(ctx, s.now().UTC())
	if err != nil {
		return AdminSessionStats{}, apperror.DependencyUnavailable(err)
	}
	return stats, nil
}

func (s *Service) RevokeSession(ctx context.Context, actor Actor, id int64) (AdminRevokeResult, error) {
	return s.revokeAdminSessions(ctx, actor, []int64{id}, true)
}

func (s *Service) RevokeSessions(ctx context.Context, actor Actor, ids []int64) (AdminRevokeResult, error) {
	return s.revokeAdminSessions(ctx, actor, ids, false)
}

func (s *Service) revokeAdminSessions(ctx context.Context, actor Actor, ids []int64, single bool) (AdminRevokeResult, error) {
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
	if s.repository == nil {
		return AdminRevokeResult{}, apperror.DependencyUnavailable(fmt.Errorf("session administration repository is unavailable"))
	}
	targets, err := s.repository.FindAdminRevokeTargets(ctx, normalized)
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

	mutationTargets := make([]Record, 0, len(targets))
	for _, target := range targets {
		if target.ID != actor.SessionID && target.RevokedAt == nil {
			mutationTargets = append(mutationTargets, target)
		}
	}
	if len(mutationTargets) == 0 {
		result, revokeErr := s.repository.RevokeAdmin(ctx, normalized, actor.SessionID, s.now().UTC())
		if revokeErr != nil {
			return AdminRevokeResult{}, apperror.DependencyUnavailable(revokeErr)
		}
		return result, nil
	}
	if s.invalidator == nil || s.states == nil || s.cache == nil {
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
	result, revokeErr := s.repository.RevokeAdmin(mutationCtx, normalized, actor.SessionID, s.now().UTC())
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
	if err := s.cache.DeleteMany(ctx, result.Revoked); err != nil {
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

func (r *Repository) ListAdmin(ctx context.Context, query AdminSessionQuery, now time.Time) ([]AdminSession, int64, error) {
	db := r.db.WithContext(ctx).Table("user_session AS session").
		Select("session.id, session.user_id, app_user.username, platform.code AS platform, session.device_id, "+
			"session.client_ip, session.user_agent, session.created_at, session.updated_at, "+
			"session.refresh_expires_at, session.revoked_at, "+
			"CASE WHEN session.revoked_at IS NOT NULL THEN 'revoked' "+
			"WHEN session.refresh_expires_at <= ? THEN 'expired' ELSE 'active' END AS status", now.UTC()).
		Joins("JOIN user_account AS app_user ON app_user.id = session.user_id").
		Joins("JOIN auth_platform AS platform ON platform.id = session.platform_id").
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
		Joins("JOIN auth_platform AS platform ON platform.id = session.platform_id").
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
		rows := make([]Record, 0, len(ids))
		if err := tx.Unscoped().Table("user_session AS session").
			Select("session.*, platform.code AS platform").
			Joins("JOIN auth_platform AS platform ON platform.id = session.platform_id").
			Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "session"}}).
			Where("session.id IN ?", ids).Scan(&rows).Error; err != nil {
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

func (r *Repository) FindAdminRevokeTargets(ctx context.Context, ids []int64) ([]Record, error) {
	rows := make([]Record, 0, len(ids))
	if err := r.db.WithContext(ctx).Unscoped().Table("user_session AS session").
		Select("session.*, platform.code AS platform").
		Joins("JOIN auth_platform AS platform ON platform.id = session.platform_id").
		Where("session.id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("find sessions for admin revoke: %w", err)
	}
	return rows, nil
}

func (s *Service) ensureSessionsReady(ctx context.Context, platform string, userID int64) (authstate.SessionsFact, error) {
	state, found, err := s.states.ReadSessions(ctx, platform, userID)
	if err == nil && found {
		if state.State == authstate.StateInvalidating {
			return authstate.SessionsFact{}, authstate.ErrUpdating
		}
		return state.Fact(), nil
	}
	generation, generationErr := authstate.NewGeneration()
	if generationErr != nil {
		return authstate.SessionsFact{}, generationErr
	}
	fact := authstate.SessionsFact{Platform: platform, UserID: userID, Generation: generation}
	installed, _, installErr := s.states.InstallSessionsReadyIfMissing(ctx, fact)
	if installErr != nil {
		return authstate.SessionsFact{}, errors.Join(err, installErr)
	}
	if installed.State == authstate.StateInvalidating {
		return authstate.SessionsFact{}, authstate.ErrUpdating
	}
	return installed.Fact(), nil
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
