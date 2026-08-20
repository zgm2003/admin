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
	"net/mail"
	"strconv"
	"strings"
	"time"

	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

const RefreshTTL = 14 * 24 * time.Hour

type userStore interface {
	CreateWithRole(context.Context, user.CreateInput) (user.User, error)
	FindCredentialByUsername(context.Context, string) (user.Credential, error)
	FindCurrent(context.Context, int64) (user.Current, error)
}

type roleStore interface {
	FindDefault(context.Context) (role.Role, error)
}

type sessionStore interface {
	CreateReplacingActive(context.Context, SessionCreate, time.Time) (Session, error)
	FindActiveIdentity(context.Context, int64, int64, time.Time) (Identity, error)
	FindCurrentByUser(context.Context, int64, time.Time) (Session, error)
	FindByRefreshHash(context.Context, string, time.Time) (Session, error)
	RotateByRefreshHash(context.Context, int64, string, string, time.Time, string, string) (Session, bool, error)
	Revoke(context.Context, int64, time.Time) error
}

type pointerStore interface {
	GetString(context.Context, string) (string, bool, error)
	SetString(context.Context, string, string, time.Duration) error
	Delete(context.Context, string) error
}

type Service struct {
	users               userStore
	roles               roleStore
	sessions            sessionStore
	pointers            pointerStore
	jwt                 *JWT
	refreshTokenHMACKey []byte
	now                 func() time.Time
}

func NewService(
	users userStore,
	roles roleStore,
	sessions sessionStore,
	pointers pointerStore,
	jwt *JWT,
	refreshTokenHMACKey []byte,
) *Service {
	return &Service{
		users:               users,
		roles:               roles,
		sessions:            sessions,
		pointers:            pointers,
		jwt:                 jwt,
		refreshTokenHMACKey: append([]byte(nil), refreshTokenHMACKey...),
		now:                 time.Now,
	}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (Registered, error) {
	normalized, err := validateAccountInput(input.Username, input.Email, input.Password, input.ConfirmPassword)
	if err != nil {
		return Registered{}, err
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
		Username:     normalized.Username,
		Email:        normalized.Email,
		PasswordHash: passwordHash,
		RoleID:       defaultRole.ID,
	})
	if err != nil {
		return Registered{}, mapUserCreateError(err)
	}
	return Registered{UserID: created.ID, Username: created.Username, Email: created.Email}, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (Credential, error) {
	username := strings.TrimSpace(input.Username)
	if username == "" || input.Password == "" {
		return Credential{}, apperror.InvalidRequest(fmt.Errorf("username and password are required"))
	}
	credential, err := s.users.FindCredentialByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Credential{}, invalidCredentialError(err)
		}
		return Credential{}, apperror.DependencyUnavailable(err)
	}
	if credential.IsEnabled != yesno.Yes {
		return Credential{}, apperror.Forbidden(fmt.Errorf("user is disabled"))
	}
	if err := VerifyPassword(credential.PasswordHash, input.Password); err != nil {
		return Credential{}, invalidCredentialError(err)
	}

	now := s.now().UTC()
	refreshToken, err := newRefreshToken()
	if err != nil {
		return Credential{}, apperror.Internal(err)
	}
	refreshExpiresAt := now.Add(RefreshTTL)
	session, err := s.sessions.CreateReplacingActive(ctx, SessionCreate{
		UserID:           credential.ID,
		RefreshTokenHash: s.hashRefreshToken(refreshToken),
		ClientIP:         input.ClientIP,
		UserAgent:        input.UserAgent,
		RefreshExpiresAt: refreshExpiresAt,
	}, now)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Credential{}, apperror.Forbidden(err)
		}
		return Credential{}, apperror.DependencyUnavailable(err)
	}
	pointerKey := user.CurrentSessionPointerKey(credential.ID)
	if pointerErr := s.pointers.SetString(ctx, pointerKey, strconv.FormatInt(session.ID, 10), refreshExpiresAt.Sub(now)); pointerErr != nil {
		revokeErr := s.sessions.Revoke(ctx, session.ID, now)
		return Credential{}, apperror.DependencyUnavailable(errors.Join(pointerErr, revokeErr))
	}
	accessToken, accessExpiresAt, err := s.jwt.Issue(Identity{UserID: credential.ID, SessionID: session.ID, Version: session.Version})
	if err != nil {
		return Credential{}, apperror.Internal(err)
	}
	return Credential{
		AccessToken:      accessToken,
		ExpiresIn:        int(accessExpiresAt.Sub(now).Seconds()),
		RefreshToken:     refreshToken,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

func (s *Service) Authenticate(ctx context.Context, accessToken string) (Identity, error) {
	identity, err := s.jwt.Parse(accessToken)
	if err != nil {
		return Identity{}, apperror.Unauthorized(err)
	}
	now := s.now().UTC()
	currentSessionID, err := s.currentSessionID(ctx, identity.UserID, now)
	if err != nil {
		return Identity{}, err
	}
	if currentSessionID != identity.SessionID {
		return Identity{}, apperror.Unauthorized(fmt.Errorf("session was replaced"))
	}
	activeIdentity, err := s.sessions.FindActiveIdentity(ctx, identity.SessionID, identity.Version, now)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Identity{}, apperror.Unauthorized(err)
		}
		return Identity{}, apperror.DependencyUnavailable(err)
	}
	if activeIdentity != identity {
		return Identity{}, apperror.Unauthorized(fmt.Errorf("session identity does not match token"))
	}
	return activeIdentity, nil
}

func (s *Service) Refresh(ctx context.Context, input RefreshInput) (Credential, error) {
	if input.RefreshToken == "" {
		return Credential{}, apperror.Unauthorized(fmt.Errorf("Refresh Token is required"))
	}
	now := s.now().UTC()
	oldHash := s.hashRefreshToken(input.RefreshToken)
	current, err := s.sessions.FindByRefreshHash(ctx, oldHash, now)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Credential{}, apperror.Unauthorized(err)
		}
		return Credential{}, apperror.DependencyUnavailable(err)
	}
	currentSessionID, err := s.currentSessionID(ctx, current.UserID, now)
	if err != nil {
		return Credential{}, err
	}
	if currentSessionID != current.ID {
		return Credential{}, apperror.Unauthorized(fmt.Errorf("Refresh Token does not belong to the current session"))
	}

	newToken, err := newRefreshToken()
	if err != nil {
		return Credential{}, apperror.Internal(err)
	}
	rotated, won, err := s.sessions.RotateByRefreshHash(
		ctx, current.ID, oldHash, s.hashRefreshToken(newToken), now, input.ClientIP, input.UserAgent,
	)
	if err != nil {
		return Credential{}, apperror.DependencyUnavailable(err)
	}
	if !won {
		return Credential{}, apperror.Unauthorized(fmt.Errorf("Refresh Token was already used"))
	}
	accessToken, accessExpiresAt, err := s.jwt.Issue(Identity{UserID: rotated.UserID, SessionID: rotated.ID, Version: rotated.Version})
	if err != nil {
		return Credential{}, apperror.Internal(err)
	}
	return Credential{
		AccessToken:      accessToken,
		ExpiresIn:        int(accessExpiresAt.Sub(now).Seconds()),
		RefreshToken:     newToken,
		RefreshExpiresAt: rotated.RefreshExpiresAt,
	}, nil
}

func (s *Service) Logout(ctx context.Context, identity Identity) error {
	if err := validateIdentity(identity); err != nil {
		return apperror.Unauthorized(err)
	}
	now := s.now().UTC()
	if err := s.sessions.Revoke(ctx, identity.SessionID, now); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperror.Unauthorized(err)
		}
		return apperror.DependencyUnavailable(err)
	}
	if err := s.pointers.Delete(ctx, user.CurrentSessionPointerKey(identity.UserID)); err != nil {
		return apperror.DependencyUnavailable(err)
	}
	return nil
}

func (s *Service) CurrentUser(ctx context.Context, identity Identity) (user.Current, error) {
	if err := validateIdentity(identity); err != nil {
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

func (s *Service) currentSessionID(ctx context.Context, userID int64, now time.Time) (int64, error) {
	key := user.CurrentSessionPointerKey(userID)
	value, found, err := s.pointers.GetString(ctx, key)
	if err != nil {
		return 0, apperror.DependencyUnavailable(err)
	}
	if found {
		sessionID, parseErr := strconv.ParseInt(value, 10, 64)
		if parseErr != nil || sessionID <= 0 || strconv.FormatInt(sessionID, 10) != value {
			return 0, apperror.Unauthorized(fmt.Errorf("current session pointer is invalid"))
		}
		return sessionID, nil
	}

	current, err := s.sessions.FindCurrentByUser(ctx, userID, now)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, apperror.Unauthorized(err)
		}
		return 0, apperror.DependencyUnavailable(err)
	}
	ttl := current.RefreshExpiresAt.Sub(now)
	if ttl <= 0 {
		return 0, apperror.Unauthorized(fmt.Errorf("current session is expired"))
	}
	if err := s.pointers.SetString(ctx, key, strconv.FormatInt(current.ID, 10), ttl); err != nil {
		return 0, apperror.DependencyUnavailable(err)
	}
	return current.ID, nil
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

	email = strings.ToLower(strings.TrimSpace(email))
	parsedAddress, err := mail.ParseAddress(email)
	if err != nil || parsedAddress.Name != "" || parsedAddress.Address != email || len(email) > 254 {
		return normalizedAccount{}, apperror.InvalidRequest(fmt.Errorf("email address is invalid"))
	}
	if password != confirmPassword {
		return normalizedAccount{}, apperror.InvalidRequest(fmt.Errorf("password confirmation does not match"))
	}
	if err := ValidatePassword(password); err != nil {
		return normalizedAccount{}, apperror.InvalidRequest(err)
	}
	return normalizedAccount{Username: username, Email: email, Password: password}, nil
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
