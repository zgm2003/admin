package email

import (
	"context"
	"errors"
	"testing"
	"time"

	authstate "admin/server/internal/module/auth/state"
	messagemail "admin/server/internal/module/message/mail"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
)

type fakeAccountStore struct {
	current     Current
	emailInUse  bool
	changeCalls int
	changeInput ChangeInput
}

func (f *fakeAccountStore) Current(context.Context, int64) (Current, error) { return f.current, nil }
func (f *fakeAccountStore) EmailInUse(context.Context, int64, string) (bool, error) {
	return f.emailInUse, nil
}
func (f *fakeAccountStore) Change(_ context.Context, input ChangeInput) error {
	f.changeCalls++
	f.changeInput = input
	return nil
}

type fakeVerificationStore struct {
	valid        bool
	calls        int
	keys         []string
	digests      []string
	checkInvalid bool
	checkLimited bool
	checkCalls   int
	checkIPs     []string
}

type fakeEmailSender struct {
	prepareInput messagemail.EmailVerifyCodePrepareInput
	sendInput    messagemail.EmailVerifyCodeInput
}

func (*fakeEmailSender) VerifyCodeReady(context.Context, string) (messagemail.VerifyCodeReadiness, error) {
	return messagemail.VerifyCodeReadiness{Ready: true, TTLMinutes: 5}, nil
}
func (f *fakeEmailSender) PrepareEmailVerifyCode(_ context.Context, input messagemail.EmailVerifyCodePrepareInput) (messagemail.EmailVerifyCodePreparation, error) {
	f.prepareInput = input
	return messagemail.EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60}, nil
}
func (f *fakeEmailSender) SendPreparedEmailVerifyCode(_ context.Context, input messagemail.EmailVerifyCodeInput) (messagemail.EmailVerifyCodeResult, error) {
	f.sendInput = input
	return messagemail.EmailVerifyCodeResult{ChallengeID: input.ChallengeID, ExpiresAt: input.ExpiresAt}, nil
}

func (f *fakeVerificationStore) VerificationKey(platform, scene, loginType, account string) string {
	return platform + ":" + scene + ":" + loginType + ":" + account
}
func (*fakeVerificationStore) ProofDigest(challengeID, code string) string {
	return "digest:" + challengeID + ":" + code
}
func (*fakeVerificationStore) AcquireDelivery(context.Context, string, string, time.Duration) (bool, error) {
	return true, nil
}
func (*fakeVerificationStore) Put(context.Context, string, string, string, time.Duration) error {
	return nil
}
func (f *fakeVerificationStore) CheckAttempt(_ context.Context, _ string, _ string, clientIP string) (bool, bool, error) {
	f.checkCalls++
	f.checkIPs = append(f.checkIPs, clientIP)
	return !f.checkInvalid && !f.checkLimited, f.checkLimited, nil
}
func (f *fakeVerificationStore) ConsumeMany(_ context.Context, keys, digests []string) (bool, error) {
	f.calls++
	f.keys = append([]string(nil), keys...)
	f.digests = append([]string(nil), digests...)
	return f.valid, nil
}
func (*fakeVerificationStore) DeleteIfOwned(context.Context, string, string) error   { return nil }
func (*fakeVerificationStore) ReleaseDelivery(context.Context, string, string) error { return nil }

type fakeAuthority struct {
	calls int
	err   error
}

func (f *fakeAuthority) Mutate(ctx context.Context, current Current, mutation func(context.Context) error) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	return mutation(ctx)
}
func testKeys(t *testing.T) *secretkey.KeyRing {
	keys, err := secretkey.New("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	return keys
}
func actor() Actor { return Actor{UserID: 7, SessionID: 8, PlatformID: 1, Platform: "admin"} }

func TestBindOrChangeFirstBindConsumesOnlyNextProof(t *testing.T) {
	accounts := &fakeAccountStore{current: Current{UserID: 7, IsEnabled: true}}
	verification := &fakeVerificationStore{valid: true}
	service := NewService(accounts, nil, verification, testKeys(t), &fakeAuthority{})
	result, err := service.BindOrChange(context.Background(), actor(), BindOrChangeInput{NextEmail: " USER@Example.COM ", NextChallengeID: "next", NextCode: "123456"})
	if err != nil || result.Email != "user@example.com" || verification.calls != 1 || len(verification.keys) != 1 || accounts.changeCalls != 1 {
		t.Fatalf("result=%+v err=%v calls=%d keys=%v changes=%d", result, err, verification.calls, verification.keys, accounts.changeCalls)
	}
	if len(verification.digests) != 1 || verification.digests[0] != "digest:next:123456" || accounts.changeInput.NewHint != "u***@example.com" {
		t.Fatalf("digests=%v change=%+v", verification.digests, accounts.changeInput)
	}
}

func TestBindOrChangeExistingEmailRequiresBothProofsAndDoesNotConsumeOnInvalidProof(t *testing.T) {
	old := "old@example.com"
	accounts := &fakeAccountStore{current: Current{UserID: 7, Email: &old, IsEnabled: true}}
	verification := &fakeVerificationStore{valid: false}
	service := NewService(accounts, nil, verification, testKeys(t), &fakeAuthority{})
	_, err := service.BindOrChange(context.Background(), actor(), BindOrChangeInput{CurrentChallengeID: "current", CurrentCode: "111111", NextEmail: "new@example.com", NextChallengeID: "next", NextCode: "222222"})
	if appCode(err) != apperror.CodeUnauthorized || verification.calls != 1 || len(verification.keys) != 2 || accounts.changeCalls != 0 {
		t.Fatalf("err=%v calls=%d keys=%v changes=%d", err, verification.calls, verification.keys, accounts.changeCalls)
	}
	if len(verification.digests) != 2 || verification.digests[0] != "digest:current:111111" || verification.digests[1] != "digest:next:222222" {
		t.Fatalf("proof order=%v", verification.digests)
	}
}

func TestBindOrChangeLimitsInvalidProofAttemptsBeforeAtomicConsumption(t *testing.T) {
	old := "old@example.com"
	accounts := &fakeAccountStore{current: Current{UserID: 7, Email: &old, IsEnabled: true}}
	verification := &fakeVerificationStore{valid: true, checkInvalid: true}
	service := NewService(accounts, nil, verification, testKeys(t), &fakeAuthority{})
	inputActor := actor()
	inputActor.ClientIP = "192.0.2.8"
	input := BindOrChangeInput{CurrentChallengeID: "current", CurrentCode: "111111", NextEmail: "new@example.com", NextChallengeID: "next", NextCode: "222222"}

	_, err := service.BindOrChange(context.Background(), inputActor, input)
	if appCode(err) != apperror.CodeUnauthorized || verification.checkCalls != 1 || verification.calls != 0 || len(verification.checkIPs) != 1 || verification.checkIPs[0] != inputActor.ClientIP {
		t.Fatalf("invalid attempt err=%v checks=%d consume=%d ips=%v", err, verification.checkCalls, verification.calls, verification.checkIPs)
	}

	verification.checkInvalid = false
	verification.checkLimited = true
	verification.checkCalls = 0
	verification.checkIPs = nil
	_, err = service.BindOrChange(context.Background(), inputActor, input)
	if appCode(err) != apperror.CodeRateLimited || verification.checkCalls != 1 || verification.calls != 0 {
		t.Fatalf("limited attempt err=%v checks=%d consume=%d", err, verification.checkCalls, verification.calls)
	}
}

func TestSendCodeRejectsDisabledAccount(t *testing.T) {
	current := "old@example.com"
	accounts := &fakeAccountStore{current: Current{UserID: 7, Email: &current, IsEnabled: false}}
	service := NewService(accounts, &fakeEmailSender{}, &fakeVerificationStore{}, testKeys(t), &fakeAuthority{})
	_, err := service.SendCode(context.Background(), actor(), SendCodeInput{Target: TargetCurrent})
	if appCode(err) != apperror.CodeForbidden {
		t.Fatalf("disabled account error=%v", err)
	}
}

func TestBindOrChangeRejectsSameOrOccupiedEmailBeforeProofConsumption(t *testing.T) {
	old := "user@example.com"
	accounts := &fakeAccountStore{current: Current{UserID: 7, Email: &old, IsEnabled: true}}
	verification := &fakeVerificationStore{valid: true}
	service := NewService(accounts, nil, verification, testKeys(t), &fakeAuthority{})
	_, err := service.BindOrChange(context.Background(), actor(), BindOrChangeInput{CurrentChallengeID: "current", CurrentCode: "111111", NextEmail: old, NextChallengeID: "next", NextCode: "222222"})
	if appCode(err) != apperror.CodeConflict || verification.calls != 0 {
		t.Fatalf("same email err=%v calls=%d", err, verification.calls)
	}
	accounts.current.Email = nil
	accounts.emailInUse = true
	_, err = service.BindOrChange(context.Background(), actor(), BindOrChangeInput{NextEmail: "new@example.com", NextChallengeID: "next", NextCode: "222222"})
	if appCode(err) != apperror.CodeConflict || verification.calls != 0 {
		t.Fatalf("occupied email err=%v calls=%d", err, verification.calls)
	}
}

func TestBindOrChangeMapsAuthorityGenerationConflictToConflict(t *testing.T) {
	old := "old@example.com"
	accounts := &fakeAccountStore{current: Current{UserID: 7, Email: &old, IsEnabled: true}}
	verification := &fakeVerificationStore{valid: true}
	authority := &fakeAuthority{err: authstate.ErrGenerationChanged}
	service := NewService(accounts, nil, verification, testKeys(t), authority)
	_, err := service.BindOrChange(context.Background(), actor(), BindOrChangeInput{
		CurrentChallengeID: "current", CurrentCode: "111111",
		NextEmail: "new@example.com", NextChallengeID: "next", NextCode: "222222",
	})
	if appCode(err) != apperror.CodeConflict {
		t.Fatalf("authority error=%v, want conflict", err)
	}
}

func TestSendCodePassesAuthenticatedClientIPToMail(t *testing.T) {
	current := "old@example.com"
	accounts := &fakeAccountStore{current: Current{UserID: 7, Email: &current, IsEnabled: true}}
	verification := &fakeVerificationStore{}
	sender := &fakeEmailSender{}
	service := NewService(accounts, sender, verification, testKeys(t), &fakeAuthority{})
	service.generateCode = func() (string, error) { return "123456", nil }
	service.generateID = func() (string, error) { return "generated-challenge", nil }
	inputActor := actor()
	inputActor.ClientIP = "192.0.2.8"
	result, err := service.SendCode(context.Background(), inputActor, SendCodeInput{Target: TargetCurrent})
	if err != nil {
		t.Fatal(err)
	}
	if sender.prepareInput.ClientIP != "192.0.2.8" || sender.sendInput.ClientIP != "192.0.2.8" || result.ChallengeID != "generated-challenge" {
		t.Fatalf("prepare=%+v send=%+v result=%+v", sender.prepareInput, sender.sendInput, result)
	}
}

func appCode(err error) int {
	var value *apperror.Error
	if errors.As(err, &value) {
		return value.Code
	}
	return 0
}
