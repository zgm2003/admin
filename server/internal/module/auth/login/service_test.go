package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/auth/client"
	"admin/server/internal/module/auth/state"
	messagemail "admin/server/internal/module/message/mail"
	messagesms "admin/server/internal/module/message/sms"
	smstemplate "admin/server/internal/module/message/sms/template"
	"admin/server/internal/module/permission/authPlatform"
	"admin/server/internal/module/permission/role"
	user "admin/server/internal/module/user/account"
	"admin/server/internal/module/user/loginLog"
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

func TestLoginPhoneUsesVerifiedNormalizedIdentity(t *testing.T) {
	redisClient := openAuthRedis(t)
	policy := testPolicy()
	policy.LoginTypes = []authplatform.LoginType{authplatform.LoginTypePhone, authplatform.LoginTypePassword}
	store := &fakeVerificationCodeStore{checkValid: true, consumeValid: true}
	users := &fakeUserStore{credential: user.Credential{ID: 82601, Username: "phone-user", IsEnabled: yesno.Yes}}
	sessions := &fakeSessionStore{createSession: Session{ID: 82602, UserID: 82601, Platform: "admin", DeviceID: testAuthClient().DeviceID, Version: 1, ClientIP: "127.0.0.1"}}
	cleanupAuthRedisKeys(t, redisClient, 82601, "admin", 82602)
	service := newRedisTestService(t, redisClient, users, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	service.SetVerificationCodeStore(store)

	credential, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypePhone, LoginAccount: "86 156-7162-8271", ChallengeID: "challenge-1", Code: "123456", Client: testAuthClient()})
	if err != nil || credential.AccessToken == "" || store.checkCalls != 1 || store.consumeCalls != 1 || users.identityCalls != 1 {
		t.Fatalf("credential=%+v error=%v check=%d consume=%d identity=%d", credential, err, store.checkCalls, store.consumeCalls, users.identityCalls)
	}
	if users.identityKind != "phone" || users.identityAccount != "+8615671628271" {
		t.Fatalf("identity kind=%q account=%q", users.identityKind, users.identityAccount)
	}
	if store.proofChallengeID != "challenge-1" || store.proofCode != "123456" {
		t.Fatalf("proof challenge=%q code=%q", store.proofChallengeID, store.proofCode)
	}
}

func TestLoginConfigPublishesPhoneOnlyWhenSMSIsReady(t *testing.T) {
	policy := testPolicy()
	policy.LoginTypes = []authplatform.LoginType{authplatform.LoginTypeEmail, authplatform.LoginTypePhone, authplatform.LoginTypePassword}
	emailSender := &fakeVerifyCodeSender{readiness: messagemail.VerifyCodeReadiness{Ready: true, TTLMinutes: 5}}
	phoneSender := &fakePhoneVerifyCodeSender{readiness: messagesms.VerifyCodeReadiness{Ready: true, TTLMinutes: 5}}
	service := NewService(nil, nil, nil, &fakePolicyStore{policy: policy}, nil, nil, nil, nil, nil, nil, slog.New(slog.DiscardHandler))
	service.SetVerifyCodeSender(emailSender)
	service.SetPhoneVerifyCodeSender(phoneSender)

	config, err := service.LoginConfig(context.Background(), testAuthClient())
	if err != nil {
		t.Fatal(err)
	}
	values := make([]authplatform.LoginType, 0, len(config.LoginTypes))
	for _, option := range config.LoginTypes {
		values = append(values, option.Value)
	}
	if fmt.Sprint(values) != "[email phone password]" || phoneSender.readyCalls != 1 {
		t.Fatalf("login types=%v phone readiness calls=%d", values, phoneSender.readyCalls)
	}

	phoneSender.readyErr = errors.New("sms readiness corrupt")
	config, err = service.LoginConfig(context.Background(), testAuthClient())
	if err != nil {
		t.Fatal(err)
	}
	values = values[:0]
	for _, option := range config.LoginTypes {
		values = append(values, option.Value)
	}
	if fmt.Sprint(values) != "[email password]" {
		t.Fatalf("login types after SMS failure=%v", values)
	}

	policy.LoginTypes = []authplatform.LoginType{authplatform.LoginTypeEmail, authplatform.LoginTypePassword}
	phoneSender.readyCalls = 0
	service.policies = &fakePolicyStore{policy: policy}
	if _, err := service.LoginConfig(context.Background(), testAuthClient()); err != nil {
		t.Fatal(err)
	}
	if phoneSender.readyCalls != 0 {
		t.Fatalf("SMS readiness probed despite phone-disabled policy: %d", phoneSender.readyCalls)
	}
}

func TestSendCodeRoutesPhoneToSMSLoginScene(t *testing.T) {
	policy := testPolicy()
	policy.LoginTypes = []authplatform.LoginType{authplatform.LoginTypePhone, authplatform.LoginTypePassword}
	store := &fakeVerificationCodeStore{acquired: true}
	sender := &fakePhoneVerifyCodeSender{preparation: messagesms.PhoneVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60}}
	service := NewService(nil, nil, nil, &fakePolicyStore{policy: policy}, nil, nil, nil, nil, nil, nil, slog.New(slog.DiscardHandler))
	service.SetVerificationCodeStore(store)
	service.SetPhoneVerifyCodeSender(sender)
	service.generateCode = func() (string, error) { return "123456", nil }

	result, err := service.SendCode(context.Background(), SendCodeInput{Account: "86 156-7162-8271", LoginType: authplatform.LoginTypePhone, Scene: smstemplate.SceneLogin, Client: testAuthClient()})
	if err != nil {
		t.Fatal(err)
	}
	if sender.prepareCalls != 1 || sender.sendCalls != 1 || sender.prepareInput.Scene != smstemplate.SceneLogin || sender.prepareInput.ToPhone != "+8615671628271" || sender.sendInput.Code != "123456" {
		t.Fatalf("prepare=%+v send=%+v calls=%d/%d", sender.prepareInput, sender.sendInput, sender.prepareCalls, sender.sendCalls)
	}
	if store.keyLoginType != "phone" || store.keyAccount != "+8615671628271" || result.ResendAfterSeconds != 60 {
		t.Fatalf("key type=%q account=%q result=%+v", store.keyLoginType, store.keyAccount, result)
	}
}

func TestLoginPhoneAutoRegistersVerifiedIdentity(t *testing.T) {
	redisClient := openAuthRedis(t)
	policy := testPolicy()
	policy.LoginTypes = []authplatform.LoginType{authplatform.LoginTypePhone, authplatform.LoginTypePassword}
	store := &fakeVerificationCodeStore{checkValid: true, consumeValid: true}
	users := &fakeUserStore{
		credentialErr: gorm.ErrRecordNotFound,
		verifiedUser:  user.User{ID: 82701, Username: "phone_82701", Phone: pointerString("+8615671628271"), IsEnabled: yesno.Yes},
	}
	sessions := &fakeSessionStore{createSession: Session{ID: 82702, UserID: 82701, Platform: "admin", DeviceID: testAuthClient().DeviceID, Version: 1, ClientIP: "127.0.0.1"}}
	cleanupAuthRedisKeys(t, redisClient, 82701, "admin", 82702)
	service := newRedisTestService(t, redisClient, users, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	service.SetVerificationCodeStore(store)

	credential, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypePhone, LoginAccount: "15671628271", ChallengeID: "challenge-1", Code: "123456", Client: testAuthClient()})
	if err != nil {
		t.Fatal(err)
	}
	if !credential.IsNewUser || credential.AccessToken == "" || users.verifiedInput.IdentityKind != "phone" || users.verifiedInput.Account != "+8615671628271" || users.verifiedInput.PasswordHash != "" {
		t.Fatalf("credential=%+v verified input=%+v", credential, users.verifiedInput)
	}
}

func TestLoginPhoneRegistrationPolicyAndConflictWinner(t *testing.T) {
	redisClient := openAuthRedis(t)
	policy := testPolicy()
	policy.LoginTypes = []authplatform.LoginType{authplatform.LoginTypePhone, authplatform.LoginTypePassword}
	policy.AllowRegister = false
	store := &fakeVerificationCodeStore{checkValid: true, consumeValid: true}
	users := &fakeUserStore{credentialErr: gorm.ErrRecordNotFound}
	service := newRedisTestService(t, redisClient, users, &fakeRoleStore{}, &fakeSessionStore{}, &fakePolicyStore{policy: policy})
	service.SetVerificationCodeStore(store)
	_, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypePhone, LoginAccount: "15671628271", ChallengeID: "challenge-1", Code: "123456", Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeUnauthorized || store.consumeCalls != 0 || users.verifiedInput.IdentityKind != "" {
		t.Fatalf("closed registration error=%v consume=%d verified=%+v", err, store.consumeCalls, users.verifiedInput)
	}

	policy.AllowRegister = true
	users = &fakeUserStore{credentialErr: gorm.ErrRecordNotFound, createErr: user.ErrPhoneConflict}
	winner := user.Credential{ID: 82801, Username: "winner", IsEnabled: yesno.Yes}
	users.verifiedFn = func(context.Context, user.VerifiedIdentityInput) (user.User, error) {
		users.credential = winner
		users.credentialErr = nil
		return user.User{}, user.ErrPhoneConflict
	}
	sessions := &fakeSessionStore{createSession: Session{ID: 82802, UserID: 82801, Platform: "admin", DeviceID: testAuthClient().DeviceID, Version: 1, ClientIP: "127.0.0.1"}}
	cleanupAuthRedisKeys(t, redisClient, 82801, "admin", 82802)
	service = newRedisTestService(t, redisClient, users, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	service.SetVerificationCodeStore(&fakeVerificationCodeStore{checkValid: true, consumeValid: true})
	credential, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypePhone, LoginAccount: "15671628271", ChallengeID: "challenge-1", Code: "123456", Client: testAuthClient()})
	if err != nil || credential.AccessToken == "" || credential.IsNewUser || users.identityCalls != 2 {
		t.Fatalf("winner credential=%+v error=%v identity calls=%d", credential, err, users.identityCalls)
	}
}

func TestSendCodePhoneFailureReleasesOwnedVerification(t *testing.T) {
	policy := testPolicy()
	policy.LoginTypes = []authplatform.LoginType{authplatform.LoginTypePhone, authplatform.LoginTypePassword}
	store := &fakeVerificationCodeStore{acquired: true}
	sender := &fakePhoneVerifyCodeSender{
		preparation: messagesms.PhoneVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60},
		sendErr:     apperror.DependencyUnavailable(errors.New("sms provider unavailable")),
	}
	service := NewService(nil, nil, nil, &fakePolicyStore{policy: policy}, nil, nil, nil, nil, nil, nil, slog.New(slog.DiscardHandler))
	service.SetVerificationCodeStore(store)
	service.SetPhoneVerifyCodeSender(sender)
	_, err := service.SendCode(context.Background(), SendCodeInput{Account: "15671628271", LoginType: authplatform.LoginTypePhone, Scene: smstemplate.SceneLogin, Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeDependencyUnavailable || sender.sendCalls != 1 || store.deleteCalls != 1 || store.releaseCalls != 1 {
		t.Fatalf("error=%v send=%d delete=%d release=%d", err, sender.sendCalls, store.deleteCalls, store.releaseCalls)
	}
}

func pointerString(value string) *string { return &value }

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

	_, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypeEmail, LoginAccount: "user@example.com", ChallengeID: "challenge-1", Code: "000000", Client: testAuthClient()})
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

	_, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypeEmail, LoginAccount: "user@example.com", ChallengeID: "challenge-1", Code: "000000", Client: testAuthClient()})
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

	_, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypeEmail, LoginAccount: "new@example.com", ChallengeID: "challenge-1", Code: "123456", Client: testAuthClient()})
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

	_, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypeEmail, LoginAccount: "user@example.com", ChallengeID: "challenge-1", Code: "123456", Client: testAuthClient()})
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
		{TTLMinutes: 5, ResendAfterSeconds: -1},
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

	_, err := service.Login(context.Background(), LoginInput{LoginType: authplatform.LoginTypeEmail, LoginAccount: "user@example.com", ChallengeID: "challenge-1", Code: "000000", Client: testAuthClient()})
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

func TestAuthenticateRejectsCorruptStateWithoutPostgreSQL(t *testing.T) {
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
	if appErrorCode(err) != apperror.CodeDependencyUnavailable || sessions.authorityCalls != 0 {
		t.Fatalf("corrupt fallback = %+v,%v calls=%d", first, err, sessions.authorityCalls)
	}
	second, err := service.Authenticate(context.Background(), rawToken, testAuthClient())
	if appErrorCode(err) != apperror.CodeDependencyUnavailable || sessions.authorityCalls != 0 {
		t.Fatalf("repaired cache = %+v,%v calls=%d", second, err, sessions.authorityCalls)
	}
}

func TestAuthenticateDoesNotQueryPostgreSQLWhenRedisFails(t *testing.T) {
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
	if appErrorCode(err) != apperror.CodeDependencyUnavailable || sessions.authorityCalls != 0 {
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
		PasswordSetRequired: true,
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
	if !credential.PasswordSetRequired {
		t.Fatal("refresh lost first-password guidance")
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
	identityKind    string
	identityAccount string
	verifiedInput   user.VerifiedIdentityInput
	verifiedUser    user.User
	verifiedFn      func(context.Context, user.VerifiedIdentityInput) (user.User, error)
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

func (f *fakeUserStore) FindCredentialByIdentity(_ context.Context, kind, account string) (user.Credential, error) {
	f.identityCalls++
	f.identityKind = kind
	f.identityAccount = account
	return f.credential, f.credentialErr
}

func (f *fakeUserStore) CreateVerifiedIdentity(ctx context.Context, input user.VerifiedIdentityInput) (user.User, error) {
	f.verifiedInput = input
	if f.verifiedFn != nil {
		return f.verifiedFn(ctx, input)
	}
	return f.verifiedUser, f.createErr
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

func (f *fakeVerifyCodeSender) VerifyCodeReady(context.Context, string) (messagemail.VerifyCodeReadiness, error) {
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

type fakePhoneVerifyCodeSender struct {
	readiness    messagesms.VerifyCodeReadiness
	readyErr     error
	preparation  messagesms.PhoneVerifyCodePreparation
	prepareErr   error
	result       messagesms.PhoneVerifyCodeResult
	sendErr      error
	readyCalls   int
	prepareCalls int
	sendCalls    int
	prepareInput messagesms.PhoneVerifyCodePrepareInput
	sendInput    messagesms.PhoneVerifyCodeInput
}

func (f *fakePhoneVerifyCodeSender) VerifyCodeReady(_ context.Context, scene string) (messagesms.VerifyCodeReadiness, error) {
	f.readyCalls++
	if scene != smstemplate.SceneLogin {
		return messagesms.VerifyCodeReadiness{}, fmt.Errorf("unexpected SMS readiness scene %q", scene)
	}
	return f.readiness, f.readyErr
}

func (f *fakePhoneVerifyCodeSender) PreparePhoneVerifyCode(_ context.Context, input messagesms.PhoneVerifyCodePrepareInput) (messagesms.PhoneVerifyCodePreparation, error) {
	f.prepareCalls++
	f.prepareInput = input
	return f.preparation, f.prepareErr
}

func (f *fakePhoneVerifyCodeSender) SendPreparedPhoneVerifyCode(_ context.Context, input messagesms.PhoneVerifyCodeInput) (messagesms.PhoneVerifyCodeResult, error) {
	f.sendCalls++
	f.sendInput = input
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
	consumeManyValid  bool
	consumeManyErr    error
	deleteErr         error
	releaseErr        error
	acquireCalls      int
	checkCalls        int
	consumeCalls      int
	consumeManyCalls  int
	deleteCalls       int
	releaseCalls      int
	putCalls          int
	putTTL            time.Duration
	deleteContextErr  error
	releaseContextErr error
	keyPlatform       string
	keyScene          string
	keyLoginType      string
	keyAccount        string
	proofChallengeID  string
	proofCode         string
}

func (f *fakeVerificationCodeStore) VerificationKey(platform, scene, loginType, account string) string {
	f.keyPlatform = platform
	f.keyScene = scene
	f.keyLoginType = loginType
	f.keyAccount = account
	return "verification-key"
}
func (f *fakeVerificationCodeStore) ProofDigest(challengeID, code string) string {
	f.proofChallengeID = challengeID
	f.proofCode = code
	return "digest:" + challengeID + ":" + code
}
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
func (f *fakeVerificationCodeStore) ConsumeMany(_ context.Context, keys []string, digests []string) (bool, error) {
	f.consumeManyCalls++
	if len(keys) == 0 || len(keys) != len(digests) {
		return false, errors.New("verification keys and digests must be non-empty and aligned")
	}
	if f.consumeManyErr != nil {
		return false, f.consumeManyErr
	}
	if f.consumeManyValid {
		return true, nil
	}
	return f.consumeValid, nil
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

func TestForgotPasswordSendsCodeWithForgetScene(t *testing.T) {
	fixedNow := time.Date(2026, time.September, 7, 10, 0, 0, 0, time.UTC)
	store := &fakeVerificationCodeStore{acquired: true}
	sender := &fakeVerifyCodeSender{
		readiness:   messagemail.VerifyCodeReadiness{Ready: true, TTLMinutes: 5},
		preparation: messagemail.EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60},
	}
	users := &fakeUserStore{credential: user.Credential{ID: 7, Email: "user@example.com", PasswordHash: "hash", IsEnabled: yesno.Yes}}
	service := NewService(users, nil, nil, &fakePolicyStore{policy: testPolicy()}, nil, nil, nil, nil, nil, nil, slog.New(slog.DiscardHandler))
	service.SetVerificationCodeStore(store)
	service.SetVerifyCodeSender(sender)
	service.now = func() time.Time { return fixedNow }

	result, err := service.ForgotPassword(context.Background(), ForgotPasswordInput{Account: "USER@Example.com", LoginType: authplatform.LoginTypeEmail, Client: testAuthClient()})
	if err != nil {
		t.Fatal(err)
	}
	if sender.prepareCalls != 1 || sender.prepareInput.Scene != messagemail.SceneForget || sender.prepareInput.ToEmail != "user@example.com" {
		t.Fatalf("prepare calls=%d input=%+v", sender.prepareCalls, sender.prepareInput)
	}
	if store.putCalls != 1 || store.putTTL != 5*time.Minute {
		t.Fatalf("put calls=%d ttl=%v", store.putCalls, store.putTTL)
	}
	if sender.sendCalls != 1 || sender.sendInput.Scene != messagemail.SceneForget || sender.sendInput.Preparation.TTLMinutes != 5 {
		t.Fatalf("send calls=%d input=%+v", sender.sendCalls, sender.sendInput)
	}
	if store.releaseCalls != 1 {
		t.Fatalf("release calls=%d", store.releaseCalls)
	}
	if !result.ExpiresAt.Equal(fixedNow.Add(5*time.Minute)) || result.ResendAfterSeconds != 60 {
		t.Fatalf("result = %+v", result)
	}
}

func TestForgotPasswordRejectsUnknownEmail(t *testing.T) {
	store := &fakeVerificationCodeStore{acquired: true}
	sender := &fakeVerifyCodeSender{
		readiness:   messagemail.VerifyCodeReadiness{Ready: true, TTLMinutes: 5},
		preparation: messagemail.EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60},
	}
	users := &fakeUserStore{credentialErr: fmt.Errorf("find user credential: %w", gorm.ErrRecordNotFound)}
	service := NewService(users, nil, nil, &fakePolicyStore{policy: testPolicy()}, nil, nil, nil, nil, nil, nil, slog.New(slog.DiscardHandler))
	service.SetVerificationCodeStore(store)
	service.SetVerifyCodeSender(sender)

	_, err := service.ForgotPassword(context.Background(), ForgotPasswordInput{Account: "ghost@example.com", LoginType: authplatform.LoginTypeEmail, Client: testAuthClient()})
	if code := appErrorCode(err); code != apperror.CodeNotFound {
		t.Fatalf("err = %v, code = %d, want %d", err, code, apperror.CodeNotFound)
	}
	if store.acquireCalls != 0 || sender.prepareCalls != 0 {
		t.Fatalf("lease acquired=%d prepare calls=%d; unknown email must fail before lease", store.acquireCalls, sender.prepareCalls)
	}
}

func TestForgotPasswordRequiresPasswordLoginType(t *testing.T) {
	emailOnly := testPolicy()
	emailOnly.LoginTypes = []authplatform.LoginType{authplatform.LoginTypeEmail}
	store := &fakeVerificationCodeStore{acquired: true}
	sender := &fakeVerifyCodeSender{
		readiness:   messagemail.VerifyCodeReadiness{Ready: true, TTLMinutes: 5},
		preparation: messagemail.EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60},
	}
	users := &fakeUserStore{credential: user.Credential{ID: 7, Email: "user@example.com", PasswordHash: "hash", IsEnabled: yesno.Yes}}
	service := NewService(users, nil, nil, &fakePolicyStore{policy: emailOnly}, nil, nil, nil, nil, nil, nil, slog.New(slog.DiscardHandler))
	service.SetVerificationCodeStore(store)
	service.SetVerifyCodeSender(sender)

	_, err := service.ForgotPassword(context.Background(), ForgotPasswordInput{Account: "user@example.com", LoginType: authplatform.LoginTypeEmail, Client: testAuthClient()})
	if code := appErrorCode(err); code != apperror.CodeForbidden {
		t.Fatalf("code = %d, want %d", code, apperror.CodeForbidden)
	}
	if store.acquireCalls != 0 || sender.prepareCalls != 0 {
		t.Fatalf("lease acquired=%d prepare calls=%d", store.acquireCalls, sender.prepareCalls)
	}
}

func TestForgotPasswordPhoneUsesSMSForgetScene(t *testing.T) {
	policy := testPolicy()
	policy.LoginTypes = []authplatform.LoginType{authplatform.LoginTypePhone, authplatform.LoginTypePassword}
	store := &fakeVerificationCodeStore{acquired: true}
	sender := &fakePhoneVerifyCodeSender{preparation: messagesms.PhoneVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60}}
	users := &fakeUserStore{credential: user.Credential{ID: 7, Username: "phone-user", PasswordHash: "hash", IsEnabled: yesno.Yes}}
	service := NewService(users, nil, nil, &fakePolicyStore{policy: policy}, nil, nil, nil, nil, nil, nil, slog.New(slog.DiscardHandler))
	service.SetVerificationCodeStore(store)
	service.SetPhoneVerifyCodeSender(sender)
	service.generateCode = func() (string, error) { return "123456", nil }

	result, err := service.ForgotPassword(context.Background(), ForgotPasswordInput{Account: "86 156-7162-8271", LoginType: authplatform.LoginTypePhone, Client: testAuthClient()})
	if err != nil {
		t.Fatal(err)
	}
	if users.identityKind != "phone" || users.identityAccount != "+8615671628271" || sender.prepareInput.Scene != smstemplate.SceneForget || sender.sendInput.Scene != smstemplate.SceneForget || sender.sendInput.ToPhone != "+8615671628271" {
		t.Fatalf("identity=%q/%q prepare=%+v send=%+v", users.identityKind, users.identityAccount, sender.prepareInput, sender.sendInput)
	}
	if store.keyLoginType != "phone" || store.keyScene != smstemplate.SceneForget || result.ChallengeID == "" {
		t.Fatalf("verification key=%q/%q result=%+v", store.keyLoginType, store.keyScene, result)
	}
}

func TestForgotPasswordPhoneFailsClosedWhenSMSIsUnavailable(t *testing.T) {
	policy := testPolicy()
	policy.LoginTypes = []authplatform.LoginType{authplatform.LoginTypePhone, authplatform.LoginTypePassword}
	store := &fakeVerificationCodeStore{acquired: true}
	sender := &fakePhoneVerifyCodeSender{prepareErr: apperror.DependencyUnavailable(errors.New("sms not ready"))}
	users := &fakeUserStore{credential: user.Credential{ID: 7, IsEnabled: yesno.Yes}}
	service := NewService(users, nil, nil, &fakePolicyStore{policy: policy}, nil, nil, nil, nil, nil, nil, slog.New(slog.DiscardHandler))
	service.SetVerificationCodeStore(store)
	service.SetPhoneVerifyCodeSender(sender)

	_, err := service.ForgotPassword(context.Background(), ForgotPasswordInput{Account: "15671628271", LoginType: authplatform.LoginTypePhone, Client: testAuthClient()})
	if appErrorCode(err) != apperror.CodeDependencyUnavailable || sender.sendCalls != 0 || store.releaseCalls != 1 || store.putCalls != 0 {
		t.Fatalf("error=%v send=%d release=%d put=%d", err, sender.sendCalls, store.releaseCalls, store.putCalls)
	}
}

type fakePasswordStore struct {
	credentialByID    user.Credential
	credentialByIDErr error
	platforms         []string
	platformsErr      error
	revokeCalls       int
	revokedUserID     int64
	revokedHash       string
	revokeResult      []user.RevokedSessionRef
	revokeErr         error
	setCalls          int
	setUserID         int64
	setHash           string
	setErr            error
	revokeOtherCalls  int
	keptSessionID     int64
	revokeOtherResult []user.RevokedSessionRef
	revokeOtherErr    error
}

func (f *fakePasswordStore) FindCredentialByID(context.Context, int64) (user.Credential, error) {
	return f.credentialByID, f.credentialByIDErr
}
func (f *fakePasswordStore) FindActiveSessionPlatforms(context.Context, int64) ([]string, error) {
	return f.platforms, f.platformsErr
}
func (f *fakePasswordStore) ChangePasswordAndRevokeSessions(_ context.Context, userID int64, passwordHash string, _ time.Time) ([]user.RevokedSessionRef, error) {
	f.revokeCalls++
	f.revokedUserID = userID
	f.revokedHash = passwordHash
	return f.revokeResult, f.revokeErr
}

func (f *fakePasswordStore) SetPasswordHash(_ context.Context, userID int64, passwordHash string, _ time.Time) error {
	f.setCalls++
	f.setUserID = userID
	f.setHash = passwordHash
	return f.setErr
}

func (f *fakePasswordStore) ChangePasswordAndRevokeOtherSessions(_ context.Context, userID, keepSessionID int64, passwordHash string, _ time.Time) ([]user.RevokedSessionRef, error) {
	f.revokeOtherCalls++
	f.revokedUserID = userID
	f.keptSessionID = keepSessionID
	f.revokedHash = passwordHash
	return f.revokeOtherResult, f.revokeOtherErr
}

func newResetPasswordService(t *testing.T, users *fakeUserStore, passwords *fakePasswordStore, codes *fakeVerificationCodeStore, policy authplatform.Policy) *Service {
	t.Helper()
	redisClient := openAuthRedis(t)
	stateStore := authstate.NewStore(redisClient)
	service := NewService(users, nil, nil, &fakePolicyStore{policy: policy}, stateStore, authstate.NewInvalidator(stateStore), authstate.NewSessionCache(redisClient), nil, nil, nil, slog.New(slog.DiscardHandler))
	service.SetPasswordStore(passwords)
	service.SetVerificationCodeStore(codes)
	return service
}

func TestResetPasswordReplacesHashAndRevokesSessions(t *testing.T) {
	passwords := &fakePasswordStore{platforms: []string{"admin"}, revokeResult: []user.RevokedSessionRef{{Platform: "admin", ID: 42}}}
	codes := &fakeVerificationCodeStore{checkValid: true, consumeValid: true}
	users := &fakeUserStore{credential: user.Credential{ID: 7, Email: "user@example.com", PasswordHash: "old", IsEnabled: yesno.Yes}}
	service := newResetPasswordService(t, users, passwords, codes, testPolicy())

	err := service.ResetPassword(context.Background(), ResetPasswordInput{
		Account: "USER@Example.com", LoginType: authplatform.LoginTypeEmail, ChallengeID: "challenge-1", Code: "123456", NewPassword: "NewPassw0rd!", ConfirmPassword: "NewPassw0rd!", Client: testAuthClient(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if codes.checkCalls != 1 || codes.consumeCalls != 1 {
		t.Fatalf("check=%d consume=%d", codes.checkCalls, codes.consumeCalls)
	}
	if passwords.revokeCalls != 1 || passwords.revokedUserID != 7 || passwords.revokedHash == "" || passwords.revokedHash == "old" {
		t.Fatalf("revoke calls=%d user=%d hash=%q", passwords.revokeCalls, passwords.revokedUserID, passwords.revokedHash)
	}
}

func TestResetPasswordPhoneNormalizesIdentityAndRevokesEveryPlatform(t *testing.T) {
	policy := testPolicy()
	policy.LoginTypes = []authplatform.LoginType{authplatform.LoginTypePhone, authplatform.LoginTypePassword}
	passwords := &fakePasswordStore{
		platforms:    []string{"admin", "canvas"},
		revokeResult: []user.RevokedSessionRef{{Platform: "admin", ID: 42}, {Platform: "canvas", ID: 43}},
	}
	codes := &fakeVerificationCodeStore{checkValid: true, consumeValid: true}
	users := &fakeUserStore{credential: user.Credential{ID: 7, Username: "phone-user", PasswordHash: "old", IsEnabled: yesno.Yes}}
	service := newResetPasswordService(t, users, passwords, codes, policy)

	err := service.ResetPassword(context.Background(), ResetPasswordInput{
		Account: "86 156-7162-8271", LoginType: authplatform.LoginTypePhone, ChallengeID: "challenge-1", Code: "123456",
		NewPassword: "NewPassw0rd!", ConfirmPassword: "NewPassw0rd!", Client: testAuthClient(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if users.identityKind != "phone" || users.identityAccount != "+8615671628271" || codes.keyLoginType != "phone" || codes.keyScene != smstemplate.SceneForget || codes.consumeCalls != 1 || passwords.revokeCalls != 1 {
		t.Fatalf("identity=%q/%q key=%q/%q consume=%d revoke=%d", users.identityKind, users.identityAccount, codes.keyLoginType, codes.keyScene, codes.consumeCalls, passwords.revokeCalls)
	}
}

func TestResetPasswordRejectsInvalidChallengeBeforeCodeLookup(t *testing.T) {
	passwords := &fakePasswordStore{}
	codes := &fakeVerificationCodeStore{checkValid: true, consumeValid: true}
	users := &fakeUserStore{credential: user.Credential{ID: 7, IsEnabled: yesno.Yes}}
	service := newResetPasswordService(t, users, passwords, codes, testPolicy())
	for _, challengeID := range []string{"", " challenge", "challenge\n", strings.Repeat("x", 129)} {
		err := service.ResetPassword(context.Background(), ResetPasswordInput{
			Account: "user@example.com", LoginType: authplatform.LoginTypeEmail, ChallengeID: challengeID, Code: "123456",
			NewPassword: "NewPassw0rd!", ConfirmPassword: "NewPassw0rd!", Client: testAuthClient(),
		})
		if appErrorCode(err) != apperror.CodeInvalidRequest {
			t.Fatalf("challenge %q error=%v", challengeID, err)
		}
	}
	if codes.checkCalls != 0 || codes.consumeCalls != 0 || passwords.revokeCalls != 0 {
		t.Fatalf("invalid challenge reached mutation: check=%d consume=%d revoke=%d", codes.checkCalls, codes.consumeCalls, passwords.revokeCalls)
	}
}

func TestResetPasswordWrongCodeDoesNotConsumeOrRevoke(t *testing.T) {
	passwords := &fakePasswordStore{platforms: []string{"admin"}}
	codes := &fakeVerificationCodeStore{checkValid: false}
	users := &fakeUserStore{credential: user.Credential{ID: 7, Email: "user@example.com", PasswordHash: "old", IsEnabled: yesno.Yes}}
	service := newResetPasswordService(t, users, passwords, codes, testPolicy())

	err := service.ResetPassword(context.Background(), ResetPasswordInput{
		Account: "user@example.com", LoginType: authplatform.LoginTypeEmail, ChallengeID: "challenge-1", Code: "000000", NewPassword: "NewPassw0rd!", ConfirmPassword: "NewPassw0rd!", Client: testAuthClient(),
	})
	if code := appErrorCode(err); code != apperror.CodeUnauthorized {
		t.Fatalf("err = %v code = %d, want unauthorized", err, code)
	}
	if codes.consumeCalls != 0 || passwords.revokeCalls != 0 {
		t.Fatalf("consume=%d revoke=%d; wrong code must not burn the code or touch sessions", codes.consumeCalls, passwords.revokeCalls)
	}
}

func TestResetPasswordRateLimitedBeforeConsume(t *testing.T) {
	passwords := &fakePasswordStore{platforms: []string{"admin"}}
	codes := &fakeVerificationCodeStore{checkValid: false, checkLimited: true}
	users := &fakeUserStore{credential: user.Credential{ID: 7, Email: "user@example.com", PasswordHash: "old", IsEnabled: yesno.Yes}}
	service := newResetPasswordService(t, users, passwords, codes, testPolicy())

	err := service.ResetPassword(context.Background(), ResetPasswordInput{
		Account: "user@example.com", LoginType: authplatform.LoginTypeEmail, ChallengeID: "challenge-1", Code: "123456", NewPassword: "NewPassw0rd!", ConfirmPassword: "NewPassw0rd!", Client: testAuthClient(),
	})
	if code := appErrorCode(err); code != apperror.CodeRateLimited {
		t.Fatalf("err = %v code = %d, want rate limited", err, code)
	}
	if codes.consumeCalls != 0 || passwords.revokeCalls != 0 {
		t.Fatalf("consume=%d revoke=%d", codes.consumeCalls, passwords.revokeCalls)
	}
}

func TestResetPasswordRejectsUnknownAndDisabledEmail(t *testing.T) {
	passwords := &fakePasswordStore{}
	codes := &fakeVerificationCodeStore{checkValid: true, consumeValid: true}
	unknown := &fakeUserStore{credentialErr: fmt.Errorf("find user credential: %w", gorm.ErrRecordNotFound)}
	service := newResetPasswordService(t, unknown, passwords, codes, testPolicy())
	err := service.ResetPassword(context.Background(), ResetPasswordInput{
		Account: "ghost@example.com", LoginType: authplatform.LoginTypeEmail, ChallengeID: "challenge-1", Code: "123456", NewPassword: "NewPassw0rd!", ConfirmPassword: "NewPassw0rd!", Client: testAuthClient(),
	})
	if code := appErrorCode(err); code != apperror.CodeNotFound {
		t.Fatalf("unknown email err = %v code = %d, want not found", err, code)
	}
	if codes.checkCalls != 0 {
		t.Fatalf("check=%d; unknown email must fail before code verification", codes.checkCalls)
	}

	disabled := &fakeUserStore{credential: user.Credential{ID: 8, Email: "disabled@example.com", PasswordHash: "old", IsEnabled: yesno.No}}
	service = newResetPasswordService(t, disabled, passwords, codes, testPolicy())
	err = service.ResetPassword(context.Background(), ResetPasswordInput{
		Account: "disabled@example.com", LoginType: authplatform.LoginTypeEmail, ChallengeID: "challenge-1", Code: "123456", NewPassword: "NewPassw0rd!", ConfirmPassword: "NewPassw0rd!", Client: testAuthClient(),
	})
	if code := appErrorCode(err); code != apperror.CodeNotFound {
		t.Fatalf("disabled email err = %v code = %d, want not found", err, code)
	}
}

func TestResetPasswordRequiresPasswordLoginTypeAndConfirmation(t *testing.T) {
	emailOnly := testPolicy()
	emailOnly.LoginTypes = []authplatform.LoginType{authplatform.LoginTypeEmail}
	passwords := &fakePasswordStore{}
	codes := &fakeVerificationCodeStore{checkValid: true, consumeValid: true}
	users := &fakeUserStore{credential: user.Credential{ID: 7, Email: "user@example.com", PasswordHash: "old", IsEnabled: yesno.Yes}}
	service := newResetPasswordService(t, users, passwords, codes, emailOnly)
	err := service.ResetPassword(context.Background(), ResetPasswordInput{
		Account: "user@example.com", LoginType: authplatform.LoginTypeEmail, ChallengeID: "challenge-1", Code: "123456", NewPassword: "NewPassw0rd!", ConfirmPassword: "NewPassw0rd!", Client: testAuthClient(),
	})
	if code := appErrorCode(err); code != apperror.CodeForbidden {
		t.Fatalf("err = %v code = %d, want forbidden", err, code)
	}

	service = newResetPasswordService(t, users, passwords, codes, testPolicy())
	err = service.ResetPassword(context.Background(), ResetPasswordInput{
		Account: "user@example.com", LoginType: authplatform.LoginTypeEmail, ChallengeID: "challenge-1", Code: "123456", NewPassword: "NewPassw0rd!", ConfirmPassword: "DifferentPass1!", Client: testAuthClient(),
	})
	if code := appErrorCode(err); code != apperror.CodeInvalidRequest {
		t.Fatalf("err = %v code = %d, want invalid request", err, code)
	}
	if codes.checkCalls != 0 {
		t.Fatalf("check=%d; mismatched confirmation must fail before code verification", codes.checkCalls)
	}
}

func TestSetPasswordStoresHashWithoutRevokingSessions(t *testing.T) {
	passwords := &fakePasswordStore{credentialByID: user.Credential{ID: 7, PasswordHash: "", IsEnabled: yesno.Yes}}
	service := NewService(&fakeUserStore{}, nil, nil, &fakePolicyStore{policy: testPolicy()}, nil, nil, nil, nil, nil, nil, slog.New(slog.DiscardHandler))
	service.SetPasswordStore(passwords)

	if err := service.SetPassword(context.Background(), Identity{UserID: 7}, SetPasswordInput{NewPassword: "NewPassw0rd!", ConfirmPassword: "NewPassw0rd!"}); err != nil {
		t.Fatal(err)
	}
	if passwords.setCalls != 1 || passwords.setUserID != 7 || passwords.setHash == "" {
		t.Fatalf("set calls=%d user=%d hash=%q", passwords.setCalls, passwords.setUserID, passwords.setHash)
	}
	if passwords.revokeCalls != 0 {
		t.Fatalf("revoke calls=%d; setting the first password must not revoke sessions", passwords.revokeCalls)
	}
}

func TestSetPasswordRejectsAccountWithExistingPassword(t *testing.T) {
	passwords := &fakePasswordStore{credentialByID: user.Credential{ID: 7, PasswordHash: "existing", IsEnabled: yesno.Yes}}
	service := NewService(&fakeUserStore{}, nil, nil, &fakePolicyStore{policy: testPolicy()}, nil, nil, nil, nil, nil, nil, slog.New(slog.DiscardHandler))
	service.SetPasswordStore(passwords)

	err := service.SetPassword(context.Background(), Identity{UserID: 7}, SetPasswordInput{NewPassword: "NewPassw0rd!", ConfirmPassword: "NewPassw0rd!"})
	if code := appErrorCode(err); code != apperror.CodeConflict {
		t.Fatalf("err = %v code = %d, want conflict", err, code)
	}
	if passwords.setCalls != 0 {
		t.Fatalf("set calls=%d; accounts with a password must use change password", passwords.setCalls)
	}
}

func TestSetPasswordMapsConcurrentWinnerToConflict(t *testing.T) {
	passwords := &fakePasswordStore{credentialByID: user.Credential{ID: 7, IsEnabled: yesno.Yes}, setErr: user.ErrPasswordAlreadySet}
	service := NewService(&fakeUserStore{}, nil, nil, &fakePolicyStore{policy: testPolicy()}, nil, nil, nil, nil, nil, nil, slog.New(slog.DiscardHandler))
	service.SetPasswordStore(passwords)
	err := service.SetPassword(context.Background(), Identity{UserID: 7}, SetPasswordInput{NewPassword: "NewPassw0rd!", ConfirmPassword: "NewPassw0rd!"})
	if appErrorCode(err) != apperror.CodeConflict {
		t.Fatalf("concurrent first set: %v", err)
	}
	if passwords.revokeCalls != 0 {
		t.Fatal("first set revoked sessions")
	}
}

func TestSetPasswordRejectsInvalidInputAndMissingIdentity(t *testing.T) {
	passwords := &fakePasswordStore{credentialByID: user.Credential{ID: 7, IsEnabled: yesno.Yes}}
	service := NewService(&fakeUserStore{}, nil, nil, &fakePolicyStore{policy: testPolicy()}, nil, nil, nil, nil, nil, nil, slog.New(slog.DiscardHandler))
	service.SetPasswordStore(passwords)

	err := service.SetPassword(context.Background(), Identity{UserID: 7}, SetPasswordInput{NewPassword: "NewPassw0rd!", ConfirmPassword: "DifferentPass1!"})
	if code := appErrorCode(err); code != apperror.CodeInvalidRequest {
		t.Fatalf("mismatch err = %v code = %d, want invalid request", err, code)
	}

	err = service.SetPassword(context.Background(), Identity{UserID: 7}, SetPasswordInput{NewPassword: "short", ConfirmPassword: "short"})
	if code := appErrorCode(err); code != apperror.CodeInvalidRequest {
		t.Fatalf("weak password err = %v code = %d, want invalid request", err, code)
	}

	err = service.SetPassword(context.Background(), Identity{}, SetPasswordInput{NewPassword: "NewPassw0rd!", ConfirmPassword: "NewPassw0rd!"})
	if code := appErrorCode(err); code != apperror.CodeUnauthorized {
		t.Fatalf("missing identity err = %v code = %d, want unauthorized", err, code)
	}
	if passwords.setCalls != 0 {
		t.Fatalf("set calls=%d", passwords.setCalls)
	}
}

func TestSendPasswordCodeUsesBoundPhoneAndChangePasswordScene(t *testing.T) {
	boundPhone := "+8615671628271"
	policy := testPolicy()
	policy.LoginTypes = []authplatform.LoginType{authplatform.LoginTypePhone, authplatform.LoginTypePassword}
	codes := &fakeVerificationCodeStore{acquired: true}
	sender := &fakePhoneVerifyCodeSender{preparation: messagesms.PhoneVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60}}
	users := &fakeUserStore{current: user.Current{ID: 7, Phone: &boundPhone}}
	service := NewService(users, nil, nil, &fakePolicyStore{policy: policy}, nil, nil, nil, nil, nil, nil, slog.New(slog.DiscardHandler))
	service.SetVerificationCodeStore(codes)
	service.SetPhoneVerifyCodeSender(sender)
	service.generateCode = func() (string, error) { return "123456", nil }

	result, err := service.SendPasswordCodeForLoginType(context.Background(), Identity{UserID: 7, SessionID: 8, PlatformID: 1, Platform: "admin"}, testAuthClient(), authplatform.LoginTypePhone)
	if err != nil {
		t.Fatal(err)
	}
	if sender.prepareInput.Scene != smstemplate.SceneChangePassword || sender.sendInput.Scene != smstemplate.SceneChangePassword || sender.sendInput.ToPhone != boundPhone || sender.sendInput.UserID == nil || *sender.sendInput.UserID != 7 {
		t.Fatalf("prepare=%+v send=%+v", sender.prepareInput, sender.sendInput)
	}
	if codes.keyScene != smstemplate.SceneChangePassword || codes.keyLoginType != "phone" || result.ChallengeID == "" {
		t.Fatalf("key=%q/%q result=%+v", codes.keyScene, codes.keyLoginType, result)
	}
}

func TestChangePasswordByCodeKeepsCurrentSessionAndRevokesOthers(t *testing.T) {
	boundPhone := "+8615671628271"
	policy := testPolicy()
	policy.LoginTypes = []authplatform.LoginType{authplatform.LoginTypePhone, authplatform.LoginTypePassword}
	passwords := &fakePasswordStore{
		platforms:         []string{"admin", "canvas"},
		revokeOtherResult: []user.RevokedSessionRef{{ID: 43, UserID: 7, Platform: "canvas"}},
	}
	codes := &fakeVerificationCodeStore{checkValid: true, consumeValid: true}
	users := &fakeUserStore{current: user.Current{ID: 7, Phone: &boundPhone}}
	service := newResetPasswordService(t, users, passwords, codes, policy)

	err := service.ChangePasswordByCode(context.Background(), Identity{UserID: 7, SessionID: 8, PlatformID: 1, Platform: "admin"}, testAuthClient(), ChangePasswordByCodeInput{
		LoginType: authplatform.LoginTypePhone, ChallengeID: "challenge-1", Code: "123456", NewPassword: "NewPassw0rd!", ConfirmPassword: "NewPassw0rd!",
	})
	if err != nil {
		t.Fatal(err)
	}
	if codes.keyScene != smstemplate.SceneChangePassword || codes.consumeCalls != 1 || passwords.revokeOtherCalls != 1 || passwords.keptSessionID != 8 || passwords.revokedHash == "" {
		t.Fatalf("key=%q consume=%d revokeOther=%d keep=%d hash=%q", codes.keyScene, codes.consumeCalls, passwords.revokeOtherCalls, passwords.keptSessionID, passwords.revokedHash)
	}
	if passwords.revokeCalls != 0 {
		t.Fatalf("all-session revocation called %d times", passwords.revokeCalls)
	}
}

func TestPasswordCodeFlowsRejectAccountWithoutBoundPhone(t *testing.T) {
	policy := testPolicy()
	policy.LoginTypes = []authplatform.LoginType{authplatform.LoginTypePhone, authplatform.LoginTypePassword}
	service := NewService(&fakeUserStore{current: user.Current{ID: 7}}, nil, nil, &fakePolicyStore{policy: policy}, nil, nil, nil, nil, nil, nil, slog.New(slog.DiscardHandler))
	_, err := service.SendPasswordCodeForLoginType(context.Background(), Identity{UserID: 7, SessionID: 8, PlatformID: 1, Platform: "admin"}, testAuthClient(), authplatform.LoginTypePhone)
	if appErrorCode(err) != apperror.CodeNotFound {
		t.Fatalf("send error=%v", err)
	}
}
