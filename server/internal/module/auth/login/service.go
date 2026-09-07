package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"admin/server/internal/module/auth/client"
	"admin/server/internal/module/auth/platform"
	"admin/server/internal/module/auth/state"
	messagemail "admin/server/internal/module/message/mail"
	"admin/server/internal/module/permission/role"
	user "admin/server/internal/module/user/account"
	"admin/server/internal/module/user/loginlog"
	usersession "admin/server/internal/module/user/session"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type userStore interface {
	CreateWithRole(context.Context, user.CreateInput) (user.User, error)
	FindCredentialByEmail(context.Context, string) (user.Credential, error)
	FindCredentialByIdentity(context.Context, string, string) (user.Credential, error)
	CreateVerifiedIdentity(context.Context, user.VerifiedIdentityInput) (user.User, error)
	FindCurrent(context.Context, int64) (user.Current, error)
}

type passwordStore interface {
	FindCredentialByID(context.Context, int64) (user.Credential, error)
	FindActiveSessionPlatforms(context.Context, int64) ([]string, error)
	ChangePasswordAndRevokeSessions(context.Context, int64, string, time.Time) ([]user.RevokedSessionRef, error)
}

type roleStore interface {
	FindDefault(context.Context) (role.Role, error)
}

type sessionStore interface {
	CreateWithinLimit(context.Context, usersession.CreateInput, authplatform.Policy, time.Time) (usersession.Record, []usersession.Record, error)
	FindAuthoritative(context.Context, int64, int64, string, int64, time.Time) (usersession.Authority, error)
	FindByRefreshHash(context.Context, string, string, time.Time) (usersession.Authority, error)
	RotateByRefreshHash(context.Context, int64, string, string, string, time.Time, authclient.Client) (usersession.Record, bool, error)
	Revoke(context.Context, int64, time.Time) error
}

type policyStore interface {
	CurrentPolicy(context.Context, string) (authplatform.Policy, error)
}

type Identity struct {
	UserID         int64
	SessionID      int64
	PlatformID     int64
	Platform       string
	Version        int64
	PolicyVersion  int64
	AccessCacheTTL time.Duration
	CacheResult    string
}

type Service struct {
	users               userStore
	passwords           passwordStore
	roles               roleStore
	sessions            sessionStore
	policies            policyStore
	states              *authstate.Store
	invalidator         *authstate.Invalidator
	sessionCache        *SessionCache
	redis               *projectredis.Client
	jwt                 *JWT
	refreshTokenHMACKey []byte
	comparePassword     func(string, string) error
	verifyCodeSender    messagemail.VerifyCodeSender
	verificationCodes   VerificationCodeStore
	logger              *slog.Logger
	now                 func() time.Time
	loginLogs           loginLogRecorder
}

type loginLogRecorder interface {
	Record(context.Context, loginlog.Event) error
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

func (s *Service) SetLoginLogRecorder(recorder loginLogRecorder) { s.loginLogs = recorder }

func (s *Service) SetVerifyCodeSender(sender messagemail.VerifyCodeSender) {
	s.verifyCodeSender = sender
}

func (s *Service) SetVerificationCodeStore(store VerificationCodeStore) { s.verificationCodes = store }

func (s *Service) recordLoginEvent(ctx context.Context, event loginlog.Event) error {
	if s.loginLogs == nil {
		return nil
	}
	if err := s.loginLogs.Record(ctx, event); err != nil {
		// Login-log persistence is best-effort: an outage must never change the
		// completed login response.
		if s.logger != nil {
			s.logger.WarnContext(ctx, "login event recording failed", "error", err)
		}
	}
	return nil
}

func stringPointer(value string) *string { return &value }

func (s *Service) SetPasswordStore(store passwordStore) {
	s.passwords = store
}

type ChangePasswordInput struct {
	CurrentPassword string
	NewPassword     string
	ConfirmPassword string
}

func (s *Service) ChangePassword(ctx context.Context, identity Identity, input ChangePasswordInput) error {
	if identity.UserID < 1 || identity.SessionID < 1 || identity.Platform == "" {
		return apperror.Unauthorized(fmt.Errorf("authentication identity is missing"))
	}
	if s.passwords == nil || s.states == nil || s.invalidator == nil || s.sessionCache == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("password change dependencies are unavailable"))
	}
	if input.NewPassword != input.ConfirmPassword {
		return apperror.InvalidRequest(fmt.Errorf("password confirmation does not match"))
	}
	if err := ValidatePassword(input.NewPassword); err != nil {
		return apperror.InvalidRequest(err)
	}
	credential, err := s.passwords.FindCredentialByID(ctx, identity.UserID)
	if err != nil {
		return apperror.DependencyUnavailable(err)
	}
	if err := s.comparePassword(credential.PasswordHash, input.CurrentPassword); err != nil {
		return apperror.Unauthorized(fmt.Errorf("current password is incorrect"))
	}
	passwordHash, err := HashPassword(input.NewPassword)
	if err != nil {
		return apperror.Internal(err)
	}
	platforms, err := s.passwords.FindActiveSessionPlatforms(ctx, identity.UserID)
	if err != nil {
		return apperror.DependencyUnavailable(err)
	}
	facts := make([]authstate.SessionsFact, 0, len(platforms))
	for _, platform := range platforms {
		fact, factErr := s.ensureSessionsReady(ctx, platform, identity.UserID)
		if factErr != nil {
			return mapStateMutationError(factErr)
		}
		facts = append(facts, fact)
	}
	lease, err := s.invalidator.Acquire(ctx, authstate.MutationFacts{Sessions: facts})
	if err != nil {
		return mapStateMutationError(err)
	}
	mutationCtx, stopRenewal := lease.StartRenewal(ctx)
	revoked, updateErr := s.passwords.ChangePasswordAndRevokeSessions(mutationCtx, identity.UserID, passwordHash, s.now().UTC())
	renewalCause := context.Cause(mutationCtx)
	stopRenewal()
	if updateErr != nil || renewalCause != nil {
		return apperror.DependencyUnavailable(errors.Join(updateErr, renewalCause, lease.Rollback(ctx)))
	}
	nextFacts := make([]authstate.SessionsFact, 0, len(facts))
	for _, fact := range facts {
		generation, generationErr := authstate.NewGeneration()
		if generationErr != nil {
			return apperror.Internal(generationErr)
		}
		nextFacts = append(nextFacts, authstate.SessionsFact{Platform: fact.Platform, UserID: fact.UserID, Generation: generation})
	}
	if err := lease.Commit(ctx, authstate.MutationFacts{Sessions: nextFacts}); err != nil {
		return apperror.DependencyUnavailable(err)
	}
	for _, session := range revoked {
		if err := s.sessionCache.Delete(ctx, session.Platform, session.ID); err != nil {
			return apperror.DependencyUnavailable(err)
		}
	}
	return nil
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

const (
	verificationCodeTTL            = 10 * time.Minute
	verificationCodeCleanupTimeout = time.Second
)

// LoginConfig returns the effective, channel-filtered login methods for the
// platform. Phone is filtered until a real SMS sender is wired.
func (s *Service) LoginConfig(ctx context.Context, client authclient.Client) (authplatform.LoginConfig, error) {
	policy, err := s.policies.CurrentPolicy(ctx, client.Platform)
	if err != nil {
		return authplatform.LoginConfig{}, err
	}
	options := make([]authplatform.LoginTypeOption, 0, len(policy.LoginTypes))
	for _, loginType := range policy.LoginTypes {
		switch loginType {
		case authplatform.LoginTypeEmail:
			if s.verifyCodeSender == nil {
				continue
			}
			ready, readyErr := s.verifyCodeSender.VerifyCodeReady(ctx, policy.ID, messagemail.SceneLogin)
			if readyErr != nil {
				return authplatform.LoginConfig{}, apperror.DependencyUnavailable(readyErr)
			}
			if ready {
				options = append(options, authplatform.LoginTypeOption{Value: loginType})
			}
		case authplatform.LoginTypePhone:
			// No SMS channel is wired yet; phone is never offered.
		case authplatform.LoginTypePassword:
			options = append(options, authplatform.LoginTypeOption{Value: loginType})
		}
	}
	if len(options) == 0 {
		return authplatform.LoginConfig{}, apperror.DependencyUnavailable(fmt.Errorf("no login methods are available for authentication platform %q", policy.Code))
	}
	return authplatform.LoginConfig{LoginTypes: options, AllowRegister: policy.AllowRegister}, nil
}

// SendCode issues a six-digit email verification code. Only the digest is
// stored; the code is returned to the caller only via the email channel.
func (s *Service) SendCode(ctx context.Context, input SendCodeInput) (SendCodeResult, error) {
	if input.Scene != messagemail.SceneLogin {
		return SendCodeResult{}, apperror.InvalidRequest(fmt.Errorf("verification code scene is invalid"))
	}
	if input.LoginType != authplatform.LoginTypeEmail {
		return SendCodeResult{}, apperror.InvalidRequest(fmt.Errorf("login type is not available"))
	}
	if s.verifyCodeSender == nil || s.verificationCodes == nil {
		return SendCodeResult{}, apperror.DependencyUnavailable(fmt.Errorf("verification code dependencies are unavailable"))
	}
	email, err := normalizeEmail(input.Account)
	if err != nil {
		return SendCodeResult{}, apperror.InvalidRequest(err)
	}
	policy, err := s.policies.CurrentPolicy(ctx, input.Client.Platform)
	if err != nil {
		return SendCodeResult{}, err
	}
	if !policyAllowsLoginType(policy, input.LoginType) {
		return SendCodeResult{}, apperror.Forbidden(fmt.Errorf("login type %q is disabled for authentication platform %q", input.LoginType, policy.Code))
	}
	ready, err := s.verifyCodeSender.VerifyCodeReady(ctx, policy.ID, messagemail.SceneLogin)
	if err != nil {
		return SendCodeResult{}, apperror.DependencyUnavailable(err)
	}
	if !ready {
		return SendCodeResult{}, apperror.DependencyUnavailable(fmt.Errorf("mail verification is unavailable"))
	}

	code, err := newSixDigitCode()
	if err != nil {
		return SendCodeResult{}, apperror.Internal(err)
	}
	key := s.verificationCodes.VerificationKey(policy.Code, messagemail.SceneLogin, string(authplatform.LoginTypeEmail), email)
	digest := s.verificationCodes.Digest(code)
	leaseToken, err := newRefreshToken()
	if err != nil {
		return SendCodeResult{}, apperror.Internal(err)
	}
	acquired, err := s.verificationCodes.AcquireDelivery(ctx, key, leaseToken, 10*time.Second)
	if err != nil {
		return SendCodeResult{}, apperror.DependencyUnavailable(err)
	}
	if !acquired {
		return SendCodeResult{}, apperror.DependencyUnavailable(fmt.Errorf("verification code delivery is in progress"))
	}
	challengeID := input.ChallengeID
	if challengeID == "" {
		challengeID = leaseToken
	}
	if err := s.verificationCodes.Put(ctx, key, digest, leaseToken, verificationCodeTTL); err != nil {
		cleanupCtx, cancelCleanup := verificationCodeCleanupContext(ctx)
		releaseErr := s.verificationCodes.ReleaseDelivery(cleanupCtx, key, leaseToken)
		cancelCleanup()
		return SendCodeResult{}, apperror.DependencyUnavailable(errors.Join(err, releaseErr))
	}
	expiresAt := s.now().UTC().Add(verificationCodeTTL)
	_, sendErr := s.verifyCodeSender.SendEmailVerifyCode(ctx, messagemail.EmailVerifyCodeInput{
		PlatformID: policy.ID, ClientIP: input.Client.ClientIP, ChallengeID: challengeID,
		Scene: messagemail.SceneLogin, ToEmail: email, Code: code, TTLMinutes: int(verificationCodeTTL / time.Minute),
	})
	if sendErr != nil {
		cleanupCtx, cancelCleanup := verificationCodeCleanupContext(ctx)
		cleanupErr := errors.Join(
			s.verificationCodes.DeleteIfOwned(cleanupCtx, key, leaseToken),
			s.verificationCodes.ReleaseDelivery(cleanupCtx, key, leaseToken),
		)
		cancelCleanup()
		if cleanupErr != nil {
			if s.logger != nil {
				s.logger.ErrorContext(ctx, "verification code cleanup failed", "error", cleanupErr)
			}
			return SendCodeResult{}, apperror.DependencyUnavailable(errors.Join(sendErr, cleanupErr))
		}
		return SendCodeResult{}, sendErr
	}
	cleanupCtx, cancelCleanup := verificationCodeCleanupContext(ctx)
	releaseErr := s.verificationCodes.ReleaseDelivery(cleanupCtx, key, leaseToken)
	cancelCleanup()
	if releaseErr != nil {
		if s.logger != nil {
			s.logger.ErrorContext(ctx, "verification code delivery lease release failed", "error", releaseErr)
		}
		return SendCodeResult{}, apperror.DependencyUnavailable(releaseErr)
	}
	return SendCodeResult{ChallengeID: challengeID, ExpiresAt: expiresAt}, nil
}

func verificationCodeCleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), verificationCodeCleanupTimeout)
}

func newSixDigitCode() (string, error) {
	value := make([]byte, 4)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate verification code: %w", err)
	}
	return fmt.Sprintf("%06d", binary.BigEndian.Uint32(value)%1_000_000), nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (Credential, error) {
	switch input.LoginType {
	case authplatform.LoginTypePassword, authplatform.LoginTypeEmail, authplatform.LoginTypePhone:
	default:
		return Credential{}, apperror.InvalidRequest(fmt.Errorf("login type is invalid"))
	}
	policy, err := s.policies.CurrentPolicy(ctx, input.Client.Platform)
	if err != nil {
		return Credential{}, err
	}
	if !policyAllowsLoginType(policy, input.LoginType) {
		return Credential{}, apperror.Forbidden(fmt.Errorf("login type %q is disabled for authentication platform %q", input.LoginType, policy.Code))
	}
	if input.LoginType == authplatform.LoginTypePhone {
		return Credential{}, apperror.Forbidden(fmt.Errorf("phone login is unavailable until an SMS channel is configured"))
	}
	switch input.LoginType {
	case authplatform.LoginTypePassword:
		return s.loginWithPassword(ctx, policy, input)
	case authplatform.LoginTypeEmail:
		return s.loginWithCode(ctx, policy, input)
	}
	return Credential{}, apperror.InvalidRequest(fmt.Errorf("login type is invalid"))
}

func policyAllowsLoginType(policy authplatform.Policy, loginType authplatform.LoginType) bool {
	for _, configured := range policy.LoginTypes {
		if configured == loginType {
			return true
		}
	}
	return false
}

func (s *Service) loginWithPassword(ctx context.Context, policy authplatform.Policy, input LoginInput) (Credential, error) {
	email, err := normalizeEmail(input.LoginAccount)
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
			if logErr := s.recordLoginEvent(ctx, loginlog.Event{PlatformID: policy.ID, LoginAccount: email, EventType: loginlog.EventLogin, LoginType: stringPointer(loginlog.LoginPassword), IsSuccess: yesno.No, ReasonCode: "invalid_credentials", ClientIP: input.Client.ClientIP, UserAgent: input.Client.UserAgent}); logErr != nil {
				return Credential{}, logErr
			}
			return Credential{}, invalidCredentialError(err)
		}
		return Credential{}, apperror.DependencyUnavailable(err)
	}
	if credential.IsEnabled != yesno.Yes {
		if logErr := s.recordLoginEvent(ctx, loginlog.Event{UserID: &credential.ID, PlatformID: policy.ID, LoginAccount: email, EventType: loginlog.EventLogin, LoginType: stringPointer(loginlog.LoginPassword), IsSuccess: yesno.No, ReasonCode: "account_disabled", ClientIP: input.Client.ClientIP, UserAgent: input.Client.UserAgent}); logErr != nil {
			return Credential{}, logErr
		}
		return Credential{}, apperror.Forbidden(fmt.Errorf("user is disabled"))
	}
	if credential.PasswordHash == "" {
		_ = s.recordLoginEvent(ctx, loginlog.Event{UserID: &credential.ID, PlatformID: policy.ID, LoginAccount: email, EventType: loginlog.EventLogin, LoginType: stringPointer(loginlog.LoginPassword), IsSuccess: yesno.No, ReasonCode: "invalid_credentials", ClientIP: input.Client.ClientIP, UserAgent: input.Client.UserAgent})
		return Credential{}, invalidCredentialError(fmt.Errorf("password credential is unavailable"))
	}
	if err := s.comparePassword(credential.PasswordHash, input.Password); err != nil {
		if logErr := s.recordLoginEvent(ctx, loginlog.Event{UserID: &credential.ID, PlatformID: policy.ID, LoginAccount: email, EventType: loginlog.EventLogin, LoginType: stringPointer(loginlog.LoginPassword), IsSuccess: yesno.No, ReasonCode: "invalid_credentials", ClientIP: input.Client.ClientIP, UserAgent: input.Client.UserAgent}); logErr != nil {
			return Credential{}, logErr
		}
		return Credential{}, invalidCredentialError(err)
	}

	return s.issueCredential(ctx, input.Client, policy, credential, email, loginlog.LoginPassword, false)
}

func (s *Service) loginWithCode(ctx context.Context, policy authplatform.Policy, input LoginInput) (Credential, error) {
	if s.verificationCodes == nil {
		return Credential{}, apperror.DependencyUnavailable(fmt.Errorf("verification code store is unavailable"))
	}
	account, err := normalizeLoginAccount(input.LoginType, input.LoginAccount)
	if err != nil {
		return Credential{}, apperror.InvalidRequest(err)
	}
	key := s.verificationCodes.VerificationKey(policy.Code, messagemail.SceneLogin, string(input.LoginType), account)
	digest := s.verificationCodes.Digest(input.Code)
	identityKind := loginIdentityKind(input.LoginType)
	valid, limited, checkErr := s.verificationCodes.CheckAttempt(ctx, key, digest, input.Client.ClientIP)
	if checkErr != nil {
		return Credential{}, apperror.DependencyUnavailable(checkErr)
	}
	if limited {
		return Credential{}, apperror.RateLimited(fmt.Errorf("verification code attempts exceeded"))
	}
	if !valid {
		s.recordCodeLoginFailure(ctx, input, policy, account, nil, "invalid_credentials")
		return Credential{}, invalidCredentialError(fmt.Errorf("verification code is invalid or expired"))
	}

	credential, findErr := s.users.FindCredentialByIdentity(ctx, identityKind, account)
	if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return Credential{}, apperror.DependencyUnavailable(findErr)
	}
	isNewUser := errors.Is(findErr, gorm.ErrRecordNotFound)
	if isNewUser && !policy.AllowRegister {
		s.recordCodeLoginFailure(ctx, input, policy, account, nil, "invalid_credentials")
		return Credential{}, invalidCredentialError(fmt.Errorf("verification code is invalid or expired"))
	}
	if !isNewUser && credential.IsEnabled != yesno.Yes {
		s.recordCodeLoginFailure(ctx, input, policy, account, &credential.ID, "account_disabled")
		return Credential{}, apperror.Forbidden(fmt.Errorf("user is disabled"))
	}

	consumed, consumeErr := s.verificationCodes.Consume(ctx, key, digest)
	if consumeErr != nil {
		return Credential{}, apperror.DependencyUnavailable(consumeErr)
	}
	if !consumed {
		s.recordCodeLoginFailure(ctx, input, policy, account, nil, "invalid_credentials")
		return Credential{}, invalidCredentialError(fmt.Errorf("verification code is invalid or expired"))
	}

	if isNewUser {
		created, createErr := s.users.CreateVerifiedIdentity(ctx, user.VerifiedIdentityInput{
			IdentityKind: identityKind, Account: account,
			Username: deterministicUsername(account), PasswordHash: "",
		})
		if createErr != nil {
			if errors.Is(createErr, user.ErrEmailConflict) || errors.Is(createErr, user.ErrPhoneConflict) || errors.Is(createErr, user.ErrUsernameConflict) {
				winner, requeryErr := s.users.FindCredentialByIdentity(ctx, identityKind, account)
				if requeryErr != nil {
					return Credential{}, apperror.DependencyUnavailable(requeryErr)
				}
				credential = winner
				isNewUser = false
			} else {
				return Credential{}, apperror.DependencyUnavailable(createErr)
			}
		} else {
			credential = user.Credential{ID: created.ID, Username: created.Username, Email: created.Email, PasswordHash: created.PasswordHash, IsEnabled: created.IsEnabled}
		}
	}
	if credential.IsEnabled != yesno.Yes {
		return Credential{}, apperror.Forbidden(fmt.Errorf("user is disabled"))
	}
	return s.issueCredential(ctx, input.Client, policy, credential, account, loginTypeFor(input.LoginType), isNewUser)
}

func (s *Service) recordCodeLoginFailure(ctx context.Context, input LoginInput, policy authplatform.Policy, account string, userID *int64, reason string) {
	_ = s.recordLoginEvent(ctx, loginlog.Event{
		UserID: userID, PlatformID: policy.ID, LoginAccount: account, EventType: loginlog.EventLogin,
		LoginType: stringPointer(loginTypeFor(input.LoginType)), IsSuccess: yesno.No, ReasonCode: reason,
		ClientIP: input.Client.ClientIP, UserAgent: input.Client.UserAgent,
	})
}

// issueCredential is the shared success tail: auth-state readiness, session
// creation/limit, snapshot publish, JWT issue and best-effort login log.
func (s *Service) issueCredential(ctx context.Context, client authclient.Client, policy authplatform.Policy, credential user.Credential, loginAccount, loginType string, isNewUser bool) (Credential, error) {
	userFact, err := s.ensureUserReady(ctx, credential.ID, true, false)
	if err != nil {
		return Credential{}, mapStateMutationError(err)
	}
	sessionsFact, err := s.ensureSessionsReady(ctx, client.Platform, credential.ID)
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
	created, revoked, createErr := s.sessions.CreateWithinLimit(mutationCtx, usersession.CreateInput{
		UserID: credential.ID, Platform: client.Platform, DeviceID: client.DeviceID,
		RefreshTokenHash: s.hashRefreshToken(refreshToken), ClientIP: client.ClientIP,
		UserAgent: client.UserAgent, RefreshExpiresAt: refreshExpiresAt,
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
	nextSessionsFact := authstate.SessionsFact{Platform: client.Platform, UserID: credential.ID, Generation: nextGeneration}
	if err := lease.Commit(ctx, authstate.MutationFacts{Sessions: []authstate.SessionsFact{nextSessionsFact}}); err != nil {
		return Credential{}, apperror.DependencyUnavailable(err)
	}
	refs := make([]authstate.SessionReference, 0, len(revoked))
	for _, session := range revoked {
		refs = append(refs, authstate.SessionReference{Platform: session.Platform, SessionID: session.ID})
	}
	if err := s.sessionCache.DeleteMany(ctx, refs); err != nil {
		return Credential{}, apperror.DependencyUnavailable(err)
	}
	created.RefreshExpiresAt = refreshExpiresAt
	if created.Platform == "" {
		created.Platform = client.Platform
	}
	if created.DeviceID == "" {
		created.DeviceID = client.DeviceID
	}
	if created.ClientIP == "" {
		created.ClientIP = client.ClientIP
	}
	if err := s.recordLoginEvent(ctx, loginlog.Event{UserID: &credential.ID, SessionID: &created.ID, PlatformID: policy.ID, LoginAccount: loginAccount, EventType: loginlog.EventLogin, LoginType: stringPointer(loginType), IsSuccess: yesno.Yes, ReasonCode: "success", ClientIP: client.ClientIP, UserAgent: client.UserAgent}); err != nil {
		return Credential{}, err
	}
	authority := usersession.Authority{Session: created, UserID: credential.ID, UserIsEnabled: credential.IsEnabled}
	if err := s.publishAuthority(ctx, authority, policy, userFact.Generation, nextSessionsFact.Generation, now); err != nil {
		return Credential{}, err
	}
	accessToken, accessExpiresAt, err := s.jwt.Issue(TokenIdentity{
		UserID: credential.ID, SessionID: created.ID, Platform: client.Platform, Version: created.Version,
	}, policy.AccessTTL)
	if err != nil {
		return Credential{}, apperror.Internal(err)
	}
	return Credential{
		AccessToken: accessToken, ExpiresIn: int(accessExpiresAt.Sub(now).Seconds()), RefreshToken: refreshToken,
		RefreshExpiresAt: refreshExpiresAt, IsNewUser: isNewUser,
	}, nil
}

func normalizeLoginAccount(loginType authplatform.LoginType, account string) (string, error) {
	switch loginType {
	case authplatform.LoginTypeEmail:
		return normalizeEmail(account)
	case authplatform.LoginTypePhone:
		normalized := strings.TrimSpace(account)
		if normalized == "" {
			return "", fmt.Errorf("phone number is invalid")
		}
		return normalized, nil
	default:
		return "", fmt.Errorf("login type is invalid")
	}
}

func loginIdentityKind(loginType authplatform.LoginType) string {
	if loginType == authplatform.LoginTypePhone {
		return "phone"
	}
	return "email"
}

func loginTypeFor(loginType authplatform.LoginType) string {
	switch loginType {
	case authplatform.LoginTypeEmail:
		return loginlog.LoginEmail
	case authplatform.LoginTypePhone:
		return loginlog.LoginPhone
	default:
		return loginlog.LoginPassword
	}
}

func deterministicUsername(account string) string {
	digest := sha256.Sum256([]byte(account))
	return "user_" + hex.EncodeToString(digest[:6])
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

	authority, err := s.sessions.FindAuthoritative(ctx, token.UserID, token.SessionID, token.Platform, token.Version, now)
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
	rotatedAuthority := usersession.Authority{Session: rotated, UserID: authority.UserID, UserIsEnabled: authority.UserIsEnabled, UserDeleted: authority.UserDeleted}
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
	if err := s.revokeSession(ctx, identity.UserID, identity.SessionID, identity.Platform); err != nil {
		return err
	}
	if s.loginLogs != nil {
		if err := s.loginLogs.Record(ctx, loginlog.Event{
			UserID: &identity.UserID, SessionID: &identity.SessionID, PlatformID: identity.PlatformID,
			EventType: loginlog.EventLogout, IsSuccess: yesno.Yes, ReasonCode: "success",
			ClientIP: client.ClientIP, UserAgent: client.UserAgent,
		}); err != nil {
			return apperror.DependencyUnavailable(err)
		}
	}
	return nil
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
	authority := usersession.Authority{
		Session: usersession.Record{
			ID: snapshot.SessionID, UserID: snapshot.UserID, PlatformID: policy.ID, DeviceID: snapshot.DeviceID,
			Version: snapshot.SessionVersion, ClientIP: snapshot.ClientIP, RefreshExpiresAt: snapshot.RefreshExpiresAt,
			Platform: snapshot.Platform,
		},
		UserID: snapshot.UserID, UserIsEnabled: yesno.Yes,
	}
	if err := s.enforceClient(ctx, authority, policy, client); err != nil {
		return Identity{}, false, "", err
	}
	return identityFromAuthority(authority, policy, "hit"), true, "hit", nil
}

func (s *Service) enforceClient(ctx context.Context, authority usersession.Authority, policy authplatform.Policy, client authclient.Client) error {
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

func (s *Service) publishAuthority(ctx context.Context, authority usersession.Authority, policy authplatform.Policy, userGeneration, sessionsGeneration string, now time.Time) error {
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

func snapshotFromAuthority(authority usersession.Authority, policy authplatform.Policy, userGeneration, sessionsGeneration string) SessionSnapshot {
	return SessionSnapshot{
		SchemaVersion: sessionSnapshotSchemaVersion, UserID: authority.UserID, SessionID: authority.Session.ID,
		Platform: authority.Session.Platform, SessionVersion: authority.Session.Version, PolicyVersion: policy.PolicyVersion,
		UserGeneration: userGeneration, SessionsGeneration: sessionsGeneration, DeviceID: authority.Session.DeviceID,
		ClientIP: authority.Session.ClientIP, RefreshExpiresAt: authority.Session.RefreshExpiresAt.UTC(), Revoked: authority.Session.RevokedAt != nil,
	}
}

func identityFromAuthority(authority usersession.Authority, policy authplatform.Policy, cacheResult string) Identity {
	return Identity{
		UserID: authority.UserID, SessionID: authority.Session.ID, PlatformID: policy.ID, Platform: authority.Session.Platform,
		Version: authority.Session.Version, PolicyVersion: policy.PolicyVersion, AccessCacheTTL: policy.AccessCacheTTL,
		CacheResult: cacheResult,
	}
}

func validateAuthority(authority usersession.Authority) error {
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
