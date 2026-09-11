package sms

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	smslog "admin/server/internal/module/message/sms/log"
	"admin/server/internal/module/message/sms/logVerification"
	"admin/server/internal/module/message/sms/rateLimitPolicy"
	"admin/server/internal/module/message/sms/template"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
)

func TestDecryptRuntimeCredentialsRejectsCorruptCiphertext(t *testing.T) {
	keys, err := secretkey.New(strings.Repeat("s", 64))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = decryptRuntimeCredentials(keys, ConfiguredConfig{
		SecretIDCiphertext:  "sms:v1:corrupt-id",
		SecretKeyCiphertext: "sms:v1:corrupt-key",
	})
	if err == nil {
		t.Fatal("corrupt runtime credentials were accepted")
	}
}

type runtimeStoreTest struct {
	facts RuntimeFacts
	err   error
}

func (s runtimeStoreTest) Load(_ context.Context, _ func(context.Context) (RuntimeFacts, error)) (RuntimeFacts, error) {
	return s.facts, s.err
}
func (s runtimeStoreTest) LoadReadiness(_ context.Context, scene string, compute func(context.Context) (VerifyCodeReadiness, error)) (VerifyCodeReadiness, error) {
	return readinessOf(s.facts, scene)
}

type sendingLogStoreTest struct {
	createErr error
	finishErr error
	created   *smslog.Model
	finished  string
}

func (s *sendingLogStoreTest) CreatePending(_ context.Context, row *smslog.Model) error {
	if s.createErr != nil {
		return s.createErr
	}
	row.ID = 19
	s.created = row
	return nil
}
func (s *sendingLogStoreTest) FindActiveChallenge(context.Context, int64, string) (smslog.Model, error) {
	return smslog.Model{}, errors.New("not used")
}
func (s *sendingLogStoreTest) Finish(_ context.Context, _ int64, _ int64, status, _, _ string, _ int, _, _ string, _ int64, _ *time.Time, _ time.Time) error {
	s.finished = status
	return s.finishErr
}

type verificationStoreTest struct {
	err   error
	calls int
}

func (s *verificationStoreTest) Create(_ context.Context, _ *logVerification.Model) error {
	s.calls++
	return s.err
}

type policyStoreTest struct{}

func (policyStoreTest) Catalog(context.Context, int64) (rateLimitPolicy.Catalog, error) {
	return rateLimitPolicy.Catalog{Policies: []rateLimitPolicy.Model{
		{Key: rateLimitPolicy.KeyMinute, Limit: 1, WindowSeconds: 60},
		{Key: rateLimitPolicy.KeyTenMin, Limit: 5, WindowSeconds: 600},
	}}, nil
}

type limiterTest struct{ result LimitResult }

func (s limiterTest) Reserve(context.Context, ...LimitRequest) (LimitResult, error) {
	return s.result, nil
}

type senderTest struct {
	err   error
	calls int
	input SendInput
}

func (s *senderTest) Send(_ context.Context, input SendInput) (ProviderResult, error) {
	s.calls++
	s.input = input
	if s.err != nil {
		return ProviderResult{}, s.err
	}
	return ProviderResult{RequestID: "request-1", SerialNo: "serial-1", Fee: 1}, nil
}

func readySendingFixture(t *testing.T) (*Service, *sendingLogStoreTest, *verificationStoreTest, *senderTest) {
	t.Helper()
	keys, err := secretkey.New(strings.Repeat("s", 64))
	if err != nil {
		t.Fatal(err)
	}
	secretID, _, err := secretkey.EncryptSMSValue(keys.SMSEncryptionKey(), "secret-id")
	if err != nil {
		t.Fatal(err)
	}
	secretKey, _, err := secretkey.EncryptSMSValue(keys.SMSEncryptionKey(), "secret-key")
	if err != nil {
		t.Fatal(err)
	}
	facts := RuntimeFacts{
		Configured: true,
		Config: ConfiguredConfig{
			SecretIDCiphertext: secretID, SecretKeyCiphertext: secretKey,
			SDKAppID: "sdk-app", SignName: "sign", Region: "ap-guangzhou",
			TTLMinutes: 5, IsEnabled: yesno.Yes,
		},
		Templates: map[string]TemplateFact{
			template.SceneLogin:          readyTemplateFact("登录验证码", "123456"),
			template.SceneForget:         readyTemplateFact("找回密码", "123457"),
			template.SceneBindPhone:      readyTemplateFact("绑定手机", "123458"),
			template.SceneChangePassword: readyTemplateFact("修改密码", "123459"),
		},
	}
	logs := &sendingLogStoreTest{}
	verification := &verificationStoreTest{}
	sender := &senderTest{}
	service := NewService(Stores{
		Runtime: runtimeStoreTest{facts: facts}, Log: logs, Verification: verification,
		Policy: policyStoreTest{}, Limiter: limiterTest{result: LimitResult{Allowed: true}}, Sender: sender,
	}, keys)
	service.now = func() time.Time { return time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC) }
	return service, logs, verification, sender
}

func readyTemplateFact(name, providerID string) TemplateFact {
	return TemplateFact{
		Name: name, TencentTemplateID: providerID,
		ParameterKeys:    []string{"code", "ttl_minutes"},
		ExampleVariables: map[string]string{"code": "123456", "ttl_minutes": "5"},
		IsEnabled:        yesno.Yes,
	}
}

func preparedSendingInput() PhoneVerifyCodeInput {
	return PhoneVerifyCodeInput{
		PlatformID: 2, ChallengeID: "challenge-1", Scene: template.SceneLogin,
		ToPhone: "15671628271", Code: "123456",
		ExpiresAt:   time.Date(2026, 9, 11, 12, 5, 0, 0, time.UTC),
		Preparation: PhoneVerifyCodePreparation{TTLMinutes: 5},
	}
}

func TestSendPreparedPhoneVerifyCodeClosesDeliveredLog(t *testing.T) {
	service, logs, verification, sender := readySendingFixture(t)
	result, err := service.SendPreparedPhoneVerifyCode(context.Background(), preparedSendingInput())
	if err != nil {
		t.Fatal(err)
	}
	if result.LogID != 19 || logs.finished != smslog.StatusSent || verification.calls != 1 || sender.calls != 1 {
		t.Fatalf("result=%+v finished=%q verification=%d sender=%d", result, logs.finished, verification.calls, sender.calls)
	}
	if sender.input.SecretID != "secret-id" || sender.input.SecretKey != "secret-key" || sender.input.ToPhone != "+8615671628271" {
		t.Fatalf("provider input=%+v", sender.input)
	}
}

func TestSendPreparedPhoneVerifyCodeDoesNotSendWhenAuditFails(t *testing.T) {
	service, logs, verification, sender := readySendingFixture(t)
	verification.err = errors.New("audit unavailable")
	_, err := service.SendPreparedPhoneVerifyCode(context.Background(), preparedSendingInput())
	if appErrorCode(err) != apperror.CodeDependencyUnavailable || sender.calls != 0 || logs.finished != smslog.StatusFailed {
		t.Fatalf("error=%v sender=%d finished=%q", err, sender.calls, logs.finished)
	}
}

func TestSendPreparedPhoneVerifyCodeSupportsExactlyTheFourScenes(t *testing.T) {
	for _, scene := range []string{template.SceneLogin, template.SceneForget, template.SceneBindPhone, template.SceneChangePassword} {
		t.Run(scene, func(t *testing.T) {
			service, logs, verification, sender := readySendingFixture(t)
			input := preparedSendingInput()
			input.Scene = scene
			if _, err := service.SendPreparedPhoneVerifyCode(context.Background(), input); err != nil {
				t.Fatal(err)
			}
			if logs.finished != smslog.StatusSent || verification.calls != 1 || sender.calls != 1 {
				t.Fatalf("finished=%q verification=%d sender=%d", logs.finished, verification.calls, sender.calls)
			}
		})
	}
	service, _, _, sender := readySendingFixture(t)
	input := preparedSendingInput()
	input.Scene = "test"
	if _, err := service.SendPreparedPhoneVerifyCode(context.Background(), input); appErrorCode(err) != apperror.CodeInvalidRequest || sender.calls != 0 {
		t.Fatalf("invalid scene error=%v sender=%d", err, sender.calls)
	}
}

func TestSendPreparedPhoneVerifyCodeMapsDuplicateChallengeToConflict(t *testing.T) {
	service, logs, verification, sender := readySendingFixture(t)
	logs.createErr = smslog.ErrChallengeActive
	_, err := service.SendPreparedPhoneVerifyCode(context.Background(), preparedSendingInput())
	if appErrorCode(err) != apperror.CodeConflict || verification.calls != 0 || sender.calls != 0 {
		t.Fatalf("error=%v verification=%d sender=%d", err, verification.calls, sender.calls)
	}
}

func TestSendPreparedPhoneVerifyCodePropagatesCancellation(t *testing.T) {
	service, _, _, sender := readySendingFixture(t)
	sender.err = context.Canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := service.SendPreparedPhoneVerifyCode(ctx, preparedSendingInput())
	if appErrorCode(err) != apperror.CodeDependencyUnavailable || sender.calls != 1 {
		t.Fatalf("error=%v sender=%d", err, sender.calls)
	}
}

func TestSendAdminTestDoesNotCreateVerificationFact(t *testing.T) {
	service, logs, verification, sender := readySendingFixture(t)
	result, err := service.SendAdminTest(context.Background(), AdminTestInput{PlatformID: 2, Scene: template.SceneLogin, ToPhone: "15671628271"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != smslog.StatusSent || logs.finished != smslog.StatusSent || verification.calls != 0 || sender.calls != 1 {
		t.Fatalf("result=%+v finished=%q verification=%d sender=%d", result, logs.finished, verification.calls, sender.calls)
	}
}

func TestSendPreparedPhoneVerifyCodeProviderFailureBestEffortClosesFailed(t *testing.T) {
	service, logs, _, sender := readySendingFixture(t)
	sender.err = NewProviderError("ProviderRejected", "request rejected")
	_, err := service.SendPreparedPhoneVerifyCode(context.Background(), preparedSendingInput())
	if appErrorCode(err) != apperror.CodeDependencyUnavailable || logs.finished != smslog.StatusFailed {
		t.Fatalf("error=%v finished=%q", err, logs.finished)
	}
}

func TestSendPreparedPhoneVerifyCodeFinishFailureLeavesPending(t *testing.T) {
	service, logs, _, _ := readySendingFixture(t)
	logs.finishErr = errors.New("finish unavailable")
	_, err := service.SendPreparedPhoneVerifyCode(context.Background(), preparedSendingInput())
	if appErrorCode(err) != apperror.CodeDependencyUnavailable || logs.created == nil || logs.created.Status != smslog.StatusPending {
		t.Fatalf("error=%v created=%+v", err, logs.created)
	}
}

func TestPreparePhoneVerifyCodeTreatsDisabledTemplateAsDependencyFailure(t *testing.T) {
	service, _, _, _ := readySendingFixture(t)
	facts := service.stores.Runtime.(runtimeStoreTest).facts
	fact := facts.Templates[template.SceneLogin]
	fact.IsEnabled = yesno.No
	facts.Templates[template.SceneLogin] = fact
	service.stores.Runtime = runtimeStoreTest{facts: facts}
	_, err := service.PreparePhoneVerifyCode(context.Background(), PhoneVerifyCodePrepareInput{PlatformID: 2, Scene: template.SceneLogin, ToPhone: "15671628271"})
	if appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("error=%v", err)
	}
}

func appErrorCode(err error) int {
	var appError *apperror.Error
	if errors.As(err, &appError) {
		return appError.Code
	}
	return 0
}

func TestPreparedPhoneCodeValidationRejectsUnsafePayloads(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	valid := PhoneVerifyCodeInput{
		ChallengeID: "challenge-1", Code: "123456",
		ExpiresAt:   now.Add(5 * time.Minute),
		Preparation: PhoneVerifyCodePreparation{TTLMinutes: 5},
	}
	mutations := []func(*PhoneVerifyCodeInput){
		func(value *PhoneVerifyCodeInput) { value.Code = "12345" },
		func(value *PhoneVerifyCodeInput) { value.Code = "12345a" },
		func(value *PhoneVerifyCodeInput) { value.Code = "１２３４５６" },
		func(value *PhoneVerifyCodeInput) { value.ChallengeID = "\tchallenge" },
		func(value *PhoneVerifyCodeInput) { value.ExpiresAt = now },
		func(value *PhoneVerifyCodeInput) { value.ExpiresAt = now.Add(5*time.Minute + time.Nanosecond) },
	}
	for index, mutate := range mutations {
		input := valid
		mutate(&input)
		if err := validatePreparedPhoneCodeInput(now, input); err == nil {
			t.Fatalf("mutation %d was accepted: %+v", index, input)
		}
	}
	if err := validatePreparedPhoneCodeInput(now, valid); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
}
