package auth

import (
	"context"
	"errors"
	"fmt"

	authclient "admin/server/internal/module/auth/client"
	"admin/server/internal/module/auth/platform"
	messagemail "admin/server/internal/module/message/mail"
	user "admin/server/internal/module/user/account"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

// ForgotPasswordInput is the request body of POST /auth/password/forgot.
type ForgotPasswordInput struct {
	Email  string
	Client authclient.Client
}

// ForgotPassword sends a password-reset verification code (scene=forget) to a
// registered email. It fails closed before acquiring any delivery lease when
// password login is disabled for the platform or the email is unknown, so
// unknown or disabled accounts never consume rate-limit budget.
func (s *Service) ForgotPassword(ctx context.Context, input ForgotPasswordInput) (SendCodeResult, error) {
	if s.verifyCodeSender == nil || s.verificationCodes == nil {
		return SendCodeResult{}, apperror.DependencyUnavailable(fmt.Errorf("verification code dependencies are unavailable"))
	}
	policy, err := s.policies.CurrentPolicy(ctx, input.Client.Platform)
	if err != nil {
		return SendCodeResult{}, err
	}
	if !policyAllowsLoginType(policy, authplatform.LoginTypePassword) {
		return SendCodeResult{}, apperror.Forbidden(fmt.Errorf("login type %q is disabled for authentication platform %q", authplatform.LoginTypePassword, policy.Code))
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return SendCodeResult{}, apperror.InvalidRequest(err)
	}
	credential, err := s.users.FindCredentialByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return SendCodeResult{}, apperror.NotFound(fmt.Errorf("email is not registered"))
		}
		return SendCodeResult{}, apperror.DependencyUnavailable(err)
	}
	if credential.IsEnabled != yesno.Yes {
		return SendCodeResult{}, apperror.NotFound(fmt.Errorf("email is not registered"))
	}
	return s.deliverEmailVerificationCode(ctx, policy, messagemail.SceneForget, email, input.Client, "")
}

// ResetPasswordInput is the request body of POST /auth/password/reset.
type ResetPasswordInput struct {
	Email           string
	Code            string
	NewPassword     string
	ConfirmPassword string
	Client          authclient.Client
}

// ResetPassword consumes a forget-scene verification code and replaces the
// account password, revoking every active session across all platforms. The
// code is consumed before the mutation runs, so a partially failed reset
// never leaves a replayable code behind.
func (s *Service) ResetPassword(ctx context.Context, input ResetPasswordInput) error {
	if s.verificationCodes == nil || s.passwords == nil || s.states == nil || s.invalidator == nil || s.sessionCache == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("password reset dependencies are unavailable"))
	}
	policy, err := s.policies.CurrentPolicy(ctx, input.Client.Platform)
	if err != nil {
		return err
	}
	if !policyAllowsLoginType(policy, authplatform.LoginTypePassword) {
		return apperror.Forbidden(fmt.Errorf("login type %q is disabled for authentication platform %q", authplatform.LoginTypePassword, policy.Code))
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return apperror.InvalidRequest(err)
	}
	if input.NewPassword != input.ConfirmPassword {
		return apperror.InvalidRequest(fmt.Errorf("password confirmation does not match"))
	}
	if err := ValidatePassword(input.NewPassword); err != nil {
		return apperror.InvalidRequest(err)
	}
	credential, err := s.users.FindCredentialByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperror.NotFound(fmt.Errorf("email is not registered"))
		}
		return apperror.DependencyUnavailable(err)
	}
	if credential.IsEnabled != yesno.Yes {
		return apperror.NotFound(fmt.Errorf("email is not registered"))
	}
	key := s.verificationCodes.VerificationKey(policy.Code, messagemail.SceneForget, string(authplatform.LoginTypeEmail), email)
	digest := s.verificationCodes.Digest(input.Code)
	valid, limited, checkErr := s.verificationCodes.CheckAttempt(ctx, key, digest, input.Client.ClientIP)
	if checkErr != nil {
		return apperror.DependencyUnavailable(checkErr)
	}
	if limited {
		return apperror.RateLimited(fmt.Errorf("verification code attempts exceeded"))
	}
	if !valid {
		return invalidCredentialError(fmt.Errorf("verification code is invalid or expired"))
	}
	consumed, consumeErr := s.verificationCodes.Consume(ctx, key, digest)
	if consumeErr != nil {
		return apperror.DependencyUnavailable(consumeErr)
	}
	if !consumed {
		return invalidCredentialError(fmt.Errorf("verification code is invalid or expired"))
	}
	passwordHash, err := HashPassword(input.NewPassword)
	if err != nil {
		return apperror.Internal(err)
	}
	return s.revokeAllSessionsAndAdvanceState(ctx, credential.ID, passwordHash)
}

// SetPasswordInput is the request body of POST /account/password/set.
type SetPasswordInput struct {
	NewPassword     string
	ConfirmPassword string
}

// SetPassword stores the first password of a passwordless account. Accounts
// that already have a password must use ChangePassword instead; sessions are
// kept alive because setting the first password is not a recovery event.
func (s *Service) SetPassword(ctx context.Context, identity Identity, input SetPasswordInput) error {
	if s.passwords == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("password dependencies are unavailable"))
	}
	if identity.UserID < 1 {
		return apperror.Unauthorized(fmt.Errorf("authentication identity is missing"))
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
	if credential.PasswordHash != "" {
		return apperror.Conflict(i18n.KeyConflict, nil, fmt.Errorf("password is already set; use change password instead"))
	}
	passwordHash, err := HashPassword(input.NewPassword)
	if err != nil {
		return apperror.Internal(err)
	}
	if err := s.passwords.SetPasswordHash(ctx, identity.UserID, passwordHash, s.now().UTC()); err != nil {
		if errors.Is(err, user.ErrPasswordAlreadySet) {
			return apperror.Conflict(i18n.KeyConflict, nil, err)
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperror.NotFound(fmt.Errorf("user is not registered"))
		}
		return apperror.DependencyUnavailable(err)
	}
	return nil
}
