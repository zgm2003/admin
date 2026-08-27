package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"admin/server/internal/module/authclient"
	"admin/server/internal/module/authplatform"
	"admin/server/internal/module/authstate"
	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type userStore interface {
	CreateWithRole(context.Context, user.CreateInput) (user.User, error)
	FindCredentialByEmail(context.Context, string) (user.Credential, error)
	FindCurrent(context.Context, int64) (user.Current, error)
}

type roleStore interface {
	FindDefault(context.Context) (role.Role, error)
}

type sessionStore interface {
	CreateWithinLimit(context.Context, SessionCreate, authplatform.Policy, time.Time) (Session, []Session, error)
	FindAuthoritative(context.Context, TokenIdentity, time.Time) (SessionAuthority, error)
	FindByRefreshHash(context.Context, string, string, time.Time) (SessionAuthority, error)
	RotateByRefreshHash(context.Context, int64, string, string, string, time.Time, authclient.Client) (Session, bool, error)
	Revoke(context.Context, int64, time.Time) error
}

type policyStore interface {
	CurrentPolicy(context.Context, string) (authplatform.Policy, error)
}

type Identity struct {
	UserID         int64
	SessionID      int64
	Platform       string
	Version        int64
	PolicyVersion  int64
	AccessCacheTTL time.Duration
	CacheResult    string
}

type Service struct {
	users               userStore
	roles               roleStore
	sessions            sessionStore
	policies            policyStore
	states              *authstate.Store
	invalidator         *authstate.Invalidator
	sessionCache        *SessionCache
	adminSessions       adminSessionRepository
	redis               *projectredis.Client
	jwt                 *JWT
	refreshTokenHMACKey []byte
	comparePassword     func(string, string) error
	logger              *slog.Logger
	now                 func() time.Time
}

func NewService(
	users userStore,
	roles roleStore,
	sessions sessionStore,
	policies policyStore,
	states *authstate.Store,
	invalidator *authstate.Invalidator,
	sessionCache *SessionCache,
	redis *projectredis.Client,
	jwt *JWT,
	refreshTokenHMACKey []byte,
	logger *slog.Logger,
) *Service {
	return &Service{
		users: users, roles: roles, sessions: sessions, policies: policies, states: states,
		invalidator: invalidator, sessionCache: sessionCache, redis: redis, jwt: jwt,
		refreshTokenHMACKey: append([]byte(nil), refreshTokenHMACKey...), comparePassword: VerifyPassword, logger: logger, now: time.Now,
	}
}

func (s *Service) SetSessionAdminRepository(repository adminSessionRepository) {
	s.adminSessions = repository
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (Registered, error) {
	policy, err := s.policies.CurrentPolicy(ctx, input.Client.Platform)
	if err != nil {
		return Registered{}, err
	}
	if !policy.AllowRegister {
		return Registered{}, apperror.Forbidden(fmt.Errorf("registration is disabled for authentication platform %q", policy.Code))
	}
	normalized, err := validateAccountInput(input.Username, input.Email, input.Password, input.ConfirmPassword)
	if err != nil {
		return Registered{}, err
	}
	if err := s.redis.Ping(ctx); err != nil {
		return Registered{}, apperror.DependencyUnavailable(err)
	}
	defaultRole, err := s.roles.FindDefault(ctx)
	if err != nil {
		return Registered{}, apperror.Internal(err)
	}
	passwordHash, err := HashPassword(normalized.Password)
	if err != nil {
		return Registered{}, apperror.Internal(err)
	}
	created, err := s.users.CreateWithRole(ctx, user.CreateInput{
		Username: normalized.Username, Email: normalized.Email, PasswordHash: passwordHash, RoleID: defaultRole.ID,
	})
	if err != nil {
		return Registered{}, mapUserCreateError(err)
	}
	return Registered{UserID: created.ID, Username: created.Username, Email: created.Email}, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (Credential, error) {
	policy, err := s.policies.CurrentPolicy(ctx, input.Client.Platform)
	if err != nil {
		return Credential{}, err
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return Credential{}, apperror.InvalidRequest(err)
	}
	if input.Password == "" {
		return Credential{}, apperror.InvalidRequest(fmt.Errorf("email and password are required"))
	}
	credential, err := s.users.FindCredentialByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = s.comparePassword(missingCredentialPasswordHash, input.Password)
			return Credential{}, invalidCredentialError(err)
		}
		return Credential{}, apperror.DependencyUnavailable(err)
	}
	if credential.IsEnabled != yesno.Yes {
		return Credential{}, apperror.Forbidden(fmt.Errorf("user is disabled"))
	}
	if err := s.comparePassword(credential.PasswordHash, input.Password); err != nil {
		return Credential{}, invalidCredentialError(err)
	}

	userFact, err := s.ensureUserReady(ctx, credential.ID, true, false)
	if err != nil {
		return Credential{}, mapStateMutationError(err)
	}
	sessionsFact, err := s.ensureSessionsReady(ctx, input.Client.Platform, credential.ID)
	if err != nil {
		return Credential{}, mapStateMutationError(err)
	}
	lease, err := s.invalidator.Acquire(ctx, authstate.MutationFacts{Sessions: []authstate.SessionsFact{sessionsFact}})
	if err != nil {
		return Credential{}, mapStateMutationError(err)
	}

	now := s.now().UTC()
	refreshToken, err := newRefreshToken()
	if err != nil {
		_ = lease.Rollback(ctx)
		return Credential{}, apperror.Internal(err)
	}
	refreshExpiresAt := now.Add(policy.RefreshTTL)
	mutationCtx, stopRenewal := lease.StartRenewal(ctx)
	created, revoked, createErr := s.sessions.CreateWithinLimit(mutationCtx, SessionCreate{
		UserID: credential.ID, Platform: input.Client.Platform, DeviceID: input.Client.DeviceID,
		RefreshTokenHash: s.hashRefreshToken(refreshToken), ClientIP: input.Client.ClientIP,
		UserAgent: input.Client.UserAgent, RefreshExpiresAt: refreshExpiresAt,
	}, policy, now)
	renewalCause := context.Cause(mutationCtx)
	stopRenewal()
	if createErr != nil || renewalCause != nil {
		rollbackErr := lease.Rollback(ctx)
		if errors.Is(createErr, gorm.ErrRecordNotFound) {
			return Credential{}, apperror.Forbidden(errors.Join(createErr, renewalCause, rollbackErr))
		}
		return Credential{}, apperror.DependencyUnavailable(errors.Join(createErr, renewalCause, rollbackErr))
	}
	nextGeneration, err := authstate.NewGeneration()
	if err != nil {
		return Credential{}, apperror.Internal(err)
	}
	nextSessionsFact := authstate.SessionsFact{Platform: input.Client.Platform, UserID: credential.ID, Generation: nextGeneration}
	if err := lease.Commit(ctx, authstate.MutationFacts{Sessions: []authstate.SessionsFact{nextSessionsFact}}); err != nil {
		return Credential{}, apperror.DependencyUnavailable(err)
	}
	if err := s.sessionCache.DeleteMany(ctx, revoked); err != nil {
		return Credential{}, apperror.DependencyUnavailable(err)
	}
	created.RefreshExpiresAt = refreshExpiresAt
	if created.Platform == "" {
		created.Platform = input.Client.Platform
	}
	if created.DeviceID == "" {
		created.DeviceID = input.Client.DeviceID
	}
	if created.ClientIP == "" {
		created.ClientIP = input.Client.ClientIP
	}
	authority := SessionAuthority{Session: created, UserID: credential.ID, UserIsEnabled: credential.IsEnabled}
	if err := s.publishAuthority(ctx, authority, policy, userFact.Generation, nextSessionsFact.Generation, now); err != nil {
		return Credential{}, err
	}
	accessToken, accessExpiresAt, err := s.jwt.Issue(TokenIdentity{
		UserID: credential.ID, SessionID: created.ID, Platform: input.Client.Platform, Version: created.Version,
	}, policy.AccessTTL)
	if err != nil {
		return Credential{}, apperror.Internal(err)
	}
	return Credential{
		AccessToken: accessToken, ExpiresIn: int(accessExpiresAt.Sub(now).Seconds()), RefreshToken: refreshToken,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

func (s *Service) Authenticate(ctx context.Context, accessToken string, client authclient.Client) (Identity, error) {
	token, err := s.jwt.Parse(accessToken)
	if err != nil {
		return Identity{}, apperror.Unauthorized(err)
	}
	if token.Platform != client.Platform {
		return Identity{}, apperror.Unauthorized(fmt.Errorf("Access Token platform does not match request platform"))
	}
	policy, err := s.policies.CurrentPolicy(ctx, client.Platform)
	if err != nil {
		return Identity{}, err
	}
	now := s.now().UTC()
	cached, hit, cacheResult, err := s.cachedIdentity(ctx, token, client, policy, now)
	if err != nil {
		return Identity{}, err
	}
	if hit {
		cached.CacheResult = "hit"
		return cached, nil
	}

	authority, err := s.sessions.FindAuthoritative(ctx, token, now)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Identity{}, apperror.Unauthorized(err)
		}
		return Identity{}, apperror.DependencyUnavailable(err)
	}
	if err := validateAuthority(authority); err != nil {
		return Identity{}, apperror.Unauthorized(err)
	}
	if err := s.enforceClient(ctx, authority, policy, client); err != nil {
		return Identity{}, err
	}
	identity := identityFromAuthority(authority, policy, cacheResult)
	userFact, userErr := s.ensureUserReady(ctx, authority.UserID, authority.UserIsEnabled == yesno.Yes, authority.UserDeleted)
	if userErr != nil {
		if isStateUpdating(userErr) {
			return Identity{}, authplatform.SessionUpdating(userErr)
		}
		s.logCacheError(ctx, "userState", userErr)
		identity.CacheResult = "error"
		return identity, nil
	}
	sessionsFact, sessionsErr := s.ensureSessionsReady(ctx, authority.Session.Platform, authority.UserID)
	if sessionsErr != nil {
		if isStateUpdating(sessionsErr) {
			return Identity{}, authplatform.SessionUpdating(sessionsErr)
		}
		s.logCacheError(ctx, "sessionState", sessionsErr)
		identity.CacheResult = "error"
		return identity, nil
	}
	currentPolicy, policyErr := s.policies.CurrentPolicy(ctx, client.Platform)
	if policyErr != nil {
		return Identity{}, policyErr
	}
	if currentPolicy.PolicyVersion != policy.PolicyVersion {
		return Identity{}, authplatform.SessionUpdating(fmt.Errorf("authentication policy changed during session rebuild"))
	}
	if err := s.publishAuthority(ctx, authority, policy, userFact.Generation, sessionsFact.Generation, now); err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Code == authplatform.CodeSessionUpdating {
			return Identity{}, err
		}
		s.logCacheError(ctx, "sessionSnapshot", err)
		identity.CacheResult = "error"
	}
	return identity, nil
}

func (s *Service) Refresh(ctx context.Context, input RefreshInput) (Credential, error) {
	if input.RefreshToken == "" {
		return Credential{}, apperror.Unauthorized(fmt.Errorf("Refresh Token is required"))
	}
	policy, err := s.policies.CurrentPolicy(ctx, input.Client.Platform)
	if err != nil {
		return Credential{}, err
	}
	now := s.now().UTC()
	oldHash := s.hashRefreshToken(input.RefreshToken)
	authority, err := s.sessions.FindByRefreshHash(ctx, input.Client.Platform, oldHash, now)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Credential{}, apperror.Unauthorized(err)
		}
		return Credential{}, apperror.DependencyUnavailable(err)
	}
	if err := validateAuthority(authority); err != nil {
		return Credential{}, apperror.Unauthorized(err)
	}
	if err := s.enforceClient(ctx, authority, policy, input.Client); err != nil {
		return Credential{}, err
	}
	userFact, err := s.ensureUserReady(ctx, authority.UserID, true, false)
	if err != nil {
		return Credential{}, mapStateMutationError(err)
	}
	sessionsFact, err := s.ensureSessionsReady(ctx, input.Client.Platform, authority.UserID)
	if err != nil {
		return Credential{}, mapStateMutationError(err)
	}
	lease, err := s.invalidator.Acquire(ctx, authstate.MutationFacts{Sessions: []authstate.SessionsFact{sessionsFact}})
	if err != nil {
		return Credential{}, mapStateMutationError(err)
	}
	newToken, err := newRefreshToken()
	if err != nil {
		_ = lease.Rollback(ctx)
		return Credential{}, apperror.Internal(err)
	}
	mutationCtx, stopRenewal := lease.StartRenewal(ctx)
	rotated, won, rotateErr := s.sessions.RotateByRefreshHash(
		mutationCtx, authority.Session.ID, input.Client.Platform, oldHash, s.hashRefreshToken(newToken), now, input.Client,
	)
	renewalCause := context.Cause(mutationCtx)
	stopRenewal()
	if rotateErr != nil || renewalCause != nil {
		return Credential{}, apperror.DependencyUnavailable(errors.Join(rotateErr, renewalCause, lease.Rollback(ctx)))
	}
	if !won {
		return Credential{}, apperror.Unauthorized(errors.Join(fmt.Errorf("Refresh Token was already used"), lease.Rollback(ctx)))
	}
	nextGeneration, err := authstate.NewGeneration()
	if err != nil {
		return Credential{}, apperror.Internal(err)
	}
	nextSessionsFact := authstate.SessionsFact{Platform: input.Client.Platform, UserID: authority.UserID, Generation: nextGeneration}
	if err := lease.Commit(ctx, authstate.MutationFacts{Sessions: []authstate.SessionsFact{nextSessionsFact}}); err != nil {
		return Credential{}, apperror.DependencyUnavailable(err)
	}
	rotatedAuthority := SessionAuthority{Session: rotated, UserID: authority.UserID, UserIsEnabled: authority.UserIsEnabled, UserDeleted: authority.UserDeleted}
	if err := s.publishAuthority(ctx, rotatedAuthority, policy, userFact.Generation, nextSessionsFact.Generation, now); err != nil {
		return Credential{}, err
	}
	accessToken, accessExpiresAt, err := s.jwt.Issue(TokenIdentity{
		UserID: authority.UserID, SessionID: rotated.ID, Platform: input.Client.Platform, Version: rotated.Version,
	}, policy.AccessTTL)
	if err != nil {
		return Credential{}, apperror.Internal(err)
	}
	return Credential{
		AccessToken: accessToken, ExpiresIn: int(accessExpiresAt.Sub(now).Seconds()), RefreshToken: newToken,
		RefreshExpiresAt: rotated.RefreshExpiresAt,
	}, nil
}

func (s *Service) Logout(ctx context.Context, identity Identity, client authclient.Client) error {
	if err := validateSessionIdentity(identity); err != nil {
		return apperror.Unauthorized(err)
	}
	if identity.Platform != client.Platform {
		return apperror.Unauthorized(fmt.Errorf("session platform does not match request platform"))
	}
	return s.revokeSession(ctx, identity.UserID, identity.SessionID, identity.Platform)
}

func (s *Service) CurrentUser(ctx context.Context, identity Identity) (user.Current, error) {
	if err := validateSessionIdentity(identity); err != nil {
		return user.Current{}, apperror.Unauthorized(err)
	}
	current, err := s.users.FindCurrent(ctx, identity.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return user.Current{}, apperror.Unauthorized(err)
		}
		return user.Current{}, apperror.DependencyUnavailable(err)
	}
	return current, nil
}

func (s *Service) cachedIdentity(ctx context.Context, token TokenIdentity, client authclient.Client, policy authplatform.Policy, now time.Time) (Identity, bool, string, error) {
	userState, userFound, userErr := s.states.ReadUser(ctx, token.UserID)
	if userErr != nil {
		s.logCacheError(ctx, "userState", userErr)
		return Identity{}, false, "error", nil
	}
	if userFound && userState.State == authstate.StateInvalidating {
		return Identity{}, false, "", authplatform.SessionUpdating(authstate.ErrUpdating)
	}
	if !userFound {
		return Identity{}, false, "miss", nil
	}
	if userState.Deleted || !userState.IsEnabled {
		return Identity{}, false, "", apperror.Unauthorized(fmt.Errorf("user is unavailable"))
	}
	sessionsState, sessionsFound, sessionsErr := s.states.ReadSessions(ctx, token.Platform, token.UserID)
	if sessionsErr != nil {
		s.logCacheError(ctx, "sessionState", sessionsErr)
		return Identity{}, false, "error", nil
	}
	if sessionsFound && sessionsState.State == authstate.StateInvalidating {
		return Identity{}, false, "", authplatform.SessionUpdating(authstate.ErrUpdating)
	}
	if !sessionsFound {
		return Identity{}, false, "miss", nil
	}
	snapshot, snapshotFound, snapshotErr := s.sessionCache.Read(ctx, token.Platform, token.SessionID)
	if snapshotErr != nil {
		s.logCacheError(ctx, "sessionSnapshot", snapshotErr)
		return Identity{}, false, "error", nil
	}
	if !snapshotFound {
		return Identity{}, false, "miss", nil
	}
	if snapshot.UserID != token.UserID || snapshot.SessionVersion != token.Version || snapshot.PolicyVersion != policy.PolicyVersion ||
		snapshot.UserGeneration != userState.Generation || snapshot.SessionsGeneration != sessionsState.Generation {
		return Identity{}, false, "miss", nil
	}
	if snapshot.Revoked || !snapshot.RefreshExpiresAt.After(now) {
		return Identity{}, false, "", apperror.Unauthorized(fmt.Errorf("session is unavailable"))
	}
	authority := SessionAuthority{
		Session: Session{
			ID: snapshot.SessionID, UserID: snapshot.UserID, Platform: snapshot.Platform, DeviceID: snapshot.DeviceID,
			Version: snapshot.SessionVersion, ClientIP: snapshot.ClientIP, RefreshExpiresAt: snapshot.RefreshExpiresAt,
		},
		UserID: snapshot.UserID, UserIsEnabled: yesno.Yes,
	}
	if err := s.enforceClient(ctx, authority, policy, client); err != nil {
		return Identity{}, false, "", err
	}
	return identityFromAuthority(authority, policy, "hit"), true, "hit", nil
}

func (s *Service) enforceClient(ctx context.Context, authority SessionAuthority, policy authplatform.Policy, client authclient.Client) error {
	if (!policy.BindDevice || authority.Session.DeviceID == client.DeviceID) && (!policy.BindIP || authority.Session.ClientIP == client.ClientIP) {
		return nil
	}
	if err := s.revokeSession(ctx, authority.UserID, authority.Session.ID, authority.Session.Platform); err != nil {
		return err
	}
	return apperror.Unauthorized(fmt.Errorf("session client binding does not match"))
}

func (s *Service) revokeSession(ctx context.Context, userID, sessionID int64, platform string) error {
	sessionsFact, err := s.ensureSessionsReady(ctx, platform, userID)
	if err != nil {
		return mapStateMutationError(err)
	}
	lease, err := s.invalidator.Acquire(ctx, authstate.MutationFacts{Sessions: []authstate.SessionsFact{sessionsFact}})
	if err != nil {
		return mapStateMutationError(err)
	}
	mutationCtx, stopRenewal := lease.StartRenewal(ctx)
	revokeErr := s.sessions.Revoke(mutationCtx, sessionID, s.now().UTC())
	renewalCause := context.Cause(mutationCtx)
	stopRenewal()
	if revokeErr != nil || renewalCause != nil {
		return apperror.DependencyUnavailable(errors.Join(revokeErr, renewalCause, lease.Rollback(ctx)))
	}
	nextGeneration, err := authstate.NewGeneration()
	if err != nil {
		return apperror.Internal(err)
	}
	if err := lease.Commit(ctx, authstate.MutationFacts{Sessions: []authstate.SessionsFact{{
		Platform: platform, UserID: userID, Generation: nextGeneration,
	}}}); err != nil {
		return apperror.DependencyUnavailable(err)
	}
	if err := s.sessionCache.Delete(ctx, platform, sessionID); err != nil {
		return apperror.DependencyUnavailable(err)
	}
	return nil
}

func (s *Service) ensureUserReady(ctx context.Context, userID int64, enabled, deleted bool) (authstate.UserFact, error) {
	state, found, err := s.states.ReadUser(ctx, userID)
	if err == nil && found {
		if state.State == authstate.StateInvalidating {
			return authstate.UserFact{}, authstate.ErrUpdating
		}
		if state.IsEnabled != enabled || state.Deleted != deleted {
			return authstate.UserFact{}, authstate.ErrGenerationChanged
		}
		return state.Fact(), nil
	}
	generation, generationErr := authstate.NewGeneration()
	if generationErr != nil {
		return authstate.UserFact{}, generationErr
	}
	fact := authstate.UserFact{UserID: userID, Generation: generation, IsEnabled: enabled, Deleted: deleted}
	installed, _, installErr := s.states.InstallUserReadyIfMissing(ctx, fact)
	if installErr != nil {
		return authstate.UserFact{}, errors.Join(err, installErr)
	}
	if installed.State == authstate.StateInvalidating {
		return authstate.UserFact{}, authstate.ErrUpdating
	}
	if installed.IsEnabled != enabled || installed.Deleted != deleted {
		return authstate.UserFact{}, authstate.ErrGenerationChanged
	}
	return installed.Fact(), nil
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

func (s *Service) publishAuthority(ctx context.Context, authority SessionAuthority, policy authplatform.Policy, userGeneration, sessionsGeneration string, now time.Time) error {
	snapshot := snapshotFromAuthority(authority, policy, userGeneration, sessionsGeneration)
	ttl := policy.SessionCacheTTL
	if remaining := snapshot.RefreshExpiresAt.Sub(now); remaining < ttl {
		ttl = remaining
	}
	if ttl <= 0 {
		return apperror.Unauthorized(fmt.Errorf("session Refresh Token is expired"))
	}
	published, err := s.sessionCache.PublishIfCurrent(ctx, snapshot, ttl)
	if err != nil {
		return apperror.DependencyUnavailable(err)
	}
	if !published {
		return authplatform.SessionUpdating(authstate.ErrGenerationChanged)
	}
	return nil
}

func snapshotFromAuthority(authority SessionAuthority, policy authplatform.Policy, userGeneration, sessionsGeneration string) SessionSnapshot {
	return SessionSnapshot{
		SchemaVersion: sessionSnapshotSchemaVersion, UserID: authority.UserID, SessionID: authority.Session.ID,
		Platform: authority.Session.Platform, SessionVersion: authority.Session.Version, PolicyVersion: policy.PolicyVersion,
		UserGeneration: userGeneration, SessionsGeneration: sessionsGeneration, DeviceID: authority.Session.DeviceID,
		ClientIP: authority.Session.ClientIP, RefreshExpiresAt: authority.Session.RefreshExpiresAt.UTC(), Revoked: authority.Session.RevokedAt != nil,
	}
}

func identityFromAuthority(authority SessionAuthority, policy authplatform.Policy, cacheResult string) Identity {
	return Identity{
		UserID: authority.UserID, SessionID: authority.Session.ID, Platform: authority.Session.Platform,
		Version: authority.Session.Version, PolicyVersion: policy.PolicyVersion, AccessCacheTTL: policy.AccessCacheTTL,
		CacheResult: cacheResult,
	}
}

func validateAuthority(authority SessionAuthority) error {
	if authority.UserID < 1 || authority.Session.UserID != authority.UserID || authority.Session.ID < 1 || authority.Session.Version < 1 ||
		authority.UserDeleted || authority.UserIsEnabled != yesno.Yes || authority.Session.RevokedAt != nil {
		return fmt.Errorf("authoritative session is unavailable")
	}
	return nil
}

func validateSessionIdentity(identity Identity) error {
	return validateIdentity(TokenIdentity{UserID: identity.UserID, SessionID: identity.SessionID, Platform: identity.Platform, Version: identity.Version})
}

func mapStateMutationError(err error) error {
	if isStateUpdating(err) {
		return authplatform.SessionUpdating(err)
	}
	return apperror.DependencyUnavailable(err)
}

func isStateUpdating(err error) bool {
	return errors.Is(err, authstate.ErrUpdating) || errors.Is(err, authstate.ErrGenerationChanged) || errors.Is(err, authstate.ErrMutationTokenMismatch)
}

func (s *Service) logCacheError(ctx context.Context, kind string, err error) {
	s.logger.ErrorContext(ctx, "authentication cache operation failed", "cacheKind", kind, "cacheResult", "error", "error", err)
}

type normalizedAccount struct {
	Username string
	Email    string
	Password string
}

func validateAccountInput(username, email, password, confirmPassword string) (normalizedAccount, error) {
	username, err := user.NormalizeUsername(username)
	if err != nil {
		return normalizedAccount{}, apperror.InvalidRequest(err)
	}

	email, err = normalizeEmail(email)
	if err != nil {
		return normalizedAccount{}, apperror.InvalidRequest(err)
	}
	if password != confirmPassword {
		return normalizedAccount{}, apperror.InvalidRequest(fmt.Errorf("password confirmation does not match"))
	}
	if err := ValidatePassword(password); err != nil {
		return normalizedAccount{}, apperror.InvalidRequest(err)
	}
	return normalizedAccount{Username: username, Email: email, Password: password}, nil
}

func normalizeEmail(value string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(value))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Name != "" || parsed.Address != email || len(email) > 254 {
		return "", fmt.Errorf("email address is invalid")
	}
	return email, nil
}

func mapUserCreateError(err error) error {
	switch {
	case errors.Is(err, user.ErrUsernameConflict):
		return apperror.Conflict(i18n.KeyUsernameConflict, nil, err)
	case errors.Is(err, user.ErrEmailConflict):
		return apperror.Conflict(i18n.KeyEmailConflict, nil, err)
	default:
		return apperror.DependencyUnavailable(err)
	}
}

func invalidCredentialError(cause error) error {
	return apperror.Unauthorized(cause)
}

func newRefreshToken() (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate Refresh Token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

func (s *Service) hashRefreshToken(token string) string {
	digest := hmac.New(sha256.New, s.refreshTokenHMACKey)
	_, _ = digest.Write([]byte(token))
	return hex.EncodeToString(digest.Sum(nil))
}
