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
	if err := db.WithContext(ctx).Exec(`INSERT INTO message_mail_config (platform_id, region, from_email, from_name, ttl_minutes, is_enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, 1, "ap-guangzhou", "sender@example.com", "Sender", 10, yesno.Yes, now, now).Error; err != nil {
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
