package phone

import (
	"context"
	"errors"
	"testing"
	"time"

	messagesms "admin/server/internal/module/message/sms"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
)

type fakePhoneSender struct{}

func (fakePhoneSender) VerifyCodeReady(context.Context, string) (messagesms.VerifyCodeReadiness, error) {
	return messagesms.VerifyCodeReadiness{Ready: true, TTLMinutes: 5}, nil
}
func (fakePhoneSender) PreparePhoneVerifyCode(context.Context, messagesms.PhoneVerifyCodePrepareInput) (messagesms.PhoneVerifyCodePreparation, error) {
	return messagesms.PhoneVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60}, nil
}
func (fakePhoneSender) SendPreparedPhoneVerifyCode(_ context.Context, input messagesms.PhoneVerifyCodeInput) (messagesms.PhoneVerifyCodeResult, error) {
	return messagesms.PhoneVerifyCodeResult{}, nil
}

type fakeAccountStore struct {
	current      Current
	currentErr   error
	phoneInUse   bool
	inUseErr     error
	changeErr    error
	changeInput  ChangeInput
	currentCalls int
	inUseCalls   int
	changeCalls  int
}

func (f *fakeAccountStore) Current(_ context.Context, _ int64) (Current, error) {
	f.currentCalls++
	return f.current, f.currentErr
}

func (f *fakeAccountStore) PhoneInUse(_ context.Context, _ int64, _ string) (bool, error) {
	f.inUseCalls++
	return f.phoneInUse, f.inUseErr
}

func (f *fakeAccountStore) Change(_ context.Context, input ChangeInput) error {
	f.changeCalls++
	f.changeInput = input
	return f.changeErr
}

type fakeVerificationStore struct {
	consumeValid   bool
	consumeErr     error
	consumeKeys    []string
	consumeDigests []string
	consumeMany    int
	checkInvalid   bool
	checkLimited   bool
	checkCalls     int
	checkIPs       []string
}

func (*fakeVerificationStore) VerificationKey(platform, scene, loginType, account string) string {
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
	f.consumeMany++
	f.consumeKeys = append([]string(nil), keys...)
	f.consumeDigests = append([]string(nil), digests...)
	return f.consumeValid, f.consumeErr
}

func (*fakeVerificationStore) DeleteIfOwned(context.Context, string, string) error { return nil }

func (*fakeVerificationStore) ReleaseDelivery(context.Context, string, string) error { return nil }

type fakeAuthorityCoordinator struct {
	calls int
	err   error
}

func (f *fakeAuthorityCoordinator) Mutate(_ context.Context, _ Current, mutation func(context.Context) error) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	return mutation(context.Background())
}

func testPhoneKeys(t *testing.T) *secretkey.KeyRing {
	t.Helper()
	keys, err := secretkey.New("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

func TestBindOrChangeFirstBindConsumesOnlyNextProof(t *testing.T) {
	accounts := &fakeAccountStore{current: Current{UserID: 7, IsEnabled: true}}
	verification := &fakeVerificationStore{consumeValid: true}
	authority := &fakeAuthorityCoordinator{}
	service := NewService(accounts, nil, verification, testPhoneKeys(t), authority)
	service.now = func() time.Time { return time.Date(2026, 9, 11, 1, 2, 3, 0, time.UTC) }

	result, err := service.BindOrChange(context.Background(), Actor{UserID: 7, SessionID: 8, PlatformID: 1, Platform: "admin"}, BindOrChangeInput{
		NextPhone: "86 156-7162-8271", NextChallengeID: "next-challenge", NextCode: "123456",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Phone != "+8615671628271" || verification.consumeMany != 1 || len(verification.consumeKeys) != 1 {
		t.Fatalf("result=%+v consumeMany=%d keys=%v", result, verification.consumeMany, verification.consumeKeys)
	}
	if accounts.changeCalls != 1 || accounts.changeInput.Action != ActionBind || accounts.changeInput.OldPhone != "" || accounts.changeInput.NewPhone != result.Phone {
		t.Fatalf("change=%+v calls=%d", accounts.changeInput, accounts.changeCalls)
	}
	if authority.calls != 1 {
		t.Fatalf("authority mutations=%d", authority.calls)
	}
}

func TestBindOrChangeExistingPhoneRequiresAndAtomicallyConsumesBothProofs(t *testing.T) {
	currentPhone := "+8613800000000"
	accounts := &fakeAccountStore{current: Current{UserID: 7, Phone: &currentPhone, IsEnabled: true}}
	verification := &fakeVerificationStore{consumeValid: true}
	service := NewService(accounts, nil, verification, testPhoneKeys(t), &fakeAuthorityCoordinator{})

	_, err := service.BindOrChange(context.Background(), Actor{UserID: 7, SessionID: 8, PlatformID: 1, Platform: "admin"}, BindOrChangeInput{
		NextPhone: "15671628271", NextChallengeID: "next", NextCode: "222222",
	})
	if appErrorCode(err) != apperror.CodeInvalidRequest || verification.consumeMany != 0 {
		t.Fatalf("missing current proof error=%v consumeMany=%d", err, verification.consumeMany)
	}

	result, err := service.BindOrChange(context.Background(), Actor{UserID: 7, SessionID: 8, PlatformID: 1, Platform: "admin"}, BindOrChangeInput{
		CurrentChallengeID: "current", CurrentCode: "111111", NextPhone: "15671628271", NextChallengeID: "next", NextCode: "222222",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Phone != "+8615671628271" || verification.consumeMany != 1 || len(verification.consumeKeys) != 2 || accounts.changeInput.Action != ActionChange {
		t.Fatalf("result=%+v keys=%v change=%+v", result, verification.consumeKeys, accounts.changeInput)
	}
	if len(verification.consumeDigests) != 2 || verification.consumeDigests[0] != "digest:current:111111" || verification.consumeDigests[1] != "digest:next:222222" {
		t.Fatalf("proof order=%v", verification.consumeDigests)
	}
}

func TestBindOrChangeRejectsUnavailableOrReusedProofBeforeDatabaseMutation(t *testing.T) {
	accounts := &fakeAccountStore{current: Current{UserID: 7, IsEnabled: true}}
	verification := &fakeVerificationStore{consumeValid: false}
	service := NewService(accounts, nil, verification, testPhoneKeys(t), &fakeAuthorityCoordinator{})

	_, err := service.BindOrChange(context.Background(), Actor{UserID: 7, SessionID: 8, PlatformID: 1, Platform: "admin"}, BindOrChangeInput{
		NextPhone: "15671628271", NextChallengeID: "next", NextCode: "123456",
	})
	if appErrorCode(err) != apperror.CodeUnauthorized || verification.consumeMany != 1 || accounts.changeCalls != 0 {
		t.Fatalf("error=%v consumeMany=%d changeCalls=%d", err, verification.consumeMany, accounts.changeCalls)
	}

	verification.consumeErr = errors.New("redis unavailable")
	_, err = service.BindOrChange(context.Background(), Actor{UserID: 7, SessionID: 8, PlatformID: 1, Platform: "admin"}, BindOrChangeInput{
		NextPhone: "15671628271", NextChallengeID: "next", NextCode: "123456",
	})
	if appErrorCode(err) != apperror.CodeDependencyUnavailable || accounts.changeCalls != 0 {
		t.Fatalf("redis error=%v changeCalls=%d", err, accounts.changeCalls)
	}
}

func TestBindOrChangeLimitsInvalidProofAttemptsBeforeAtomicConsumption(t *testing.T) {
	currentPhone := "+8613800000000"
	accounts := &fakeAccountStore{current: Current{UserID: 7, Phone: &currentPhone, IsEnabled: true}}
	verification := &fakeVerificationStore{consumeValid: true, checkInvalid: true}
	service := NewService(accounts, nil, verification, testPhoneKeys(t), &fakeAuthorityCoordinator{})
	actor := Actor{UserID: 7, SessionID: 8, PlatformID: 1, Platform: "admin", ClientIP: "192.0.2.8"}
	input := BindOrChangeInput{CurrentChallengeID: "current", CurrentCode: "111111", NextPhone: "15671628271", NextChallengeID: "next", NextCode: "222222"}

	_, err := service.BindOrChange(context.Background(), actor, input)
	if appErrorCode(err) != apperror.CodeUnauthorized || verification.checkCalls != 1 || verification.consumeMany != 0 || len(verification.checkIPs) != 1 || verification.checkIPs[0] != actor.ClientIP {
		t.Fatalf("invalid attempt err=%v checks=%d consume=%d ips=%v", err, verification.checkCalls, verification.consumeMany, verification.checkIPs)
	}

	verification.checkInvalid = false
	verification.checkLimited = true
	verification.checkCalls = 0
	verification.checkIPs = nil
	_, err = service.BindOrChange(context.Background(), actor, input)
	if appErrorCode(err) != apperror.CodeRateLimited || verification.checkCalls != 1 || verification.consumeMany != 0 {
		t.Fatalf("limited attempt err=%v checks=%d consume=%d", err, verification.checkCalls, verification.consumeMany)
	}
}

func TestSendCodeRejectsDisabledAccount(t *testing.T) {
	currentPhone := "+8613800000000"
	accounts := &fakeAccountStore{current: Current{UserID: 7, Phone: &currentPhone, IsEnabled: false}}
	service := NewService(accounts, fakePhoneSender{}, &fakeVerificationStore{}, testPhoneKeys(t), &fakeAuthorityCoordinator{})
	_, err := service.SendCode(context.Background(), Actor{UserID: 7, SessionID: 8, PlatformID: 1, Platform: "admin"}, SendCodeInput{Target: TargetCurrent})
	if appErrorCode(err) != apperror.CodeForbidden {
		t.Fatalf("disabled account error=%v", err)
	}
}

func TestBindOrChangeRejectsSameOrOccupiedNextPhoneWithoutConsumingProof(t *testing.T) {
	currentPhone := "+8615671628271"
	accounts := &fakeAccountStore{current: Current{UserID: 7, Phone: &currentPhone, IsEnabled: true}}
	verification := &fakeVerificationStore{consumeValid: true}
	service := NewService(accounts, nil, verification, testPhoneKeys(t), &fakeAuthorityCoordinator{})

	_, err := service.BindOrChange(context.Background(), Actor{UserID: 7, SessionID: 8, PlatformID: 1, Platform: "admin"}, BindOrChangeInput{
		CurrentChallengeID: "current", CurrentCode: "111111", NextPhone: "15671628271", NextChallengeID: "next", NextCode: "222222",
	})
	if appErrorCode(err) != apperror.CodeConflict || verification.consumeMany != 0 {
		t.Fatalf("same phone error=%v consumeMany=%d", err, verification.consumeMany)
	}

	accounts.current.Phone = nil
	accounts.phoneInUse = true
	_, err = service.BindOrChange(context.Background(), Actor{UserID: 7, SessionID: 8, PlatformID: 1, Platform: "admin"}, BindOrChangeInput{
		NextPhone: "15671628271", NextChallengeID: "next", NextCode: "222222",
	})
	if appErrorCode(err) != apperror.CodeConflict || verification.consumeMany != 0 {
		t.Fatalf("occupied phone error=%v consumeMany=%d", err, verification.consumeMany)
	}
}

func TestSendCodeRejectsCurrentTargetWithPhoneInBody(t *testing.T) {
	value := "15671628271"
	service := NewService(&fakeAccountStore{}, nil, nil, nil, nil)
	_, err := service.SendCode(context.Background(), Actor{UserID: 7, SessionID: 8, PlatformID: 1, Platform: "admin"}, SendCodeInput{Target: TargetCurrent, Phone: &value})
	if appErrorCode(err) != apperror.CodeInvalidRequest {
		t.Fatalf("error=%v", err)
	}
}

func appErrorCode(err error) int {
	var value *apperror.Error
	if errors.As(err, &value) {
		return value.Code
	}
	return 0
}
