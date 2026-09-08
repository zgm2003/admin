package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"admin/server/internal/module/auth/client"
	"admin/server/internal/module/auth/state"
	"admin/server/internal/shared/apperror"
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
	DeleteMany(context.Context, []authstate.SessionReference) error
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
	refs := make([]authstate.SessionReference, 0, len(result.Revoked))
	for _, session := range result.Revoked {
		refs = append(refs, authstate.SessionReference{Platform: session.Platform, SessionID: session.ID})
	}
	if err := s.cache.DeleteMany(ctx, refs); err != nil {
		return AdminRevokeResult{}, apperror.DependencyUnavailable(err)
	}
	return result, nil
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
