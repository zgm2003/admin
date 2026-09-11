package email

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
	"time"

	authstate "admin/server/internal/module/auth/state"
	messagemail "admin/server/internal/module/message/mail"
	mailtemplate "admin/server/internal/module/message/mail/template"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	sharedemail "admin/server/internal/shared/email"
	"admin/server/internal/shared/i18n"
	"gorm.io/gorm"
)

type Service struct {
	accounts     accountStore
	sender       emailCodeSender
	verification VerificationCodeStore
	keys         *secretkey.KeyRing
	authority    authorityCoordinator
	now          func() time.Time
	generateCode func() (string, error)
	generateID   func() (string, error)
}

func NewService(accounts accountStore, sender emailCodeSender, verification VerificationCodeStore, keys *secretkey.KeyRing, authority authorityCoordinator) *Service {
	return &Service{accounts: accounts, sender: sender, verification: verification, keys: keys, authority: authority, now: time.Now, generateCode: newCode, generateID: newID}
}

func (s *Service) SendCode(ctx context.Context, actor Actor, input SendCodeInput) (SendCodeResult, error) {
	if err := validateActor(actor); err != nil {
		return SendCodeResult{}, err
	}
	if input.Target != TargetCurrent && input.Target != TargetNext {
		return SendCodeResult{}, apperror.InvalidRequest(fmt.Errorf("email target is invalid"))
	}
	if input.Target == TargetCurrent && input.Email != nil {
		return SendCodeResult{}, apperror.InvalidRequest(fmt.Errorf("current email must not be supplied"))
	}
	if input.Target == TargetNext && input.Email == nil {
		return SendCodeResult{}, apperror.InvalidRequest(fmt.Errorf("next email is required"))
	}
	if input.ChallengeID != "" && !validChallenge(input.ChallengeID) {
		return SendCodeResult{}, apperror.InvalidRequest(fmt.Errorf("challengeId is invalid"))
	}
	if s.accounts == nil || s.sender == nil || s.verification == nil {
		return SendCodeResult{}, apperror.DependencyUnavailable(fmt.Errorf("email verification dependencies are unavailable"))
	}
	current, err := s.accounts.Current(ctx, actor.UserID)
	if err != nil {
		return SendCodeResult{}, mapRepositoryError(err)
	}
	if current.Deleted {
		return SendCodeResult{}, apperror.NotFound(fmt.Errorf("email account is deleted"))
	}
	if !current.IsEnabled {
		return SendCodeResult{}, apperror.Forbidden(fmt.Errorf("email account is disabled"))
	}
	var destination string
	if input.Target == TargetCurrent {
		if current.Email == nil {
			return SendCodeResult{}, apperror.NotFound(fmt.Errorf("email is not bound"))
		}
		destination = *current.Email
	} else {
		destination, err = normalizeEmail(*input.Email)
		if err != nil {
			return SendCodeResult{}, apperror.InvalidRequest(err)
		}
		if current.Email != nil && destination == *current.Email {
			return SendCodeResult{}, apperror.Conflict(i18n.KeyConflict, nil, fmt.Errorf("next email matches current email"))
		}
		inUse, checkErr := s.accounts.EmailInUse(ctx, actor.UserID, destination)
		if checkErr != nil {
			return SendCodeResult{}, apperror.DependencyUnavailable(checkErr)
		}
		if inUse {
			return SendCodeResult{}, emailConflict(ErrEmailConflict)
		}
	}
	return s.deliver(ctx, actor, destination, input.ChallengeID)
}

func (s *Service) BindOrChange(ctx context.Context, actor Actor, input BindOrChangeInput) (EmailResult, error) {
	if err := validateActor(actor); err != nil {
		return EmailResult{}, err
	}
	if s.accounts == nil || s.verification == nil || s.keys == nil || s.authority == nil {
		return EmailResult{}, apperror.DependencyUnavailable(fmt.Errorf("email mutation dependencies are unavailable"))
	}
	if !validProof(input.NextChallengeID, input.NextCode) {
		return EmailResult{}, apperror.InvalidRequest(fmt.Errorf("next email proof is invalid"))
	}
	nextEmail, err := normalizeEmail(input.NextEmail)
	if err != nil {
		return EmailResult{}, apperror.InvalidRequest(err)
	}
	current, err := s.accounts.Current(ctx, actor.UserID)
	if err != nil {
		return EmailResult{}, mapRepositoryError(err)
	}
	if current.Deleted {
		return EmailResult{}, apperror.NotFound(fmt.Errorf("email account is deleted"))
	}
	if !current.IsEnabled {
		return EmailResult{}, apperror.Forbidden(fmt.Errorf("email account is disabled"))
	}
	oldEmail, action := "", ActionBind
	keys, digests := make([]string, 0, 2), make([]string, 0, 2)
	if current.Email == nil {
		if input.CurrentChallengeID != "" || input.CurrentCode != "" {
			return EmailResult{}, apperror.InvalidRequest(fmt.Errorf("current email proof is not allowed for first bind"))
		}
	} else {
		oldEmail, action = *current.Email, ActionChange
		if oldEmail == nextEmail {
			return EmailResult{}, apperror.Conflict(i18n.KeyConflict, nil, fmt.Errorf("next email matches current email"))
		}
		if !validProof(input.CurrentChallengeID, input.CurrentCode) {
			return EmailResult{}, apperror.InvalidRequest(fmt.Errorf("current email proof is required"))
		}
		keys = append(keys, s.verification.VerificationKey(actor.Platform, mailtemplate.SceneBindEmail, "email", oldEmail))
		digests = append(digests, s.verification.ProofDigest(input.CurrentChallengeID, input.CurrentCode))
	}
	inUse, err := s.accounts.EmailInUse(ctx, actor.UserID, nextEmail)
	if err != nil {
		return EmailResult{}, apperror.DependencyUnavailable(err)
	}
	if inUse {
		return EmailResult{}, emailConflict(ErrEmailConflict)
	}
	keys = append(keys, s.verification.VerificationKey(actor.Platform, mailtemplate.SceneBindEmail, "email", nextEmail))
	digests = append(digests, s.verification.ProofDigest(input.NextChallengeID, input.NextCode))
	if err := s.checkProofAttempts(ctx, actor.ClientIP, keys, digests); err != nil {
		return EmailResult{}, err
	}
	var result EmailResult
	err = s.authority.Mutate(ctx, current, func(mutationCtx context.Context) error {
		consumed, consumeErr := s.verification.ConsumeMany(mutationCtx, keys, digests)
		if consumeErr != nil {
			return apperror.DependencyUnavailable(consumeErr)
		}
		if !consumed {
			return apperror.Unauthorized(fmt.Errorf("email verification proof is invalid or expired"))
		}
		if changeErr := s.accounts.Change(mutationCtx, ChangeInput{UserID: actor.UserID, PlatformID: actor.PlatformID, Action: action, OldEmail: oldEmail, NewEmail: nextEmail,
			OldHint: sharedemail.Hint(oldEmail), OldHMAC: s.emailHMAC(oldEmail), NewHint: sharedemail.Hint(nextEmail), NewHMAC: s.emailHMAC(nextEmail), Now: s.now().UTC()}); changeErr != nil {
			return mapRepositoryError(changeErr)
		}
		result = EmailResult{Email: nextEmail}
		return nil
	})
	if err != nil {
		var public *apperror.Error
		if errors.As(err, &public) {
			return EmailResult{}, public
		}
		if errors.Is(err, authstate.ErrUpdating) || errors.Is(err, authstate.ErrGenerationChanged) || errors.Is(err, authstate.ErrMutationTokenMismatch) {
			return EmailResult{}, apperror.Conflict(i18n.KeyConflict, nil, err)
		}
		return EmailResult{}, apperror.DependencyUnavailable(err)
	}
	return result, nil
}

func (s *Service) checkProofAttempts(ctx context.Context, clientIP string, keys, digests []string) error {
	for index := range keys {
		valid, limited, err := s.verification.CheckAttempt(ctx, keys[index], digests[index], clientIP)
		if err != nil {
			return apperror.DependencyUnavailable(err)
		}
		if limited {
			return apperror.RateLimited(fmt.Errorf("email verification attempts exceeded"))
		}
		if !valid {
			return apperror.Unauthorized(fmt.Errorf("email verification proof is invalid or expired"))
		}
	}
	return nil
}

func (s *Service) deliver(ctx context.Context, actor Actor, destination, challengeID string) (SendCodeResult, error) {
	key := s.verification.VerificationKey(actor.Platform, mailtemplate.SceneBindEmail, "email", destination)
	leaseToken, err := s.generateID()
	if err != nil {
		return SendCodeResult{}, apperror.Internal(err)
	}
	acquired, err := s.verification.AcquireDelivery(ctx, key, leaseToken, 10*time.Second)
	if err != nil {
		return SendCodeResult{}, apperror.DependencyUnavailable(err)
	}
	if !acquired {
		return SendCodeResult{}, apperror.Conflict(i18n.KeyConflict, nil, fmt.Errorf("verification delivery is already in progress"))
	}
	release := func() error {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		return s.verification.ReleaseDelivery(cleanup, key, leaseToken)
	}
	prep, err := s.sender.PrepareEmailVerifyCode(ctx, messagemail.EmailVerifyCodePrepareInput{PlatformID: actor.PlatformID, ClientIP: actor.ClientIP, Scene: mailtemplate.SceneBindEmail, ToEmail: destination})
	if err != nil {
		return SendCodeResult{}, errors.Join(err, release())
	}
	if prep.TTLMinutes < 1 || prep.TTLMinutes > 60 || prep.ResendAfterSeconds < 0 || prep.ResendAfterSeconds > 86400 {
		return SendCodeResult{}, apperror.DependencyUnavailable(errors.Join(fmt.Errorf("mail preparation is invalid"), release()))
	}
	code, err := s.generateCode()
	if err != nil {
		return SendCodeResult{}, apperror.Internal(errors.Join(err, release()))
	}
	if challengeID == "" {
		challengeID = leaseToken
	}
	if err := s.verification.Put(ctx, key, s.verification.ProofDigest(challengeID, code), leaseToken, time.Duration(prep.TTLMinutes)*time.Minute); err != nil {
		return SendCodeResult{}, apperror.DependencyUnavailable(errors.Join(err, release()))
	}
	expiresAt := s.now().UTC().Add(time.Duration(prep.TTLMinutes) * time.Minute)
	userID := actor.UserID
	_, sendErr := s.sender.SendPreparedEmailVerifyCode(ctx, messagemail.EmailVerifyCodeInput{PlatformID: actor.PlatformID, UserID: &userID, ClientIP: actor.ClientIP, ChallengeID: challengeID, Scene: mailtemplate.SceneBindEmail, ToEmail: destination, Code: code, ExpiresAt: expiresAt, Preparation: prep})
	if sendErr != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		cleanupErr := errors.Join(s.verification.DeleteIfOwned(cleanup, key, leaseToken), s.verification.ReleaseDelivery(cleanup, key, leaseToken))
		cancel()
		if cleanupErr != nil {
			return SendCodeResult{}, apperror.DependencyUnavailable(errors.Join(sendErr, cleanupErr))
		}
		return SendCodeResult{}, sendErr
	}
	if err := release(); err != nil {
		return SendCodeResult{}, apperror.DependencyUnavailable(err)
	}
	return SendCodeResult{ChallengeID: challengeID, ExpiresAt: expiresAt, ResendAfterSeconds: prep.ResendAfterSeconds}, nil
}

func normalizeEmail(value string) (string, error) {
	return sharedemail.Normalize(value)
}

func (s *Service) emailHMAC(value string) string {
	if value == "" {
		return ""
	}
	mac := hmac.New(sha256.New, s.keys.MailRecipientHMACKey())
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func validateActor(actor Actor) error {
	if actor.UserID < 1 || actor.SessionID < 1 || actor.PlatformID < 1 || actor.Platform == "" {
		return apperror.Unauthorized(fmt.Errorf("authentication identity is missing"))
	}
	return nil
}
func validProof(challengeID, code string) bool { return validChallenge(challengeID) && validCode(code) }
func validChallenge(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for i := range value {
		if value[i] <= 0x20 || value[i] == 0x7f {
			return false
		}
	}
	return true
}
func validCode(value string) bool {
	if len(value) != 6 {
		return false
	}
	for i := range value {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}
func newCode() (string, error) {
	var value [4]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate verification code: %w", err)
	}
	return fmt.Sprintf("%06d", binary.BigEndian.Uint32(value[:])%1_000_000), nil
}
func newID() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate email challenge: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}
func mapRepositoryError(err error) error {
	switch {
	case errors.Is(err, ErrEmailConflict):
		return emailConflict(err)
	case errors.Is(err, ErrCurrentEmailChanged):
		return apperror.Conflict(i18n.KeyConflict, nil, err)
	case errors.Is(err, gorm.ErrRecordNotFound):
		return apperror.NotFound(err)
	default:
		return apperror.DependencyUnavailable(err)
	}
}
func emailConflict(err error) error { return apperror.Conflict(i18n.KeyEmailConflict, nil, err) }
