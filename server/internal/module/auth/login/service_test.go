package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/auth/client"
	"admin/server/internal/module/auth/platform"
	"admin/server/internal/module/auth/state"
	messagemail "admin/server/internal/module/message/mail"
	"admin/server/internal/module/permission/role"
	user "admin/server/internal/module/user/account"
	"admin/server/internal/module/user/loginlog"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestRegisterUsesPlatformPolicyAndRequiresRedis(t *testing.T) {
	redisClient := openAuthRedis(t)
	users := &fakeUserStore{createFn: func(_ context.Context, input user.CreateInput) (user.User, error) {
		return user.User{ID: 81001, Username: input.Username, Email: input.Email, IsEnabled: yesno.Yes}, nil
	}}
	service := newRedisTestService(t, redisClient, users, &fakeRoleStore{defaultRole: role.Role{ID: 3}}, &fakeSessionStore{}, &fakePolicyStore{policy: testPolicy()})
	registered, err := service.Register(context.Background(), RegisterInput{
		Username: "  张三_01  ", Email: " USER@Example.COM ", Password: "password123", ConfirmPassword: "password123", Client: testAuthClient(),
	})
	if err != nil || registered.UserID != 81001 || users.created.RoleID != 3 || users.created.PasswordHash == "" {
		t.Fatalf("Register() = %+v,%v input=%+v", registered, err, users.created)
	}

	disabled := testPolicy()
	disabled.AllowRegister = false
	service = newRedisTestService(t, redisClient, users, &fakeRoleStore{defaultRole: role.Role{ID: 3}}, &fakeSessionStore{}, &fakePolicyStore{policy: disabled})
	if _, err := service.Register(context.Background(), RegisterInput{
		Username: "valid_user", Email: "valid@example.com", Password: "password123", ConfirmPassword: "password123", Client: testAuthClient(),
	}); appErrorCode(err) != apperror.CodeForbidden {
		t.Fatalf("registration-disabled error = %v", err)
	}
}

func TestRegisterMapsValidationAndConflicts(t *testing.T) {
	redisClient := openAuthRedis(t)
	for _, input := range []RegisterInput{
		{Username: "ab", Email: "user@example.com", Password: "password", ConfirmPassword: "password", Client: testAuthClient()},
		{Username: "valid_user", Email: "not-an-email", Password: "password", ConfirmPassword: "password", Client: testAuthClient()},
		{Username: "valid_user", Email: "user@example.com", Password: "password", ConfirmPassword: "different", Client: testAuthClient()},
	} {
		service := newRedisTestService(t, redisClient, &fakeUserStore{}, &fakeRoleStore{}, &fakeSessionStore{}, &fakePolicyStore{policy: testPolicy()})
		if _, err := service.Register(context.Background(), input); appErrorCode(err) != apperror.CodeInvalidRequest {
			t.Errorf("Register(%+v) error = %v", input, err)
		}
	}
	for _, repositoryError := range []error{user.ErrUsernameConflict, user.ErrEmailConflict} {
		users := &fakeUserStore{createErr: repositoryError}
		service := newRedisTestService(t, redisClient, users, &fakeRoleStore{defaultRole: role.Role{ID: 1}}, &fakeSessionStore{}, &fakePolicyStore{policy: testPolicy()})
		_, err := service.Register(context.Background(), RegisterInput{
			Username: "valid_user", Email: "user@example.com", Password: "password", ConfirmPassword: "password", Client: testAuthClient(),
		})
		if appErrorCode(err) != apperror.CodeConflict {
			t.Errorf("repository error %v mapped to %v", repositoryError, err)
		}
	}
}

func TestLoginUsesPolicyTTLAndPublishesSessionSnapshot(t *testing.T) {
	redisClient := openAuthRedis(t)
	cleanupAuthRedisKeys(t, redisClient, 82001, "admin", 82002)
	passwordHash, err := HashPassword("password")
	if err != nil {
		t.Fatal(err)
	}
	fixedNow := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	policy := testPolicy()
	policy.AccessTTL = 23 * time.Minute
	policy.RefreshTTL = 5 * time.Minute
	sessions := &fakeSessionStore{createSession: Session{ID: 82002, UserID: 82001, Platform: "admin", DeviceID: testAuthClient().DeviceID, Version: 1, ClientIP: "127.0.0.1"}}
	users := &fakeUserStore{credential: user.Credential{ID: 82001, Username: "admin", Email: "admin@example.com", PasswordHash: passwordHash, IsEnabled: yesno.Yes}}
	service := newRedisTestService(t, redisClient, users, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now

	credential, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypePassword, LoginAccount: "admin@example.com", Password: "password", Client: testAuthClient()})
	if err != nil {
		t.Fatal(err)
	}
	if credential.ExpiresIn != int(policy.AccessTTL.Seconds()) || !credential.RefreshExpiresAt.Equal(fixedNow.Add(policy.RefreshTTL)) {
		t.Fatalf("credential = %+v", credential)
	}
	identity, err := service.jwt.Parse(credential.AccessToken)
	if err != nil || identity.SessionID != 82002 || identity.Platform != "admin" {
		t.Fatalf("access identity = %+v,%v", identity, err)
	}
	state, found, err := service.states.ReadSessions(context.Background(), "admin", 82001)
	if err != nil || !found || state.State != authstate.StateReady {
		t.Fatalf("sessions state = %+v,%v,%v", state, found, err)
	}
	snapshot, found, err := service.sessionCache.Read(context.Background(), "admin", 82002)
	if err != nil || !found || snapshot.SessionVersion != 1 || snapshot.SessionsGeneration != state.Generation {
		t.Fatalf("session snapshot = %+v,%v,%v", snapshot, found, err)
	}
	ttl, found, err := redisClient.TTL(context.Background(), SessionKey("admin", 82002))
	if err != nil || !found || ttl > policy.RefreshTTL || ttl < policy.RefreshTTL-30*time.Second {
		t.Fatalf("session snapshot TTL = %v,%v,%v", ttl, found, err)
	}
}

func TestLoginNormalizesEmailAndKeepsCredentialErrorsUniform(t *testing.T) {
	ctx := context.Background()
	redisClient := openAuthRedis(t)
	passwordHash, err := HashPassword("password")
	if err != nil {
		t.Fatal(err)
	}
	users := &fakeUserStore{credential: user.Credential{
		ID: 1, Username: "admin", Email: "admin@example.com", PasswordHash: passwordHash, IsEnabled: yesno.Yes,
	}}
	service := newRedisTestService(t, redisClient, users, &fakeRoleStore{}, &fakeSessionStore{}, &fakePolicyStore{policy: testPolicy()})
	_, wrongErr := service.Login(ctx, LoginInput{LoginType: authplatform.LoginTypePassword, LoginAccount: " Admin@Example.COM ", Password: "wrong", Client: testAuthClient()})
	if appErrorCode(wrongErr) != apperror.CodeUnauthorized {
		t.Fatalf("wrong password error = %v", wrongErr)
	}
	if users.credentialEmail != "admin@example.com" {
		t.Fatalf("credential email = %q", users.credentialEmail)
	}

	users.credentialErr = gorm.ErrRecordNotFound
	_, missingErr := service.Login(ctx, LoginInput{LoginType: authplatform.LoginTypePassword, LoginAccount: "missing@example.com", Password: "wrong", Client: testAuthClient()})
	var wrongPublic, missingPublic *apperror.Error
	if !errors.As(wrongErr, &wrongPublic) || !errors.As(missingErr, &missingPublic) ||
		wrongPublic.HTTPStatus != missingPublic.HTTPStatus || wrongPublic.Code != missingPublic.Code ||
		wrongPublic.MessageKey != missingPublic.MessageKey {
		t.Fatalf("public errors differ: wrong=%v missing=%v", wrongErr, missingErr)
	}
}

func TestLoginComparesPasswordWithValidBcryptHashWhenEmailDoesNotExist(t *testing.T) {
	redisClient := openAuthRedis(t)
	service := newRedisTestService(t, redisClient, &fakeUserStore{credentialErr: gorm.ErrRecordNotFound}, &fakeRoleStore{}, &fakeSessionStore{}, &fakePolicyStore{policy: testPolicy()})
	compareCalls := 0
	comparedPassword := ""
	comparedCost := 0
	var comparedCostErr error
	service.comparePassword = func(hash, password string) error {
		compareCalls++
		comparedPassword = password
		comparedCost, comparedCostErr = bcrypt.Cost([]byte(hash))
		return VerifyPassword(hash, password)
	}

	_, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypePassword, LoginAccount: "missing@example.com", Password: "  supplied password  ", Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeUnauthorized {
		t.Fatalf("missing email error = %v", err)
	}
	if compareCalls != 1 || comparedPassword != "  supplied password  " || comparedCostErr != nil || comparedCost != bcrypt.DefaultCost {
		t.Fatalf("password compare calls=%d password=%q cost=%d costErr=%v", compareCalls, comparedPassword, comparedCost, comparedCostErr)
	}
}

func TestLoginRecordsFailedLoginWithoutCreatingSession(t *testing.T) {
	redisClient := openAuthRedis(t)
	recorder := &recordingLoginLog{}
	service := newRedisTestService(t, redisClient, &fakeUserStore{credentialErr: gorm.ErrRecordNotFound}, &fakeRoleStore{}, &fakeSessionStore{}, &fakePolicyStore{policy: testPolicy()})
	service.SetLoginLogRecorder(recorder)
	_, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypePassword, LoginAccount: "missing@example.com", Password: "wrong", Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeUnauthorized || len(recorder.events) != 1 {
		t.Fatalf("login error=%v events=%+v", err, recorder.events)
	}
	event := recorder.events[0]
	if event.EventType != loginlog.EventLogin || event.IsSuccess != yesno.No || event.LoginType == nil || *event.LoginType != loginlog.LoginPassword || event.UserID != nil || event.SessionID != nil {
		t.Fatalf("failed login event=%+v", event)
	}
}

func TestLoginPreservesPasswordWhitespaceForBcrypt(t *testing.T) {
	redisClient := openAuthRedis(t)
	cleanupAuthRedisKeys(t, redisClient, 82501, "admin", 82502)
	password := "  password  "
	passwordHash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	users := &fakeUserStore{credential: user.Credential{
		ID: 82501, Username: "admin", Email: "admin@example.com", PasswordHash: passwordHash, IsEnabled: yesno.Yes,
	}}
	sessions := &fakeSessionStore{createSession: Session{
		ID: 82502, UserID: 82501, Platform: "admin", DeviceID: testAuthClient().DeviceID, Version: 1, ClientIP: "127.0.0.1",
	}}
	service := newRedisTestService(t, redisClient, users, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: testPolicy()})

	if _, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypePassword, LoginAccount: "admin@example.com", Password: strings.TrimSpace(password), Client: testAuthClient()}); appErrorCode(err) != apperror.CodeUnauthorized {
		t.Fatalf("trimmed password error = %v", err)
	}
	if _, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypePassword, LoginAccount: "admin@example.com", Password: password, Client: testAuthClient()}); err != nil {
		t.Fatalf("original password error = %v", err)
	}
}

func TestLoginRejectsInvalidEmailBeforeCredentialLookup(t *testing.T) {
	redisClient := openAuthRedis(t)
	users := &fakeUserStore{}
	service := newRedisTestService(t, redisClient, users, &fakeRoleStore{}, &fakeSessionStore{}, &fakePolicyStore{policy: testPolicy()})

	for _, email := range []string{"", "not-an-email", strings.Repeat("a", 243) + "@example.com"} {
		if _, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypePassword, LoginAccount: email, Password: "password", Client: testAuthClient()}); appErrorCode(err) != apperror.CodeInvalidRequest {
			t.Errorf("Login(%q) error = %v", email, err)
		}
	}
	if users.credentialEmail != "" {
		t.Fatalf("invalid email reached credential lookup: %q", users.credentialEmail)
	}
}

func TestLoginMapsDisabledAndRepositoryCredentialErrors(t *testing.T) {
	redisClient := openAuthRedis(t)
	passwordHash, err := HashPassword("password")
	if err != nil {
		t.Fatal(err)
	}
	users := &fakeUserStore{credential: user.Credential{ID: 1, Email: "admin@example.com", PasswordHash: passwordHash, IsEnabled: yesno.No}}
	service := newRedisTestService(t, redisClient, users, &fakeRoleStore{}, &fakeSessionStore{}, &fakePolicyStore{policy: testPolicy()})
	if _, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypePassword, LoginAccount: "admin@example.com", Password: "password", Client: testAuthClient()}); appErrorCode(err) != apperror.CodeForbidden {
		t.Fatalf("disabled user error = %v", err)
	}

	users.credentialErr = errors.New("postgres unavailable")
	if _, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypePassword, LoginAccount: "admin@example.com", Password: "password", Client: testAuthClient()}); appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("repository error = %v", err)
	}
}

func TestLoginRejectsPasswordWhenPlatformDoesNotEnableIt(t *testing.T) {
	policy := testPolicy()
	policy.LoginTypes = []authplatform.LoginType{authplatform.LoginTypeEmail}
	users := &fakeUserStore{credentialErr: gorm.ErrRecordNotFound}
	service := NewService(users, nil, nil, &fakePolicyStore{policy: policy}, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypePassword, LoginAccount: "admin@example.com", Password: "password", Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeForbidden {
		t.Fatalf("disabled password login error = %v", err)
	}
	if users.credentialEmail != "" {
		t.Fatalf("disabled password login queried user %q", users.credentialEmail)
	}
}

func TestLoginRejectsPhoneUntilSMSChannelExists(t *testing.T) {
	policy := testPolicy()
	policy.LoginTypes = []authplatform.LoginType{authplatform.LoginTypePhone}
	store := &fakeVerificationCodeStore{checkValid: true, consumeValid: true}
	users := &fakeUserStore{credentialErr: gorm.ErrRecordNotFound}
	service := NewService(users, nil, nil, &fakePolicyStore{policy: policy}, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.SetVerificationCodeStore(store)

	_, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypePhone, LoginAccount: "+8615671628271", Code: "123456", Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeForbidden || store.checkCalls != 0 || users.identityCalls != 0 {
		t.Fatalf("phone login error=%v checkCalls=%d identityCalls=%d", err, store.checkCalls, users.identityCalls)
	}
}

func TestLoginRejectsEmptyPasswordHashBeforeBcrypt(t *testing.T) {
	policy := testPolicy()
	users := &fakeUserStore{credential: user.Credential{ID: 1, Email: "admin@example.com", PasswordHash: "", IsEnabled: yesno.Yes}}
	service := NewService(users, nil, nil, &fakePolicyStore{policy: policy}, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	compareCalls := 0
	service.comparePassword = func(string, string) error {
		compareCalls++
		return nil
	}

	_, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypePassword, LoginAccount: "admin@example.com", Password: "password", Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeUnauthorized || compareCalls != 0 {
		t.Fatalf("empty password hash error=%v compareCalls=%d", err, compareCalls)
	}
}

func TestSendCodeRequiresConfiguredLoginType(t *testing.T) {
	policy := testPolicy()
	policy.LoginTypes = []authplatform.LoginType{authplatform.LoginTypePassword}
	store := &fakeVerificationCodeStore{acquired: true}
	sender := &fakeVerifyCodeSender{readiness: messagemail.VerifyCodeReadiness{Ready: true}, result: messagemail.EmailVerifyCodeResult{ExpiresAt: time.Now().Add(10 * time.Minute)}}
	service := NewService(nil, nil, nil, &fakePolicyStore{policy: policy}, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.SetVerificationCodeStore(store)
	service.SetVerifyCodeSender(sender)

	_, err := service.SendCode(context.Background(), SendCodeInput{Account: "user@example.com", LoginType: authplatform.LoginTypeEmail, Scene: messagemail.SceneLogin, Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeForbidden || sender.sendCalls != 0 || store.acquireCalls != 0 {
		t.Fatalf("unconfigured email send error=%v senderCalls=%d acquireCalls=%d", err, sender.sendCalls, store.acquireCalls)
	}
}

func TestSendCodeLoadsPolicyBeforeNormalizingEmail(t *testing.T) {
	policies := &fakePolicyStore{policy: testPolicy()}
	store := &fakeVerificationCodeStore{acquired: true}
	sender := &fakeVerifyCodeSender{preparation: messagemail.EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60}}
	service := NewService(nil, nil, nil, policies, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.SetVerificationCodeStore(store)
	service.SetVerifyCodeSender(sender)

	_, err := service.SendCode(context.Background(), SendCodeInput{
		Account: "not-an-email", LoginType: authplatform.LoginTypeEmail, Scene: messagemail.SceneLogin, Client: testAuthClient(),
	})
	if appErrorCode(err) != apperror.CodeInvalidRequest || policies.calls != 1 {
		t.Fatalf("invalid email error=%v policy calls=%d, want invalid request after one policy read", err, policies.calls)
	}
	if store.acquireCalls != 0 || sender.prepareCalls != 0 {
		t.Fatalf("invalid email reached delivery: acquire=%d prepare=%d", store.acquireCalls, sender.prepareCalls)
	}
}

func TestLoginCodeChecksBeforeUserLookupAndConsume(t *testing.T) {
	policy := testPolicy()
	store := &fakeVerificationCodeStore{checkValid: false, consumeValid: true}
	users := &fakeUserStore{}
	service := NewService(users, nil, nil, &fakePolicyStore{policy: policy}, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.SetVerificationCodeStore(store)

	_, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypeEmail, LoginAccount: "user@example.com", Code: "000000", Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeUnauthorized || store.checkCalls != 1 || store.consumeCalls != 0 || users.identityCalls != 0 {
		t.Fatalf("wrong code error=%v check=%d consume=%d identity=%d", err, store.checkCalls, store.consumeCalls, users.identityCalls)
	}
}

func TestLoginCodeRateLimitStopsBeforeUserLookupAndLoginLog(t *testing.T) {
	store := &fakeVerificationCodeStore{checkLimited: true}
	users := &fakeUserStore{}
	recorder := &recordingLoginLog{}
	service := NewService(users, nil, nil, &fakePolicyStore{policy: testPolicy()}, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.SetVerificationCodeStore(store)
	service.SetLoginLogRecorder(recorder)

	_, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypeEmail, LoginAccount: "user@example.com", Code: "000000", Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeRateLimited || users.identityCalls != 0 || len(recorder.events) != 0 {
		t.Fatalf("limited code error=%v identityCalls=%d loginEvents=%d", err, users.identityCalls, len(recorder.events))
	}
}

func TestLoginCodeDoesNotConsumeWhenRegistrationIsDisabled(t *testing.T) {
	policy := testPolicy()
	policy.AllowRegister = false
	store := &fakeVerificationCodeStore{checkValid: true, consumeValid: true}
	users := &fakeUserStore{credentialErr: gorm.ErrRecordNotFound}
	service := NewService(users, nil, nil, &fakePolicyStore{policy: policy}, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.SetVerificationCodeStore(store)

	_, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypeEmail, LoginAccount: "new@example.com", Code: "123456", Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeUnauthorized || store.consumeCalls != 0 {
		t.Fatalf("registration-disabled code login error=%v consumeCalls=%d", err, store.consumeCalls)
	}
}

func TestLoginCodeDoesNotConsumeForDisabledUser(t *testing.T) {
	policy := testPolicy()
	store := &fakeVerificationCodeStore{checkValid: true, consumeValid: true}
	users := &fakeUserStore{credential: user.Credential{ID: 1, Email: "user@example.com", IsEnabled: yesno.No}}
	service := NewService(users, nil, nil, &fakePolicyStore{policy: policy}, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.SetVerificationCodeStore(store)

	_, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypeEmail, LoginAccount: "user@example.com", Code: "123456", Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeForbidden || store.consumeCalls != 0 {
		t.Fatalf("disabled-user code login error=%v consumeCalls=%d", err, store.consumeCalls)
	}
}

func TestSendCodeFailsClosedWhenCleanupFails(t *testing.T) {
	store := &fakeVerificationCodeStore{acquired: true, deleteErr: errors.New("redis delete failed")}
	sender := &fakeVerifyCodeSender{readiness: messagemail.VerifyCodeReadiness{Ready: true}, preparation: messagemail.EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60}, sendErr: apperror.Conflict(i18n.KeyConflict, nil, errors.New("challenge conflict"))}
	service := NewService(nil, nil, nil, &fakePolicyStore{policy: testPolicy()}, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.SetVerificationCodeStore(store)
	service.SetVerifyCodeSender(sender)

	_, err := service.SendCode(context.Background(), SendCodeInput{Account: "user@example.com", LoginType: authplatform.LoginTypeEmail, Scene: messagemail.SceneLogin, Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeDependencyUnavailable || store.deleteCalls != 1 || store.releaseCalls != 1 {
		t.Fatalf("cleanup failure error=%v deleteCalls=%d releaseCalls=%d", err, store.deleteCalls, store.releaseCalls)
	}
}

func TestSendCodeCleansUpWithBoundedContextAfterRequestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store := &fakeVerificationCodeStore{acquired: true}
	sender := &fakeVerifyCodeSender{
		readiness:   messagemail.VerifyCodeReadiness{Ready: true},
		preparation: messagemail.EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60},
		sendFn: func(context.Context, messagemail.EmailVerifyCodeInput) (messagemail.EmailVerifyCodeResult, error) {
			cancel()
			return messagemail.EmailVerifyCodeResult{}, errors.New("mail provider failed after request cancellation")
		},
	}
	service := NewService(nil, nil, nil, &fakePolicyStore{policy: testPolicy()}, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.SetVerificationCodeStore(store)
	service.SetVerifyCodeSender(sender)

	_, err := service.SendCode(ctx, SendCodeInput{Account: "user@example.com", LoginType: authplatform.LoginTypeEmail, Scene: messagemail.SceneLogin, Client: testAuthClient()})
	if err == nil || store.deleteCalls != 1 || store.releaseCalls != 1 {
		t.Fatalf("send error=%v deleteCalls=%d releaseCalls=%d", err, store.deleteCalls, store.releaseCalls)
	}
	if store.deleteContextErr != nil || store.releaseContextErr != nil {
		t.Fatalf("cleanup reused canceled request context: delete=%v release=%v", store.deleteContextErr, store.releaseContextErr)
	}
}

func TestSendCodeReleasesDeliveryLeaseAfterSuccess(t *testing.T) {
	store := &fakeVerificationCodeStore{acquired: true}
	sender := &fakeVerifyCodeSender{readiness: messagemail.VerifyCodeReadiness{Ready: true}, preparation: messagemail.EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60}}
	service := NewService(nil, nil, nil, &fakePolicyStore{policy: testPolicy()}, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.SetVerificationCodeStore(store)
	service.SetVerifyCodeSender(sender)

	if _, err := service.SendCode(context.Background(), SendCodeInput{Account: "user@example.com", LoginType: authplatform.LoginTypeEmail, Scene: messagemail.SceneLogin, Client: testAuthClient()}); err != nil {
		t.Fatal(err)
	}
	if store.releaseCalls != 1 {
		t.Fatalf("successful send releaseCalls=%d", store.releaseCalls)
	}
}

func TestSendCodeUsesPreparationTTLAndResendWindow(t *testing.T) {
	fixedNow := time.Date(2026, time.September, 7, 10, 0, 0, 0, time.UTC)
	store := &fakeVerificationCodeStore{acquired: true}
	sender := &fakeVerifyCodeSender{readiness: messagemail.VerifyCodeReadiness{Ready: true}, preparation: messagemail.EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60}}
	service := NewService(nil, nil, nil, &fakePolicyStore{policy: testPolicy()}, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.SetVerificationCodeStore(store)
	service.SetVerifyCodeSender(sender)
	service.now = func() time.Time { return fixedNow }

	result, err := service.SendCode(context.Background(), SendCodeInput{Account: "user@example.com", LoginType: authplatform.LoginTypeEmail, Scene: messagemail.SceneLogin, Client: testAuthClient()})
	if err != nil {
		t.Fatal(err)
	}
	if !result.ExpiresAt.Equal(fixedNow.Add(5 * time.Minute)) {
		t.Fatalf("SendCode expiry = %v, want %v", result.ExpiresAt, fixedNow.Add(5*time.Minute))
	}
	if result.ResendAfterSeconds != 60 {
		t.Fatalf("SendCode resendAfterSeconds = %d, want 60", result.ResendAfterSeconds)
	}
	if sender.prepareCalls != 1 || sender.sendCalls != 1 {
		t.Fatalf("prepareCalls=%d sendCalls=%d, want 1 each", sender.prepareCalls, sender.sendCalls)
	}
	if store.putCalls != 1 || store.putTTL != 5*time.Minute {
		t.Fatalf("Put calls=%d TTL=%v, want one Put with 5m", store.putCalls, store.putTTL)
	}
	if !sender.sendInput.ExpiresAt.Equal(result.ExpiresAt) || sender.sendInput.Preparation != sender.preparation {
		t.Fatalf("prepared send input=%+v result=%+v preparation=%+v", sender.sendInput, result, sender.preparation)
	}
}

func TestSendCodeRejectsInvalidMailPreparationBeforePut(t *testing.T) {
	for _, preparation := range []messagemail.EmailVerifyCodePreparation{
		{TTLMinutes: 0, ResendAfterSeconds: 60},
		{TTLMinutes: 61, ResendAfterSeconds: 60},
		{TTLMinutes: 5, ResendAfterSeconds: 0},
		{TTLMinutes: 5, ResendAfterSeconds: 86401},
	} {
		store := &fakeVerificationCodeStore{acquired: true}
		sender := &fakeVerifyCodeSender{preparation: preparation}
		service := NewService(nil, nil, nil, &fakePolicyStore{policy: testPolicy()}, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
		service.SetVerificationCodeStore(store)
		service.SetVerifyCodeSender(sender)

		_, err := service.SendCode(context.Background(), SendCodeInput{Account: "user@example.com", LoginType: authplatform.LoginTypeEmail, Scene: messagemail.SceneLogin, Client: testAuthClient()})
		if appErrorCode(err) != apperror.CodeDependencyUnavailable {
			t.Fatalf("preparation %+v error=%v, want dependency unavailable", preparation, err)
		}
		if store.putCalls != 0 || store.releaseCalls != 1 || sender.sendCalls != 0 {
			t.Fatalf("preparation %+v put=%d release=%d send=%d", preparation, store.putCalls, store.releaseCalls, sender.sendCalls)
		}
	}
}

func TestSendCodePutFailureReleasesLeaseWithoutSending(t *testing.T) {
	store := &fakeVerificationCodeStore{acquired: true, putErr: errors.New("Redis Eval failed")}
	sender := &fakeVerifyCodeSender{preparation: messagemail.EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60}}
	service := NewService(nil, nil, nil, &fakePolicyStore{policy: testPolicy()}, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.SetVerificationCodeStore(store)
	service.SetVerifyCodeSender(sender)

	_, err := service.SendCode(context.Background(), SendCodeInput{Account: "user@example.com", LoginType: authplatform.LoginTypeEmail, Scene: messagemail.SceneLogin, Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeDependencyUnavailable || store.releaseCalls != 1 || store.deleteCalls != 0 || sender.sendCalls != 0 {
		t.Fatalf("Put failure error=%v release=%d delete=%d send=%d", err, store.releaseCalls, store.deleteCalls, sender.sendCalls)
	}
}

func TestSendCodeSuccessfulSendFailsClosedWhenLeaseReleaseFails(t *testing.T) {
	store := &fakeVerificationCodeStore{acquired: true, releaseErr: errors.New("Redis release failed")}
	sender := &fakeVerifyCodeSender{preparation: messagemail.EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60}}
	service := NewService(nil, nil, nil, &fakePolicyStore{policy: testPolicy()}, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.SetVerificationCodeStore(store)
	service.SetVerifyCodeSender(sender)

	_, err := service.SendCode(context.Background(), SendCodeInput{Account: "user@example.com", LoginType: authplatform.LoginTypeEmail, Scene: messagemail.SceneLogin, Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeDependencyUnavailable || sender.sendCalls != 1 || store.releaseCalls != 1 {
		t.Fatalf("release failure error=%v send=%d release=%d", err, sender.sendCalls, store.releaseCalls)
	}
}

func TestSendCodeGenerationFailureReportsLeaseCleanupFailure(t *testing.T) {
	store := &fakeVerificationCodeStore{acquired: true, releaseErr: errors.New("Redis release failed")}
	sender := &fakeVerifyCodeSender{preparation: messagemail.EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60}}
	service := NewService(nil, nil, nil, &fakePolicyStore{policy: testPolicy()}, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.SetVerificationCodeStore(store)
	service.SetVerifyCodeSender(sender)
	service.generateCode = func() (string, error) { return "", errors.New("random source unavailable") }

	_, err := service.SendCode(context.Background(), SendCodeInput{Account: "user@example.com", LoginType: authplatform.LoginTypeEmail, Scene: messagemail.SceneLogin, Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeDependencyUnavailable || store.releaseCalls != 1 || sender.sendCalls != 0 {
		t.Fatalf("generation failure error=%v release=%d send=%d", err, store.releaseCalls, sender.sendCalls)
	}
}

func TestSendCodeReturnsRateLimitedWhenPrepareIsRateLimited(t *testing.T) {
	store := &fakeVerificationCodeStore{acquired: true}
	sender := &fakeVerifyCodeSender{
		readiness:  messagemail.VerifyCodeReadiness{Ready: true},
		prepareErr: apperror.RateLimited(errors.New("too many requests")),
	}
	service := NewService(nil, nil, nil, &fakePolicyStore{policy: testPolicy()}, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.SetVerificationCodeStore(store)
	service.SetVerifyCodeSender(sender)

	_, err := service.SendCode(context.Background(), SendCodeInput{Account: "user@example.com", LoginType: authplatform.LoginTypeEmail, Scene: messagemail.SceneLogin, Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeRateLimited {
		t.Fatalf("prepare rate-limit error = %v", err)
	}
	if store.releaseCalls != 1 || store.deleteCalls != 0 {
		t.Fatalf("prepare failure releaseCalls=%d deleteCalls=%d", store.releaseCalls, store.deleteCalls)
	}
}

func TestLoginCodeRecordsEmailCredentialFailure(t *testing.T) {
	store := &fakeVerificationCodeStore{checkValid: false}
	recorder := &recordingLoginLog{}
	service := NewService(&fakeUserStore{}, nil, nil, &fakePolicyStore{policy: testPolicy()}, nil, nil, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.SetVerificationCodeStore(store)
	service.SetLoginLogRecorder(recorder)

	_, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypeEmail, LoginAccount: "user@example.com", Code: "000000", Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeUnauthorized {
		t.Fatalf("invalid code error = %v", err)
	}
	if len(recorder.events) != 1 || recorder.events[0].LoginType == nil || *recorder.events[0].LoginType != loginlog.LoginEmail || recorder.events[0].IsSuccess != yesno.No {
		t.Fatalf("invalid code login events = %+v", recorder.events)
	}
}

func TestAuthenticateUsesWarmRedisWithoutPostgreSQL(t *testing.T) {
	redisClient := openAuthRedis(t)
	cleanupAuthRedisKeys(t, redisClient, 83001, "admin", 83002)
	policy := testPolicy()
	sessions := &fakeSessionStore{}
	service := newRedisTestService(t, redisClient, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	fixedNow := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now
	installWarmSession(t, service, SessionAuthority{
		Session: Session{ID: 83002, UserID: 83001, Platform: "admin", DeviceID: testAuthClient().DeviceID, Version: 4, ClientIP: "127.0.0.1", RefreshExpiresAt: fixedNow.Add(time.Hour)},
		UserID:  83001, UserIsEnabled: yesno.Yes,
	}, policy)
	rawToken, _, err := service.jwt.Issue(TokenIdentity{UserID: 83001, SessionID: 83002, Platform: "admin", Version: 4}, policy.AccessTTL)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := service.Authenticate(context.Background(), rawToken, testAuthClient())
	if err != nil {
		t.Fatal(err)
	}
	if sessions.authorityCalls != 0 || identity.UserID != 83001 || identity.PlatformID != policy.ID || identity.Platform != policy.Code || identity.PolicyVersion != policy.PolicyVersion || identity.AccessCacheTTL != policy.AccessCacheTTL || identity.CacheResult != "hit" {
		t.Fatalf("Authenticate() = %+v calls=%d", identity, sessions.authorityCalls)
	}
}

func TestAuthenticateFallsBackToPostgreSQLAndRebuildsRedis(t *testing.T) {
	redisClient := openAuthRedis(t)
	cleanupAuthRedisKeys(t, redisClient, 84001, "admin", 84002)
	fixedNow := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	policy := testPolicy()
	authority := SessionAuthority{
		Session: Session{ID: 84002, UserID: 84001, Platform: "admin", DeviceID: testAuthClient().DeviceID, Version: 2, ClientIP: "127.0.0.1", RefreshExpiresAt: fixedNow.Add(time.Hour)},
		UserID:  84001, UserIsEnabled: yesno.Yes,
	}
	sessions := &fakeSessionStore{authority: authority}
	service := newRedisTestService(t, redisClient, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now
	rawToken, _, _ := service.jwt.Issue(TokenIdentity{UserID: 84001, SessionID: 84002, Platform: "admin", Version: 2}, policy.AccessTTL)

	first, err := service.Authenticate(context.Background(), rawToken, testAuthClient())
	if err != nil || first.PlatformID != policy.ID || first.Platform != policy.Code || first.CacheResult != "miss" || sessions.authorityCalls != 1 {
		t.Fatalf("fallback Authenticate() = %+v,%v calls=%d", first, err, sessions.authorityCalls)
	}
	second, err := service.Authenticate(context.Background(), rawToken, testAuthClient())
	if err != nil || second.PlatformID != policy.ID || second.Platform != policy.Code || second.CacheResult != "hit" || sessions.authorityCalls != 1 {
		t.Fatalf("warm Authenticate() = %+v,%v calls=%d", second, err, sessions.authorityCalls)
	}
}

func TestAuthenticateRepairsCorruptStateAfterPostgreSQLFallback(t *testing.T) {
	redisClient := openAuthRedis(t)
	cleanupAuthRedisKeys(t, redisClient, 84501, "admin", 84502)
	fixedNow := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	policy := testPolicy()
	authority := SessionAuthority{
		Session: Session{ID: 84502, UserID: 84501, Platform: "admin", DeviceID: testAuthClient().DeviceID, Version: 2, ClientIP: "127.0.0.1", RefreshExpiresAt: fixedNow.Add(time.Hour)},
		UserID:  84501, UserIsEnabled: yesno.Yes,
	}
	sessions := &fakeSessionStore{authority: authority}
	service := newRedisTestService(t, redisClient, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now
	if err := redisClient.SetString(context.Background(), authstate.UserStateKey(84501), `{"schemaVersion":1,"state":"ready","unknown":true}`, 0); err != nil {
		t.Fatal(err)
	}
	rawToken, _, _ := service.jwt.Issue(TokenIdentity{UserID: 84501, SessionID: 84502, Platform: "admin", Version: 2}, policy.AccessTTL)

	first, err := service.Authenticate(context.Background(), rawToken, testAuthClient())
	if err != nil || sessions.authorityCalls != 1 {
		t.Fatalf("corrupt fallback = %+v,%v calls=%d", first, err, sessions.authorityCalls)
	}
	second, err := service.Authenticate(context.Background(), rawToken, testAuthClient())
	if err != nil || second.CacheResult != "hit" || sessions.authorityCalls != 1 {
		t.Fatalf("repaired cache = %+v,%v calls=%d", second, err, sessions.authorityCalls)
	}
}

func TestAuthenticateUsesPostgreSQLAuthorityWhenRedisFails(t *testing.T) {
	redisClient := openAuthRedis(t)
	fixedNow := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	policy := testPolicy()
	authority := SessionAuthority{
		Session: Session{ID: 85002, UserID: 85001, Platform: "admin", DeviceID: testAuthClient().DeviceID, Version: 1, ClientIP: "127.0.0.1", RefreshExpiresAt: fixedNow.Add(time.Hour)},
		UserID:  85001, UserIsEnabled: yesno.Yes,
	}
	sessions := &fakeSessionStore{authority: authority}
	service := newRedisTestService(t, redisClient, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now
	rawToken, _, _ := service.jwt.Issue(TokenIdentity{UserID: 85001, SessionID: 85002, Platform: "admin", Version: 1}, policy.AccessTTL)
	if err := redisClient.Close(); err != nil {
		t.Fatal(err)
	}

	identity, err := service.Authenticate(context.Background(), rawToken, testAuthClient())
	if err != nil || identity.UserID != 85001 || identity.CacheResult != "error" || sessions.authorityCalls != 1 {
		t.Fatalf("Redis error fallback = %+v,%v calls=%d", identity, err, sessions.authorityCalls)
	}
	sessions.authorityErr = errors.New("postgres down")
	if _, err := service.Authenticate(context.Background(), rawToken, testAuthClient()); appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Redis and PostgreSQL error = %v", err)
	}
}

func TestAuthenticateRejectsInvalidatingStateWithoutPostgreSQL(t *testing.T) {
	redisClient := openAuthRedis(t)
	cleanupAuthRedisKeys(t, redisClient, 86001, "admin", 86002)
	policy := testPolicy()
	sessions := &fakeSessionStore{}
	service := newRedisTestService(t, redisClient, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	fixedNow := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now
	userFact := authstate.UserFact{UserID: 86001, Generation: "user-ready", IsEnabled: true}
	_, _, _ = service.states.InstallUserReadyIfMissing(context.Background(), userFact)
	lease, err := service.invalidator.Acquire(context.Background(), authstate.MutationFacts{Users: []authstate.UserFact{userFact}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lease.Rollback(context.Background()) })
	rawToken, _, _ := service.jwt.Issue(TokenIdentity{UserID: 86001, SessionID: 86002, Platform: "admin", Version: 1}, policy.AccessTTL)

	_, err = service.Authenticate(context.Background(), rawToken, testAuthClient())
	if appErrorCode(err) != authplatform.CodeSessionUpdating || sessions.authorityCalls != 0 {
		t.Fatalf("invalidating Authenticate() error=%v calls=%d", err, sessions.authorityCalls)
	}
}

func TestAuthenticateRevokesDeviceMismatchOnlyAfterRedisInvalidation(t *testing.T) {
	redisClient := openAuthRedis(t)
	cleanupAuthRedisKeys(t, redisClient, 86501, "admin", 86502)
	policy := testPolicy()
	policy.BindDevice = true
	fixedNow := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	authority := SessionAuthority{
		Session: Session{ID: 86502, UserID: 86501, Platform: "admin", DeviceID: testAuthClient().DeviceID, Version: 1, ClientIP: "127.0.0.1", RefreshExpiresAt: fixedNow.Add(time.Hour)},
		UserID:  86501, UserIsEnabled: yesno.Yes,
	}
	sessions := &fakeSessionStore{authority: authority}
	service := newRedisTestService(t, redisClient, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now
	installWarmSession(t, service, authority, policy)
	rawToken, _, _ := service.jwt.Issue(TokenIdentity{UserID: 86501, SessionID: 86502, Platform: "admin", Version: 1}, policy.AccessTTL)
	mismatched := testAuthClient()
	mismatched.DeviceID = "650e8400-e29b-41d4-a716-446655440000"

	if _, err := service.Authenticate(context.Background(), rawToken, mismatched); appErrorCode(err) != apperror.CodeUnauthorized {
		t.Fatalf("device mismatch error = %v", err)
	}
	if sessions.revokeCalls != 1 {
		t.Fatalf("device mismatch revoke calls = %d", sessions.revokeCalls)
	}
	if _, found, err := service.sessionCache.Read(context.Background(), "admin", 86502); err != nil || found {
		t.Fatalf("revoked snapshot still available = %v,%v", found, err)
	}

	closedRedis := openAuthRedis(t)
	closedSessions := &fakeSessionStore{authority: authority}
	closedService := newRedisTestService(t, closedRedis, &fakeUserStore{}, &fakeRoleStore{}, closedSessions, &fakePolicyStore{policy: policy})
	closedService.now = func() time.Time { return fixedNow }
	closedService.jwt.now = closedService.now
	if err := closedRedis.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := closedService.Authenticate(context.Background(), rawToken, mismatched); appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Redis failure device mismatch error = %v", err)
	}
	if closedSessions.revokeCalls != 0 {
		t.Fatal("device mismatch revoked PostgreSQL after Redis coordination failure")
	}
}

func TestRefreshRotatesWithinSessionInvalidationAndKeepsAbsoluteExpiry(t *testing.T) {
	redisClient := openAuthRedis(t)
	cleanupAuthRedisKeys(t, redisClient, 86801, "admin", 86802)
	policy := testPolicy()
	policy.AccessTTL = 7 * time.Minute
	fixedNow := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	expiresAt := fixedNow.Add(2 * time.Hour)
	authority := SessionAuthority{
		Session: Session{ID: 86802, UserID: 86801, Platform: "admin", DeviceID: testAuthClient().DeviceID, RefreshTokenHash: strings.Repeat("a", 64), Version: 3, ClientIP: "127.0.0.1", RefreshExpiresAt: expiresAt},
		UserID:  86801, UserIsEnabled: yesno.Yes,
	}
	rotated := authority.Session
	rotated.Version++
	sessions := &fakeSessionStore{refresh: authority, rotateSession: rotated, rotateWon: true}
	service := newRedisTestService(t, redisClient, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	recorder := &recordingLoginLog{}
	service.SetLoginLogRecorder(recorder)
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now
	_, _, _ = service.states.InstallUserReadyIfMissing(context.Background(), authstate.UserFact{UserID: 86801, Generation: "user-ready", IsEnabled: true})
	_, _, _ = service.states.InstallSessionsReadyIfMissing(context.Background(), authstate.SessionsFact{Platform: "admin", UserID: 86801, Generation: "sessions-ready"})

	credential, err := service.Refresh(context.Background(), RefreshInput{RefreshToken: "old-refresh", Client: testAuthClient()})
	if err != nil {
		t.Fatal(err)
	}
	if len(recorder.events) != 0 {
		t.Fatalf("refresh produced login events: %+v", recorder.events)
	}
	if sessions.rotateCalls != 1 || credential.ExpiresIn != int(policy.AccessTTL.Seconds()) || !credential.RefreshExpiresAt.Equal(expiresAt) {
		t.Fatalf("Refresh() = %+v rotateCalls=%d", credential, sessions.rotateCalls)
	}
	identity, err := service.jwt.Parse(credential.AccessToken)
	if err != nil || identity.Version != rotated.Version || identity.SessionID != rotated.ID {
		t.Fatalf("rotated identity = %+v,%v", identity, err)
	}
}

func TestLogoutRequiresRedisInvalidationBeforePostgreSQL(t *testing.T) {
	redisClient := openAuthRedis(t)
	cleanupAuthRedisKeys(t, redisClient, 87001, "admin", 87002)
	policy := testPolicy()
	sessions := &fakeSessionStore{}
	service := newRedisTestService(t, redisClient, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	_, _, _ = service.states.InstallSessionsReadyIfMissing(context.Background(), authstate.SessionsFact{Platform: "admin", UserID: 87001, Generation: "sessions-ready"})
	if err := service.Logout(context.Background(), Identity{UserID: 87001, SessionID: 87002, Platform: "admin", Version: 1}, testAuthClient()); err != nil {
		t.Fatal(err)
	}
	if sessions.revokeCalls != 1 {
		t.Fatalf("revoke calls = %d", sessions.revokeCalls)
	}

	closedRedis := openAuthRedis(t)
	closedService := newRedisTestService(t, closedRedis, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	if err := closedRedis.Close(); err != nil {
		t.Fatal(err)
	}
	if err := closedService.Logout(context.Background(), Identity{UserID: 87001, SessionID: 87003, Platform: "admin", Version: 1}, testAuthClient()); appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Redis failure Logout() = %v", err)
	}
	if sessions.revokeCalls != 1 {
		t.Fatal("PostgreSQL revoke ran after Redis coordination failure")
	}
}

func TestCurrentUserReturnsClosedIdentity(t *testing.T) {
	redisClient := openAuthRedis(t)
	phone := "+86 138-0000-0000"
	users := &fakeUserStore{current: user.Current{ID: 1, Username: "admin", Email: "admin@example.com", Phone: &phone}}
	service := newRedisTestService(t, redisClient, users, &fakeRoleStore{}, &fakeSessionStore{}, &fakePolicyStore{policy: testPolicy()})
	current, err := service.CurrentUser(context.Background(), Identity{UserID: 1, SessionID: 2, Platform: "admin", Version: 1})
	if err != nil || current != users.current {
		t.Fatalf("CurrentUser() = %+v,%v", current, err)
	}
}

func newRedisTestService(t *testing.T, redisClient *projectredis.Client, users userStore, roles roleStore, sessions sessionStore, policies policyStore) *Service {
	t.Helper()
	states := authstate.NewStore(redisClient)
	return NewService(
		users, roles, sessions, policies, states, authstate.NewInvalidator(states), NewSessionCache(redisClient), redisClient,
		NewJWT([]byte(strings.Repeat("j", 32))), []byte(strings.Repeat("h", 32)), slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
}

func testPolicy() authplatform.Policy {
	return authplatform.Policy{
		ID: 1, Code: "admin", Name: "Admin", LoginTypes: []authplatform.LoginType{authplatform.LoginTypeEmail, authplatform.LoginTypePassword}, PolicyVersion: 1, AccessTTL: 15 * time.Minute,
		RefreshTTL: 14 * 24 * time.Hour, SessionCacheTTL: 30 * time.Minute, AccessCacheTTL: 30 * time.Minute,
		MaxSessions: 1, AllowRegister: true, IsEnabled: true, IsBuiltin: true,
	}
}

func installWarmSession(t *testing.T, service *Service, authority SessionAuthority, policy authplatform.Policy) {
	t.Helper()
	userGeneration, _ := authstate.NewGeneration()
	sessionsGeneration, _ := authstate.NewGeneration()
	userFact := authstate.UserFact{UserID: authority.UserID, Generation: userGeneration, IsEnabled: authority.UserIsEnabled == yesno.Yes, Deleted: authority.UserDeleted}
	sessionsFact := authstate.SessionsFact{Platform: authority.Session.Platform, UserID: authority.UserID, Generation: sessionsGeneration}
	if _, _, err := service.states.InstallUserReadyIfMissing(context.Background(), userFact); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.states.InstallSessionsReadyIfMissing(context.Background(), sessionsFact); err != nil {
		t.Fatal(err)
	}
	snapshot := snapshotFromAuthority(authority, policy, userFact.Generation, sessionsFact.Generation)
	if published, err := service.sessionCache.PublishIfCurrent(context.Background(), snapshot, policy.SessionCacheTTL); err != nil || !published {
		t.Fatalf("publish warm session = %v,%v", published, err)
	}
}

func cleanupAuthRedisKeys(t *testing.T, client *projectredis.Client, userID int64, platform string, sessionID int64) {
	t.Helper()
	keys := []string{authstate.UserStateKey(userID), authstate.SessionsStateKey(platform, userID), SessionKey(platform, sessionID)}
	if err := client.DeleteMany(context.Background(), keys); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), keys) })
}

func appErrorCode(err error) int {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return 0
}

type fakePolicyStore struct {
	policy authplatform.Policy
	err    error
	calls  int
}

type recordingLoginLog struct{ events []loginlog.Event }

func (r *recordingLoginLog) Record(_ context.Context, event loginlog.Event) error {
	r.events = append(r.events, event)
	return nil
}

type failingLoginLog struct{}

func (failingLoginLog) Record(context.Context, loginlog.Event) error {
	return errors.New("login log database is down")
}

func TestRecordLoginEventIsBestEffort(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)
	service.SetLoginLogRecorder(failingLoginLog{})

	if err := service.recordLoginEvent(context.Background(), loginlog.Event{EventType: loginlog.EventLogin}); err != nil {
		t.Fatalf("recordLoginEvent returned an error on login-log outage: %v", err)
	}
}

func (f *fakePolicyStore) CurrentPolicy(context.Context, string) (authplatform.Policy, error) {
	f.calls++
	return f.policy, f.err
}

type fakeUserStore struct {
	created         user.CreateInput
	createFn        func(context.Context, user.CreateInput) (user.User, error)
	createErr       error
	credential      user.Credential
	credentialErr   error
	credentialEmail string
	current         user.Current
	currentErr      error
	identityCalls   int
}

func (f *fakeUserStore) CreateWithRole(ctx context.Context, input user.CreateInput) (user.User, error) {
	f.created = input
	if f.createFn != nil {
		return f.createFn(ctx, input)
	}
	return user.User{}, f.createErr
}

func (f *fakeUserStore) FindCredentialByEmail(_ context.Context, email string) (user.Credential, error) {
	f.credentialEmail = email
	return f.credential, f.credentialErr
}

func (f *fakeUserStore) FindCredentialByIdentity(context.Context, string, string) (user.Credential, error) {
	f.identityCalls++
	return f.credential, f.credentialErr
}

func (f *fakeUserStore) CreateVerifiedIdentity(context.Context, user.VerifiedIdentityInput) (user.User, error) {
	return user.User{}, f.createErr
}

func (f *fakeUserStore) FindCurrent(context.Context, int64) (user.Current, error) {
	return f.current, f.currentErr
}

type fakeVerifyCodeSender struct {
	readiness    messagemail.VerifyCodeReadiness
	readyErr     error
	preparation  messagemail.EmailVerifyCodePreparation
	prepareErr   error
	result       messagemail.EmailVerifyCodeResult
	sendErr      error
	sendFn       func(context.Context, messagemail.EmailVerifyCodeInput) (messagemail.EmailVerifyCodeResult, error)
	sendCalls    int
	prepareCalls int
	prepareInput messagemail.EmailVerifyCodePrepareInput
	sendInput    messagemail.EmailVerifyCodeInput
}

func (f *fakeVerifyCodeSender) VerifyCodeReady(context.Context, int64, string) (messagemail.VerifyCodeReadiness, error) {
	return f.readiness, f.readyErr
}

func (f *fakeVerifyCodeSender) PrepareEmailVerifyCode(_ context.Context, input messagemail.EmailVerifyCodePrepareInput) (messagemail.EmailVerifyCodePreparation, error) {
	f.prepareCalls++
	f.prepareInput = input
	return f.preparation, f.prepareErr
}

func (f *fakeVerifyCodeSender) SendPreparedEmailVerifyCode(ctx context.Context, input messagemail.EmailVerifyCodeInput) (messagemail.EmailVerifyCodeResult, error) {
	f.sendCalls++
	f.sendInput = input
	if f.sendFn != nil {
		return f.sendFn(ctx, input)
	}
	return f.result, f.sendErr
}

type fakeVerificationCodeStore struct {
	acquired          bool
	acquireErr        error
	putErr            error
	checkValid        bool
	checkLimited      bool
	checkErr          error
	consumeValid      bool
	consumeErr        error
	deleteErr         error
	releaseErr        error
	acquireCalls      int
	checkCalls        int
	consumeCalls      int
	deleteCalls       int
	releaseCalls      int
	putCalls          int
	putTTL            time.Duration
	deleteContextErr  error
	releaseContextErr error
}

func (*fakeVerificationCodeStore) VerificationKey(string, string, string, string) string {
	return "verification-key"
}
func (*fakeVerificationCodeStore) Digest(string) string { return "digest" }
func (f *fakeVerificationCodeStore) AcquireDelivery(context.Context, string, string, time.Duration) (bool, error) {
	f.acquireCalls++
	return f.acquired, f.acquireErr
}
func (f *fakeVerificationCodeStore) Put(_ context.Context, _, _, _ string, ttl time.Duration) error {
	f.putCalls++
	f.putTTL = ttl
	return f.putErr
}
func (f *fakeVerificationCodeStore) Check(context.Context, string, string) (bool, error) {
	f.checkCalls++
	return f.checkValid, f.checkErr
}
func (f *fakeVerificationCodeStore) CheckAttempt(context.Context, string, string, string) (bool, bool, error) {
	f.checkCalls++
	return f.checkValid, f.checkLimited, f.checkErr
}
func (f *fakeVerificationCodeStore) Consume(context.Context, string, string) (bool, error) {
	f.consumeCalls++
	return f.consumeValid, f.consumeErr
}
func (f *fakeVerificationCodeStore) DeleteIfOwned(ctx context.Context, _ string, _ string) error {
	f.deleteCalls++
	f.deleteContextErr = ctx.Err()
	return f.deleteErr
}
func (f *fakeVerificationCodeStore) ReleaseDelivery(ctx context.Context, _ string, _ string) error {
	f.releaseCalls++
	f.releaseContextErr = ctx.Err()
	return f.releaseErr
}

type fakeRoleStore struct {
	defaultRole role.Role
	err         error
}

func (f *fakeRoleStore) FindDefault(context.Context) (role.Role, error) {
	return f.defaultRole, f.err
}

type fakeSessionStore struct {
	createSession  Session
	createRevoked  []Session
	createErr      error
	createCalls    int
	authority      SessionAuthority
	authorityErr   error
	authorityCalls int
	refresh        SessionAuthority
	refreshErr     error
	rotateSession  Session
	rotateWon      bool
	rotateErr      error
	rotateCalls    int
	revokeCalls    int
	revokeErr      error
}

func (f *fakeSessionStore) CreateWithinLimit(context.Context, SessionCreate, authplatform.Policy, time.Time) (Session, []Session, error) {
	f.createCalls++
	return f.createSession, f.createRevoked, f.createErr
}

func (f *fakeSessionStore) FindAuthoritative(context.Context, int64, int64, string, int64, time.Time) (SessionAuthority, error) {
	f.authorityCalls++
	return f.authority, f.authorityErr
}

func (f *fakeSessionStore) FindByRefreshHash(context.Context, string, string, time.Time) (SessionAuthority, error) {
	return f.refresh, f.refreshErr
}

func (f *fakeSessionStore) RotateByRefreshHash(context.Context, int64, string, string, string, time.Time, authclient.Client) (Session, bool, error) {
	f.rotateCalls++
	return f.rotateSession, f.rotateWon, f.rotateErr
}

func (f *fakeSessionStore) Revoke(context.Context, int64, time.Time) error {
	f.revokeCalls++
	return f.revokeErr
}

func testAuthClient() authclient.Client {
	return authclient.Client{Platform: "admin", DeviceID: "550e8400-e29b-41d4-a716-446655440000", ClientIP: "127.0.0.1", UserAgent: "test-agent"}
}

var _ = gorm.ErrRecordNotFound
var _ = i18n.KeyUnauthorized
