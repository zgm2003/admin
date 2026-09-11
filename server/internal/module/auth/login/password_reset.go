package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	authclient "admin/server/internal/module/auth/client"
	messagemail "admin/server/internal/module/message/mail"
	smstemplate "admin/server/internal/module/message/sms/template"
	"admin/server/internal/module/permission/authPlatform"
	user "admin/server/internal/module/user/account"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

// ForgotPasswordInput is the request body of POST /auth/password/forgot.
type ForgotPasswordInput struct {
	Account   string
	LoginType authplatform.LoginType
	Client    authclient.Client
}

// ForgotPassword sends a password-reset verification code (scene=forget) to a
// registered email or phone. The sender, normalization, credential lookup and
// verification key are isolated by login type. It fails closed before acquiring
// any delivery lease when password login is disabled or the account is unknown.
func (s *Service) ForgotPassword(ctx context.Context, input ForgotPasswordInput) (SendCodeResult, error) {
	if s.verificationCodes == nil {
		return SendCodeResult{}, apperror.DependencyUnavailable(fmt.Errorf("verification code dependencies are unavailable"))
	}
	policy, err := s.policies.CurrentPolicy(ctx, input.Client.Platform)
	if err != nil {
		return SendCodeResult{}, err
	}
	if !policyAllowsLoginType(policy, authplatform.LoginTypePassword) {
		return SendCodeResult{}, apperror.Forbidden(fmt.Errorf("login type %q is disabled for authentication platform %q", authplatform.LoginTypePassword, policy.Code))
	}
	if input.LoginType != authplatform.LoginTypeEmail && input.LoginType != authplatform.LoginTypePhone {
		return SendCodeResult{}, apperror.InvalidRequest(fmt.Errorf("login type is not available for password reset"))
	}
	if !policyAllowsLoginType(policy, input.LoginType) {
		return SendCodeResult{}, apperror.Forbidden(fmt.Errorf("login type %q is disabled for authentication platform %q", input.LoginType, policy.Code))
	}
	account, err := normalizeLoginAccount(input.LoginType, input.Account)
	if err != nil {
		return SendCodeResult{}, apperror.InvalidRequest(err)
	}
	credential, err := s.users.FindCredentialByIdentity(ctx, string(input.LoginType), account)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return SendCodeResult{}, apperror.NotFound(fmt.Errorf("account is not registered"))
		}
		return SendCodeResult{}, apperror.DependencyUnavailable(err)
	}
	if credential.IsEnabled != yesno.Yes {
		return SendCodeResult{}, apperror.NotFound(fmt.Errorf("account is not registered"))
	}
	if input.LoginType == authplatform.LoginTypePhone {
		if s.phoneCodeSender == nil {
			return SendCodeResult{}, apperror.DependencyUnavailable(fmt.Errorf("verification code dependencies are unavailable"))
		}
		return s.deliverPhoneVerificationCode(ctx, policy, smstemplate.SceneForget, account, input.Client, "", nil)
	}
	if s.verifyCodeSender == nil {
		return SendCodeResult{}, apperror.DependencyUnavailable(fmt.Errorf("verification code dependencies are unavailable"))
	}
	return s.deliverEmailVerificationCode(ctx, policy, messagemail.SceneForget, account, input.Client, "")
}

// ResetPasswordInput is the request body of POST /auth/password/reset.
type ResetPasswordInput struct {
	Account         string
	LoginType       authplatform.LoginType
	ChallengeID     string
	Code            string
	NewPassword     string
	ConfirmPassword string
	Client          authclient.Client
}

// ChangePasswordByCodeInput carries the SMS proof for changing a password
// while keeping the current authenticated session alive.
type ChangePasswordByCodeInput struct {
	LoginType       authplatform.LoginType
	ChallengeID     string
	Code            string
	NewPassword     string
	ConfirmPassword string
}

func (s *Service) SendPasswordCodeForLoginType(ctx context.Context, identity Identity, client authclient.Client, loginType authplatform.LoginType) (SendCodeResult, error) {
	if identity.UserID < 1 || identity.SessionID < 1 || identity.Platform == "" || identity.PlatformID < 1 {
		return SendCodeResult{}, apperror.Unauthorized(fmt.Errorf("authentication identity is missing"))
	}
	if s.users == nil {
		return SendCodeResult{}, apperror.DependencyUnavailable(fmt.Errorf("password verification dependencies are unavailable"))
	}
	policy, err := s.policies.CurrentPolicy(ctx, identity.Platform)
	if err != nil {
		return SendCodeResult{}, err
	}
	if loginType != authplatform.LoginTypeEmail && loginType != authplatform.LoginTypePhone {
		return SendCodeResult{}, apperror.InvalidRequest(fmt.Errorf("login type is not available for password change"))
	}
	if !policyAllowsLoginType(policy, loginType) {
		return SendCodeResult{}, apperror.Forbidden(fmt.Errorf("login type is disabled for authentication platform %q", policy.Code))
	}
	current, err := s.users.FindCurrent(ctx, identity.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return SendCodeResult{}, apperror.NotFound(fmt.Errorf("user is not registered"))
		}
		return SendCodeResult{}, apperror.DependencyUnavailable(err)
	}
	var account string
	if loginType == authplatform.LoginTypePhone {
		if current.Phone == nil || *current.Phone == "" {
			return SendCodeResult{}, apperror.NotFound(fmt.Errorf("phone is not bound"))
		}
		account = *current.Phone
	} else {
		if current.Email == "" {
			return SendCodeResult{}, apperror.NotFound(fmt.Errorf("email is not bound"))
		}
		account = current.Email
	}
	if s.verificationCodes == nil || (loginType == authplatform.LoginTypePhone && s.phoneCodeSender == nil) || (loginType == authplatform.LoginTypeEmail && s.verifyCodeSender == nil) {
		return SendCodeResult{}, apperror.DependencyUnavailable(fmt.Errorf("password verification dependencies are unavailable"))
	}
	client.Platform = identity.Platform
	if loginType == authplatform.LoginTypePhone {
		return s.deliverPhoneVerificationCode(ctx, policy, smstemplate.SceneChangePassword, account, client, "", &identity.UserID)
	}
	return s.deliverEmailVerificationCode(ctx, policy, messagemail.SceneChangePassword, account, client, "")
}

// ChangePasswordByCode consumes one SMS proof and changes the password while
// preserving the current session and revoking all other sessions.
func (s *Service) ChangePasswordByCode(ctx context.Context, identity Identity, client authclient.Client, input ChangePasswordByCodeInput) error {
	if identity.UserID < 1 || identity.SessionID < 1 || identity.Platform == "" || identity.PlatformID < 1 {
		return apperror.Unauthorized(fmt.Errorf("authentication identity is missing"))
	}
	if s.users == nil || s.passwords == nil || s.verificationCodes == nil || s.states == nil || s.invalidator == nil || s.sessionCache == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("password verification dependencies are unavailable"))
	}
	if validateChallengeID(input.ChallengeID) != nil || !isSixASCIIDigits(input.Code) {
		return apperror.InvalidRequest(fmt.Errorf("password reset proof is invalid"))
	}
	if input.NewPassword != input.ConfirmPassword {
		return apperror.InvalidRequest(fmt.Errorf("password confirmation does not match"))
	}
	if err := ValidatePassword(input.NewPassword); err != nil {
		return apperror.InvalidRequest(err)
	}
	policy, err := s.policies.CurrentPolicy(ctx, identity.Platform)
	if err != nil {
		return err
	}
	loginType := input.LoginType
	if loginType == "" {
		return apperror.InvalidRequest(fmt.Errorf("login type is required for password change"))
	}
	if loginType != authplatform.LoginTypeEmail && loginType != authplatform.LoginTypePhone {
		return apperror.InvalidRequest(fmt.Errorf("login type is not available for password change"))
	}
	if !policyAllowsLoginType(policy, loginType) {
		return apperror.Forbidden(fmt.Errorf("login type is disabled for authentication platform %q", policy.Code))
	}
	current, err := s.users.FindCurrent(ctx, identity.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperror.NotFound(fmt.Errorf("user is not registered"))
		}
		return apperror.DependencyUnavailable(err)
	}
	var account string
	scene := messagemail.SceneChangePassword
	if loginType == authplatform.LoginTypePhone {
		if current.Phone == nil || *current.Phone == "" {
			return apperror.NotFound(fmt.Errorf("phone is not bound"))
		}
		account, scene = *current.Phone, smstemplate.SceneChangePassword
	} else {
		if current.Email == "" {
			return apperror.NotFound(fmt.Errorf("email is not bound"))
		}
		account = current.Email
	}
	key := s.verificationCodes.VerificationKey(identity.Platform, scene, string(loginType), account)
	digest := s.verificationCodes.ProofDigest(input.ChallengeID, input.Code)
	valid, limited, err := s.verificationCodes.CheckAttempt(ctx, key, digest, client.ClientIP)
	if err != nil {
		return apperror.DependencyUnavailable(err)
	}
	if limited {
		return apperror.RateLimited(fmt.Errorf("verification code attempts exceeded"))
	}
	if !valid {
		return invalidCredentialError(fmt.Errorf("verification code is invalid or expired"))
	}
	consumed, err := s.verificationCodes.Consume(ctx, key, digest)
	if err != nil {
		return apperror.DependencyUnavailable(err)
	}
	if !consumed {
		return invalidCredentialError(fmt.Errorf("verification code is invalid or expired"))
	}
	hash, err := HashPassword(input.NewPassword)
	if err != nil {
		return apperror.Internal(err)
	}
	return s.revokeOtherSessionsAndAdvanceState(ctx, identity.UserID, identity.SessionID, hash)
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
	if input.LoginType != authplatform.LoginTypeEmail && input.LoginType != authplatform.LoginTypePhone {
		return apperror.InvalidRequest(fmt.Errorf("login type is not available for password reset"))
	}
	if !policyAllowsLoginType(policy, input.LoginType) {
		return apperror.Forbidden(fmt.Errorf("login type %q is disabled for authentication platform %q", input.LoginType, policy.Code))
	}
	if err := validateChallengeID(input.ChallengeID); err != nil {
		return apperror.InvalidRequest(err)
	}
	if !isSixASCIIDigits(input.Code) {
		return apperror.InvalidRequest(fmt.Errorf("verification code must contain six ASCII digits"))
	}
	account, err := normalizeLoginAccount(input.LoginType, input.Account)
	if err != nil {
		return apperror.InvalidRequest(err)
	}
	if input.NewPassword != input.ConfirmPassword {
		return apperror.InvalidRequest(fmt.Errorf("password confirmation does not match"))
	}
	if err := ValidatePassword(input.NewPassword); err != nil {
		return apperror.InvalidRequest(err)
	}
	credential, err := s.users.FindCredentialByIdentity(ctx, string(input.LoginType), account)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperror.NotFound(fmt.Errorf("account is not registered"))
		}
		return apperror.DependencyUnavailable(err)
	}
	if credential.IsEnabled != yesno.Yes {
		return apperror.NotFound(fmt.Errorf("account is not registered"))
	}
	scene := messagemail.SceneForget
	if input.LoginType == authplatform.LoginTypePhone {
		scene = smstemplate.SceneForget
	}
	key := s.verificationCodes.VerificationKey(policy.Code, scene, string(input.LoginType), account)
	digest := s.verificationCodes.ProofDigest(input.ChallengeID, input.Code)
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

func validateChallengeID(value string) error {
	if value == "" || len(value) > 128 || strings.TrimSpace(value) != value || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("challengeId is invalid")
	}
	return nil
}

func isSixASCIIDigits(value string) bool {
	if len(value) != 6 {
		return false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
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
