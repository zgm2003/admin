package phone

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
	messagesms "admin/server/internal/module/message/sms"
	smstemplate "admin/server/internal/module/message/sms/template"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	sharedphone "admin/server/internal/shared/phone"
	"gorm.io/gorm"
)

type Service struct {
	accounts     accountStore
	sender       phoneCodeSender
	verification VerificationCodeStore
	keys         *secretkey.KeyRing
	authority    authorityCoordinator
	now          func() time.Time
	generateCode func() (string, error)
	generateID   func() (string, error)
}

func NewService(accounts accountStore, sender phoneCodeSender, verification VerificationCodeStore, keys *secretkey.KeyRing, authority authorityCoordinator) *Service {
	return &Service{accounts: accounts, sender: sender, verification: verification, keys: keys, authority: authority, now: time.Now, generateCode: newCode, generateID: newID}
}

func (s *Service) SendCode(ctx context.Context, actor Actor, input SendCodeInput) (SendCodeResult, error) {
	if err := validateActor(actor); err != nil {
		return SendCodeResult{}, err
	}
	if input.ChallengeID != "" && !validChallenge(input.ChallengeID) {
		return SendCodeResult{}, apperror.InvalidRequest(fmt.Errorf("challengeId is invalid"))
	}
	if input.Target != TargetCurrent && input.Target != TargetNext {
		return SendCodeResult{}, apperror.InvalidRequest(fmt.Errorf("phone target is invalid"))
	}
	if input.Target == TargetCurrent && input.Phone != nil {
		return SendCodeResult{}, apperror.InvalidRequest(fmt.Errorf("current phone must not be supplied"))
	}
	if input.Target == TargetNext && input.Phone == nil {
		return SendCodeResult{}, apperror.InvalidRequest(fmt.Errorf("next phone is required"))
	}
	if s.accounts == nil || s.sender == nil || s.verification == nil {
		return SendCodeResult{}, apperror.DependencyUnavailable(fmt.Errorf("phone verification dependencies are unavailable"))
	}
	current, err := s.accounts.Current(ctx, actor.UserID)
	if err != nil {
		return SendCodeResult{}, mapRepositoryError(err)
	}
	if current.Deleted {
		return SendCodeResult{}, apperror.NotFound(fmt.Errorf("phone account is deleted"))
	}
	if !current.IsEnabled {
		return SendCodeResult{}, apperror.Forbidden(fmt.Errorf("phone account is disabled"))
	}
	var destination string
	if input.Target == TargetCurrent {
		if current.Phone == nil {
			return SendCodeResult{}, apperror.NotFound(fmt.Errorf("phone is not bound"))
		}
		destination = *current.Phone
	} else {
		destination, err = sharedphone.Normalize(*input.Phone)
		if err != nil {
			return SendCodeResult{}, apperror.InvalidRequest(err)
		}
		if current.Phone != nil && destination == *current.Phone {
			return SendCodeResult{}, apperror.Conflict(i18n.KeyConflict, nil, fmt.Errorf("next phone matches current phone"))
		}
		inUse, checkErr := s.accounts.PhoneInUse(ctx, actor.UserID, destination)
		if checkErr != nil {
			return SendCodeResult{}, apperror.DependencyUnavailable(checkErr)
		}
		if inUse {
			return SendCodeResult{}, phoneConflict(ErrPhoneConflict)
		}
	}
	return s.deliver(ctx, actor, destination, input.ChallengeID)
}

func (s *Service) BindOrChange(ctx context.Context, actor Actor, input BindOrChangeInput) (PhoneResult, error) {
	if err := validateActor(actor); err != nil {
		return PhoneResult{}, err
	}
	if s.accounts == nil || s.verification == nil || s.keys == nil || s.authority == nil {
		return PhoneResult{}, apperror.DependencyUnavailable(fmt.Errorf("phone mutation dependencies are unavailable"))
	}
	if !validProof(input.NextChallengeID, input.NextCode) {
		return PhoneResult{}, apperror.InvalidRequest(fmt.Errorf("next phone proof is invalid"))
	}
	nextPhone, err := sharedphone.Normalize(input.NextPhone)
	if err != nil {
		return PhoneResult{}, apperror.InvalidRequest(err)
	}
	current, err := s.accounts.Current(ctx, actor.UserID)
	if err != nil {
		return PhoneResult{}, mapRepositoryError(err)
	}
	if current.Deleted {
		return PhoneResult{}, apperror.NotFound(fmt.Errorf("phone account is deleted"))
	}
	if !current.IsEnabled {
		return PhoneResult{}, apperror.Forbidden(fmt.Errorf("phone account is disabled"))
	}
	oldPhone := ""
	action := ActionBind
	keys := make([]string, 0, 2)
	digests := make([]string, 0, 2)
	if current.Phone == nil {
		if input.CurrentChallengeID != "" || input.CurrentCode != "" {
			return PhoneResult{}, apperror.InvalidRequest(fmt.Errorf("current phone proof is not allowed for first bind"))
		}
	} else {
		oldPhone = *current.Phone
		action = ActionChange
		if nextPhone == oldPhone {
			return PhoneResult{}, apperror.Conflict(i18n.KeyConflict, nil, fmt.Errorf("next phone matches current phone"))
		}
		if !validProof(input.CurrentChallengeID, input.CurrentCode) {
			return PhoneResult{}, apperror.InvalidRequest(fmt.Errorf("current phone proof is required"))
		}
		keys = append(keys, s.verification.VerificationKey(actor.Platform, smstemplate.SceneBindPhone, "phone", oldPhone))
		digests = append(digests, s.verification.ProofDigest(input.CurrentChallengeID, input.CurrentCode))
	}
	inUse, err := s.accounts.PhoneInUse(ctx, actor.UserID, nextPhone)
	if err != nil {
		return PhoneResult{}, apperror.DependencyUnavailable(err)
	}
	if inUse {
		return PhoneResult{}, phoneConflict(ErrPhoneConflict)
	}
	keys = append(keys, s.verification.VerificationKey(actor.Platform, smstemplate.SceneBindPhone, "phone", nextPhone))
	digests = append(digests, s.verification.ProofDigest(input.NextChallengeID, input.NextCode))
	if err := s.checkProofAttempts(ctx, actor.ClientIP, keys, digests); err != nil {
		return PhoneResult{}, err
	}

	var result PhoneResult
	err = s.authority.Mutate(ctx, current, func(mutationCtx context.Context) error {
		consumed, consumeErr := s.verification.ConsumeMany(mutationCtx, keys, digests)
		if consumeErr != nil {
			return apperror.DependencyUnavailable(consumeErr)
		}
		if !consumed {
			return apperror.Unauthorized(fmt.Errorf("phone verification proof is invalid or expired"))
		}
		change := ChangeInput{
			UserID: actor.UserID, PlatformID: actor.PlatformID, Action: action,
			OldPhone: oldPhone, NewPhone: nextPhone,
			OldHint: sharedphone.Hint(oldPhone), OldHMAC: s.phoneHMAC(oldPhone),
			NewHint: sharedphone.Hint(nextPhone), NewHMAC: s.phoneHMAC(nextPhone), Now: s.now().UTC(),
		}
		if changeErr := s.accounts.Change(mutationCtx, change); changeErr != nil {
			return mapRepositoryError(changeErr)
		}
		result = PhoneResult{Phone: nextPhone}
		return nil
	})
	if err != nil {
		var public *apperror.Error
		if errors.As(err, &public) {
			return PhoneResult{}, public
		}
		if errors.Is(err, authstate.ErrUpdating) || errors.Is(err, authstate.ErrGenerationChanged) || errors.Is(err, authstate.ErrMutationTokenMismatch) {
			return PhoneResult{}, apperror.Conflict(i18n.KeyConflict, nil, err)
		}
		return PhoneResult{}, apperror.DependencyUnavailable(err)
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
			return apperror.RateLimited(fmt.Errorf("phone verification attempts exceeded"))
		}
		if !valid {
			return apperror.Unauthorized(fmt.Errorf("phone verification proof is invalid or expired"))
		}
	}
	return nil
}

func (s *Service) deliver(ctx context.Context, actor Actor, destination, challengeID string) (SendCodeResult, error) {
	key := s.verification.VerificationKey(actor.Platform, smstemplate.SceneBindPhone, "phone", destination)
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
	preparation, err := s.sender.PreparePhoneVerifyCode(ctx, messagesms.PhoneVerifyCodePrepareInput{PlatformID: actor.PlatformID, Scene: smstemplate.SceneBindPhone, ToPhone: destination})
	if err != nil {
		return SendCodeResult{}, errors.Join(err, release())
	}
	if preparation.TTLMinutes < 1 || preparation.TTLMinutes > 60 || preparation.ResendAfterSeconds < 0 || preparation.ResendAfterSeconds > 86400 {
		return SendCodeResult{}, apperror.DependencyUnavailable(errors.Join(fmt.Errorf("sms preparation is invalid"), release()))
	}
	code, err := s.generateCode()
	if err != nil {
		return SendCodeResult{}, apperror.Internal(errors.Join(err, release()))
	}
	if challengeID == "" {
		challengeID = leaseToken
	}
	if err := s.verification.Put(ctx, key, s.verification.ProofDigest(challengeID, code), leaseToken, time.Duration(preparation.TTLMinutes)*time.Minute); err != nil {
		return SendCodeResult{}, apperror.DependencyUnavailable(errors.Join(err, release()))
	}
	expiresAt := s.now().UTC().Add(time.Duration(preparation.TTLMinutes) * time.Minute)
	userID := actor.UserID
	_, sendErr := s.sender.SendPreparedPhoneVerifyCode(ctx, messagesms.PhoneVerifyCodeInput{
		PlatformID: actor.PlatformID, UserID: &userID, ChallengeID: challengeID, Scene: smstemplate.SceneBindPhone,
		ToPhone: destination, Code: code, ExpiresAt: expiresAt, Preparation: preparation,
	})
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
	return SendCodeResult{ChallengeID: challengeID, ExpiresAt: expiresAt, ResendAfterSeconds: preparation.ResendAfterSeconds}, nil
}

func (s *Service) phoneHMAC(value string) string {
	if value == "" {
		return ""
	}
	mac := hmac.New(sha256.New, s.keys.SMSRecipientHMACKey())
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
	for index := range value {
		if value[index] <= 0x20 || value[index] == 0x7f {
			return false
		}
	}
	return true
}

func validCode(value string) bool {
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
		return "", fmt.Errorf("generate phone challenge: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}

func mapRepositoryError(err error) error {
	switch {
	case errors.Is(err, ErrPhoneConflict):
		return phoneConflict(err)
	case errors.Is(err, ErrCurrentPhoneChanged):
		return apperror.Conflict(i18n.KeyConflict, nil, err)
	case errors.Is(err, gorm.ErrRecordNotFound):
		return apperror.NotFound(err)
	default:
		return apperror.DependencyUnavailable(err)
	}
}

func phoneConflict(err error) error {
	return apperror.Conflict(i18n.KeyUserPhoneConflict, nil, err)
}
