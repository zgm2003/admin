package mail

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type limiterStub struct {
	allowed bool
	err     error
}

func (s limiterStub) Allow(context.Context, LimitRequest) (bool, error) {
	return s.allowed, s.err
}

type recordingLimiter struct {
	requests []LimitRequest
	allowed  bool
	err      error
}

func (l *recordingLimiter) Allow(_ context.Context, request LimitRequest) (bool, error) {
	l.requests = append(l.requests, request)
	return l.allowed, l.err
}

type ruleEvaluatorStub struct {
	decision RuleDecision
	err      error
}

func (s ruleEvaluatorStub) Evaluate(context.Context, int64, string, SendMode) (RuleDecision, error) {
	return s.decision, s.err
}

type stubRateLimitPolicyStore struct {
	catalog RateLimitCatalog
	err     error
}

type stubVerifyCodeReadinessStore struct {
	readiness          VerifyCodeReadiness
	currentErr         error
	beginErr           error
	publishErr         error
	rollbackErr        error
	cancelAfterBegin   context.CancelFunc
	beginCalls         int
	publishCalls       int
	rollbackCalls      int
	publishContextErr  error
	rollbackContextErr error
}

func (s *stubVerifyCodeReadinessStore) Current(context.Context, int64, string) (VerifyCodeReadiness, error) {
	return s.readiness, s.currentErr
}

func (s *stubVerifyCodeReadinessStore) BeginMutation(context.Context, int64, string) (VerifyCodeReadinessMutation, error) {
	s.beginCalls++
	if s.cancelAfterBegin != nil {
		s.cancelAfterBegin()
	}
	return VerifyCodeReadinessMutation{platformID: 1, scene: SceneLogin, priorPayload: "prior", invalidatingPayload: "invalidating"}, s.beginErr
}

func (s *stubVerifyCodeReadinessStore) PublishMutation(ctx context.Context, _ VerifyCodeReadinessMutation) error {
	s.publishCalls++
	s.publishContextErr = ctx.Err()
	return s.publishErr
}

func (s *stubVerifyCodeReadinessStore) RollbackMutation(ctx context.Context, _ VerifyCodeReadinessMutation) error {
	s.rollbackCalls++
	s.rollbackContextErr = ctx.Err()
	return s.rollbackErr
}

func (s stubRateLimitPolicyStore) Load(context.Context) (RateLimitCatalog, error) {
	return s.catalog, s.err
}

func (s stubRateLimitPolicyStore) Update(context.Context, RateLimitPolicyInput) (RateLimitCatalog, error) {
	return s.catalog, s.err
}

func defaultPolicyCatalog() RateLimitCatalog {
	return RateLimitCatalog{Version: 1, Policies: FixedRateLimitPolicies()}
}

func policyCatalogWith(overrides map[string][2]int) RateLimitCatalog {
	policies := FixedRateLimitPolicies()
	for i := range policies {
		if value, ok := overrides[policies[i].Key]; ok {
			policies[i].Limit = value[0]
			policies[i].WindowSeconds = value[1]
		}
	}
	return RateLimitCatalog{Version: 1, Policies: policies}
}

func TestSendReturnsRecipientDeniedAsMailBusinessError(t *testing.T) {
	service := NewService(nil, nil, nil, ruleEvaluatorStub{decision: RuleDecision{Allowed: false}}, nil, stubRateLimitPolicyStore{catalog: defaultPolicyCatalog()})

	_, err := service.Send(context.Background(), validBusinessSendInput())

	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v, want application error", err)
	}
	if appErr.HTTPStatus != http.StatusForbidden ||
		appErr.Code != CodeRecipientDenied ||
		appErr.MessageKey != i18n.KeyMailRecipientDenied {
		t.Fatalf("application error = status %d, code %d, key %q", appErr.HTTPStatus, appErr.Code, appErr.MessageKey)
	}
}

func TestSendReturnsRateLimitedWhenLimiterRejects(t *testing.T) {
	service := NewService(nil, nil, nil, nil, limiterStub{allowed: false}, stubRateLimitPolicyStore{catalog: defaultPolicyCatalog()})

	_, err := service.Send(context.Background(), validBusinessSendInput())

	assertApplicationError(t, err, http.StatusTooManyRequests, apperror.CodeRateLimited)
}

func TestSendReturnsDependencyUnavailableWhenLimiterFails(t *testing.T) {
	service := NewService(nil, nil, nil, nil, limiterStub{err: errors.New("redis unavailable")}, stubRateLimitPolicyStore{catalog: defaultPolicyCatalog()})

	_, err := service.Send(context.Background(), validBusinessSendInput())

	assertApplicationError(t, err, http.StatusServiceUnavailable, apperror.CodeDependencyUnavailable)
}

func TestForPlatformReturnsRateLimitedWhenLimiterRejects(t *testing.T) {
	service := NewService(nil, nil, nil, nil, limiterStub{allowed: false}, stubRateLimitPolicyStore{catalog: defaultPolicyCatalog()})

	_, err := service.TestForPlatform(context.Background(), 1, validAdminTestInput())

	assertApplicationError(t, err, http.StatusTooManyRequests, apperror.CodeRateLimited)
}

func TestForPlatformReturnsDependencyUnavailableWhenLimiterFails(t *testing.T) {
	service := NewService(nil, nil, nil, nil, limiterStub{err: errors.New("redis unavailable")}, stubRateLimitPolicyStore{catalog: defaultPolicyCatalog()})

	_, err := service.TestForPlatform(context.Background(), 1, validAdminTestInput())

	assertApplicationError(t, err, http.StatusServiceUnavailable, apperror.CodeDependencyUnavailable)
}

func TestSendReturnsDependencyUnavailableWhenStoreFails(t *testing.T) {
	limiter := &recordingLimiter{allowed: true}
	service := NewService(nil, nil, nil, nil, limiter, stubRateLimitPolicyStore{err: errors.New("redis down")})

	_, err := service.Send(context.Background(), validBusinessSendInput())

	assertApplicationError(t, err, http.StatusServiceUnavailable, apperror.CodeDependencyUnavailable)
	if len(limiter.requests) != 0 {
		t.Fatalf("limiter was called %d times before the store failure surfaced", len(limiter.requests))
	}
}

func TestSendReturnsDependencyUnavailableWhenPolicyStoreIsMissing(t *testing.T) {
	limiter := &recordingLimiter{allowed: true}
	service := NewService(nil, nil, nil, nil, limiter, nil)

	_, err := service.Send(context.Background(), validBusinessSendInput())

	assertApplicationError(t, err, http.StatusServiceUnavailable, apperror.CodeDependencyUnavailable)
	if len(limiter.requests) != 0 {
		t.Fatalf("limiter was called %d times before the missing store failure surfaced", len(limiter.requests))
	}
}

func TestForPlatformReturnsDependencyUnavailableWhenPolicyStoreIsMissing(t *testing.T) {
	limiter := &recordingLimiter{allowed: true}
	service := NewService(nil, nil, nil, nil, limiter, nil)

	_, err := service.TestForPlatform(context.Background(), 1, validAdminTestInput())

	assertApplicationError(t, err, http.StatusServiceUnavailable, apperror.CodeDependencyUnavailable)
	if len(limiter.requests) != 0 {
		t.Fatalf("limiter was called %d times before the missing store failure surfaced", len(limiter.requests))
	}
}

func TestSendUsesCurrentBusinessPolicySnapshot(t *testing.T) {
	db, ctx := openMailServiceDatabase(t)
	sender := &countingSender{}
	limiter := &recordingLimiter{allowed: true}
	service := NewService(
		NewRepository(db), nil, sender, nil, limiter,
		stubRateLimitPolicyStore{catalog: policyCatalogWith(map[string][2]int{
			"business_email_minute": {2, 120},
			"business_email_10m":    {7, 900},
			"business_ip_minute":    {11, 90},
			"business_scene_minute": {31, 120},
		})},
	)

	if _, err := service.Send(ctx, validBusinessSendInput()); err != nil {
		t.Fatal(err)
	}
	if len(limiter.requests) != 4 {
		t.Fatalf("limiter requests = %d, want 4", len(limiter.requests))
	}
	if got := limiter.requests[0]; got.Limit != 2 || got.Window != 120*time.Second {
		t.Fatalf("first request = %+v", got)
	}
	if got := limiter.requests[3]; got.Limit != 31 || got.Window != 120*time.Second {
		t.Fatalf("last request = %+v", got)
	}
}

func TestAdminTestUsesCurrentPolicySnapshot(t *testing.T) {
	db, ctx := openMailServiceDatabase(t)
	sender := &countingSender{}
	limiter := &recordingLimiter{allowed: true}
	service := NewService(
		NewRepository(db), nil, sender, nil, limiter,
		stubRateLimitPolicyStore{catalog: policyCatalogWith(map[string][2]int{
			"admin_test_user_10m":  {6, 600},
			"admin_test_ip_minute": {11, 60},
			"admin_test_email_10m": {4, 600},
		})},
	)

	if _, err := service.TestForPlatform(ctx, 1, validAdminTestInput()); err != nil {
		t.Fatal(err)
	}
	if len(limiter.requests) != 3 {
		t.Fatalf("limiter requests = %d, want 3", len(limiter.requests))
	}
	if got := limiter.requests[0]; got.Limit != 6 || got.Window != 600*time.Second {
		t.Fatalf("admin user request = %+v", got)
	}
	if got := limiter.requests[2]; got.Limit != 4 || got.Window != 600*time.Second {
		t.Fatalf("admin email request = %+v", got)
	}
}

func openMailServiceDatabase(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	db, ctx := openMailRepositoryDatabase(t)
	if err := db.WithContext(ctx).Exec(`
		CREATE TABLE message_mail_config (
			id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
			platform_id BIGINT NOT NULL,
			secret_id_ciphertext TEXT NOT NULL DEFAULT '',
			secret_key_ciphertext TEXT NOT NULL DEFAULT '',
			secret_id_hint VARCHAR(32) NOT NULL DEFAULT '',
			secret_key_hint VARCHAR(32) NOT NULL DEFAULT '',
			region VARCHAR(64) NOT NULL,
			endpoint VARCHAR(255),
			from_email VARCHAR(254) NOT NULL,
			from_name VARCHAR(128) NOT NULL,
			reply_to VARCHAR(254),
			ttl_minutes SMALLINT NOT NULL,
			is_enabled SMALLINT NOT NULL,
			last_test_at TIMESTAMPTZ,
			last_test_error VARCHAR(512) NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			deleted_at TIMESTAMPTZ
		);
		CREATE TABLE message_mail_template (
			id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
			platform_id BIGINT NOT NULL,
			scene VARCHAR(32) NOT NULL,
			name VARCHAR(128) NOT NULL,
			subject VARCHAR(255) NOT NULL,
			tencent_template_id INTEGER NOT NULL,
			variables JSONB NOT NULL,
			example_variables JSONB NOT NULL,
			is_enabled SMALLINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			deleted_at TIMESTAMPTZ
		);
	`).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := db.WithContext(ctx).Exec(`INSERT INTO message_mail_config (platform_id, secret_id_ciphertext, secret_key_ciphertext, region, from_email, from_name, ttl_minutes, is_enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, 1, "configured", "configured", "ap-guangzhou", "sender@example.com", "Sender", 5, yesno.Yes, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO message_mail_template (platform_id, scene, name, subject, tencent_template_id, variables, example_variables, is_enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?::jsonb, ?::jsonb, ?, ?, ?)`, 1, SceneLogin, "Login", "Login code", 47941, `{"code":"123456","ttl_minutes":"10"}`, `{"code":"123456","ttl_minutes":"10"}`, yesno.Yes, now, now).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func validBusinessSendInput() BusinessSendInput {
	return BusinessSendInput{
		PlatformID: 1,
		Scene:      SceneLogin,
		ToEmail:    "user@example.com",
		Variables:  map[string]string{"code": "123456", "ttl_minutes": "10"},
	}
}

func validAdminTestInput() AdminTestInput {
	return AdminTestInput{
		AdminUserID: 1,
		Scene:       SceneLogin,
		ToEmail:     "user@example.com",
		Variables:   map[string]string{"code": "123456", "ttl_minutes": "10"},
	}
}

func assertApplicationError(t *testing.T, err error, wantStatus, wantCode int) {
	t.Helper()
	var got *apperror.Error
	if !errors.As(err, &got) {
		t.Fatalf("error = %v, want application error", err)
	}
	if got.HTTPStatus != wantStatus || got.Code != wantCode {
		t.Fatalf("application error = status %d, code %d; want status %d, code %d", got.HTTPStatus, got.Code, wantStatus, wantCode)
	}
}

func TestErrorSummaryUnwrapsApplicationError(t *testing.T) {
	err := apperror.DependencyUnavailable(errors.New("mail config disabled"))
	if got := errorSummary(err); got != "mail config disabled" {
		t.Fatalf("errorSummary() = %q, want %q", got, "mail config disabled")
	}
}

func TestVerifyCodeReadyReflectsConfigState(t *testing.T) {
	db, ctx := openMailServiceDatabase(t)
	redisClient := openMailReadinessRedis(t)
	repository := NewRepository(db)
	readinessStore := NewVerifyCodeReadinessStore(repository, redisClient)
	keys := []string{verifyCodeReadinessKey(1, SceneLogin), verifyCodeReadinessLoadLockKey(1, SceneLogin)}
	if err := redisClient.DeleteMany(ctx, keys); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = redisClient.DeleteMany(context.Background(), keys) })
	service := NewService(repository, nil, nil, nil, nil, stubRateLimitPolicyStore{catalog: defaultPolicyCatalog()})
	service.SetVerifyCodeReadinessStore(readinessStore)

	ready, err := service.VerifyCodeReady(ctx, 1, SceneLogin)
	if err != nil || !ready.Ready || ready.TTLMinutes < 1 {
		t.Fatalf("ready = %v, %v", ready, err)
	}
	if ready, err := service.VerifyCodeReady(ctx, 1, "forget"); err != nil || ready.Ready {
		t.Fatalf("unsupported scene ready = %v, %v", ready, err)
	}

	template, err := repository.FindTemplateByScene(ctx, 1, SceneLogin)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SetTemplateStatus(ctx, 1, template.ID, yesno.No); err != nil {
		t.Fatal(err)
	}
	ready, err = service.VerifyCodeReady(ctx, 1, SceneLogin)
	if err != nil || ready.Ready {
		t.Fatalf("disabled config ready = %v, %v", ready, err)
	}
}

func TestReadinessMutationRollbackOutlivesCanceledRequest(t *testing.T) {
	db, _ := openMailServiceDatabase(t)
	ctx, cancel := context.WithCancel(context.Background())
	readiness := &stubVerifyCodeReadinessStore{readiness: VerifyCodeReadiness{Ready: true}, cancelAfterBegin: cancel}
	service := NewService(NewRepository(db), nil, nil, nil, nil, stubRateLimitPolicyStore{catalog: defaultPolicyCatalog()})
	service.SetVerifyCodeReadinessStore(readiness)

	err := service.SetTemplateStatus(ctx, 1, 1, yesno.No)
	if err == nil || readiness.beginCalls != 1 || readiness.rollbackCalls != 1 || readiness.publishCalls != 0 {
		t.Fatalf("status error=%v begin=%d rollback=%d publish=%d", err, readiness.beginCalls, readiness.rollbackCalls, readiness.publishCalls)
	}
	if readiness.rollbackContextErr != nil {
		t.Fatalf("readiness rollback reused canceled request context: %v", readiness.rollbackContextErr)
	}
}

func TestReadinessPublicationOutlivesCanceledRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	readiness := &stubVerifyCodeReadinessStore{}
	service := NewService(nil, nil, nil, nil, nil, nil)
	service.SetVerifyCodeReadinessStore(readiness)
	mutation := VerifyCodeReadinessMutation{platformID: 1, scene: SceneLogin, priorPayload: "prior", invalidatingPayload: "invalidating"}

	if err := service.publishVerifyCodeReadinessMutation(ctx, mutation); err != nil {
		t.Fatal(err)
	}
	if readiness.publishCalls != 1 || readiness.publishContextErr != nil {
		t.Fatalf("publication calls=%d context error=%v", readiness.publishCalls, readiness.publishContextErr)
	}
}

func TestVerifyCodeReadyFailsWhenRateLimitPolicyIsUnavailable(t *testing.T) {
	db, ctx := openMailServiceDatabase(t)
	service := NewService(NewRepository(db), nil, nil, nil, nil, stubRateLimitPolicyStore{err: errors.New("redis unavailable")})

	ready, err := service.VerifyCodeReady(ctx, 1, SceneLogin)
	if ready.Ready {
		t.Fatal("mail reported ready while its rate-limit policy was unavailable")
	}
	assertApplicationError(t, err, http.StatusServiceUnavailable, apperror.CodeDependencyUnavailable)
}

func openMailServiceWithReadiness(t *testing.T, limiter Limiter, policyStore RateLimitPolicyStore) (*Service, context.Context) {
	t.Helper()
	db, ctx := openMailServiceDatabase(t)
	repository := NewRepository(db)
	redisClient := openMailReadinessRedis(t)
	readinessStore := NewVerifyCodeReadinessStore(repository, redisClient)
	keys := []string{verifyCodeReadinessKey(1, SceneLogin), verifyCodeReadinessLoadLockKey(1, SceneLogin)}
	if err := redisClient.DeleteMany(ctx, keys); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = redisClient.DeleteMany(context.Background(), keys) })
	service := NewService(repository, nil, nil, nil, limiter, policyStore)
	service.SetVerifyCodeReadinessStore(readinessStore)
	return service, ctx
}

func TestPrepareEmailVerifyCodeReturnsConfigTTLAndResendWindow(t *testing.T) {
	limiter := &recordingLimiter{allowed: true}
	service, ctx := openMailServiceWithReadiness(t, limiter, stubRateLimitPolicyStore{catalog: policyCatalogWith(map[string][2]int{
		"business_email_minute": {1, 60},
		"business_email_10m":    {5, 600},
		"business_ip_minute":    {10, 60},
		"business_scene_minute": {20, 60},
	})})
	preparation, err := service.PrepareEmailVerifyCode(ctx, EmailVerifyCodePrepareInput{PlatformID: 1, ClientIP: "127.0.0.1", Scene: SceneLogin, ToEmail: "user@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if preparation.TTLMinutes != 5 || preparation.ResendAfterSeconds != 60 {
		t.Fatalf("preparation = %+v", preparation)
	}
	if len(limiter.requests) != 4 {
		t.Fatalf("limiter requests = %d, want 4", len(limiter.requests))
	}
}

func TestPrepareEmailVerifyCodeRateLimitsBeforeSend(t *testing.T) {
	sender := &countingSender{}
	limiter := &recordingLimiter{allowed: true}
	service, ctx := openMailServiceWithReadiness(t, limiter, stubRateLimitPolicyStore{catalog: defaultPolicyCatalog()})
	service.sender = sender
	if _, err := service.PrepareEmailVerifyCode(ctx, EmailVerifyCodePrepareInput{PlatformID: 1, ClientIP: "127.0.0.1", Scene: SceneLogin, ToEmail: "user@example.com"}); err != nil {
		t.Fatal(err)
	}
	limiter.allowed = false
	_, err := service.PrepareEmailVerifyCode(ctx, EmailVerifyCodePrepareInput{PlatformID: 1, ClientIP: "127.0.0.1", Scene: SceneLogin, ToEmail: "user@example.com"})
	assertApplicationError(t, err, http.StatusTooManyRequests, apperror.CodeRateLimited)
	if sender.calls.Load() != 0 {
		t.Fatalf("prepare reached the sender %d times", sender.calls.Load())
	}
}

func TestSendPreparedEmailVerifyCodeSendsLoginEmail(t *testing.T) {
	db, ctx := openMailServiceDatabase(t)
	sender := &countingSender{}
	service := NewService(NewRepository(db), nil, sender, nil, &recordingLimiter{allowed: true}, stubRateLimitPolicyStore{catalog: defaultPolicyCatalog()})
	expiresAt := time.Now().UTC().Add(5 * time.Minute)
	result, err := service.SendPreparedEmailVerifyCode(ctx, EmailVerifyCodeInput{
		PlatformID: 1, ClientIP: "127.0.0.1", ChallengeID: "challenge-1",
		Scene: SceneLogin, ToEmail: "user@example.com", Code: "123456",
		ExpiresAt: expiresAt, Preparation: EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.LogID < 1 || result.ChallengeID != "challenge-1" || !result.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("result = %+v", result)
	}
	if sender.calls.Load() != 1 {
		t.Fatalf("sender calls = %d, want 1", sender.calls.Load())
	}
}

func TestSendPreparedEmailVerifyCodeDoesNotRateLimitAgain(t *testing.T) {
	db, ctx := openMailServiceDatabase(t)
	sender := &countingSender{}
	limiter := &recordingLimiter{allowed: true}
	service := NewService(NewRepository(db), nil, sender, nil, limiter, stubRateLimitPolicyStore{catalog: defaultPolicyCatalog()})
	expiresAt := time.Now().UTC().Add(5 * time.Minute)
	preparation := EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60}
	if _, err := service.SendPreparedEmailVerifyCode(ctx, EmailVerifyCodeInput{PlatformID: 1, ClientIP: "127.0.0.1", ChallengeID: "prepared-1", Scene: SceneLogin, ToEmail: "user@example.com", Code: "123456", ExpiresAt: expiresAt, Preparation: preparation}); err != nil {
		t.Fatal(err)
	}
	if len(limiter.requests) != 0 {
		t.Fatalf("prepared send hit the limiter %d times, want 0", len(limiter.requests))
	}
}

func TestSendPreparedEmailVerifyCodeRejectsReusedChallenge(t *testing.T) {
	db, ctx := openMailServiceDatabase(t)
	sender := &countingSender{}
	service := NewService(NewRepository(db), nil, sender, nil, &recordingLimiter{allowed: true}, stubRateLimitPolicyStore{catalog: defaultPolicyCatalog()})
	input := EmailVerifyCodeInput{PlatformID: 1, ClientIP: "127.0.0.1", ChallengeID: "reused-challenge", Scene: SceneLogin, ToEmail: "user@example.com", Code: "123456", ExpiresAt: time.Now().UTC().Add(5 * time.Minute), Preparation: EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60}}
	if _, err := service.SendPreparedEmailVerifyCode(ctx, input); err != nil {
		t.Fatal(err)
	}
	input.Code = "654321"
	if _, err := service.SendPreparedEmailVerifyCode(ctx, input); err == nil {
		t.Fatal("reused challenge reported a new verification code as sent")
	}
	if sender.calls.Load() != 1 {
		t.Fatalf("sender calls = %d, want 1", sender.calls.Load())
	}
}

type inputRecordingSender struct{ input SendInput }

func (s *inputRecordingSender) Send(_ context.Context, input SendInput) (ProviderSendResult, error) {
	s.input = input
	return ProviderSendResult{RequestID: "request", MessageID: "message"}, nil
}

func TestSendPreparedEmailVerifyCodePersistsPreparationTTL(t *testing.T) {
	db, ctx := openMailServiceDatabase(t)
	sender := &inputRecordingSender{}
	service := NewService(NewRepository(db), nil, sender, nil, &recordingLimiter{allowed: true}, stubRateLimitPolicyStore{catalog: defaultPolicyCatalog()})
	if _, err := service.SendPreparedEmailVerifyCode(ctx, EmailVerifyCodeInput{PlatformID: 1, ClientIP: "127.0.0.1", ChallengeID: "ttl-challenge", Scene: SceneLogin, ToEmail: "user@example.com", Code: "123456", ExpiresAt: time.Now().UTC().Add(5 * time.Minute), Preparation: EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60}}); err != nil {
		t.Fatal(err)
	}
	if sender.input.TemplateData["ttl_minutes"] != "5" {
		t.Fatalf("mail template TTL = %q, want preparation TTL 5", sender.input.TemplateData["ttl_minutes"])
	}
}

func TestSendPreparedEmailVerifyCodeRejectsInvalidInput(t *testing.T) {
	db, ctx := openMailServiceDatabase(t)
	service := NewService(NewRepository(db), nil, nil, nil, nil, stubRateLimitPolicyStore{catalog: defaultPolicyCatalog()})
	now := time.Now().UTC()
	for _, in := range []EmailVerifyCodeInput{
		{PlatformID: 1, Scene: "forget", ToEmail: "user@example.com", Code: "123456", ExpiresAt: now.Add(5 * time.Minute), Preparation: EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60}},
		{PlatformID: 1, Scene: SceneLogin, ToEmail: "user@example.com", Code: "12345", ExpiresAt: now.Add(5 * time.Minute), Preparation: EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60}},
		{PlatformID: 1, Scene: SceneLogin, ToEmail: "user@example.com", Code: "123456", ExpiresAt: now.Add(5 * time.Minute), Preparation: EmailVerifyCodePreparation{TTLMinutes: 0, ResendAfterSeconds: 60}},
		{PlatformID: 1, Scene: SceneLogin, ToEmail: "user@example.com", Code: "123456", ExpiresAt: now.Add(-time.Minute), Preparation: EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60}},
		{PlatformID: 1, Scene: SceneLogin, ToEmail: "user@example.com", Code: "123456", ExpiresAt: now.Add(10 * time.Minute), Preparation: EmailVerifyCodePreparation{TTLMinutes: 5, ResendAfterSeconds: 60}},
	} {
		if _, err := service.SendPreparedEmailVerifyCode(ctx, in); err == nil {
			t.Fatalf("accepted invalid input %+v", in)
		}
	}
}
