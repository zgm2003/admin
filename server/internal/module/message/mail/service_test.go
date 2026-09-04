package mail

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
)

type limiterStub struct {
	allowed bool
	err     error
}

func (s limiterStub) Allow(context.Context, LimitRequest) (bool, error) {
	return s.allowed, s.err
}

type ruleEvaluatorStub struct {
	decision RuleDecision
	err      error
}

func (s ruleEvaluatorStub) Evaluate(context.Context, int64, string, SendMode) (RuleDecision, error) {
	return s.decision, s.err
}

func TestSendReturnsRecipientDeniedAsMailBusinessError(t *testing.T) {
	service := NewService(nil, nil, nil, ruleEvaluatorStub{decision: RuleDecision{Allowed: false}}, nil)

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
	service := NewService(nil, nil, nil, nil, limiterStub{allowed: false})

	_, err := service.Send(context.Background(), validBusinessSendInput())

	assertApplicationError(t, err, http.StatusTooManyRequests, apperror.CodeRateLimited)
}

func TestSendReturnsDependencyUnavailableWhenLimiterFails(t *testing.T) {
	service := NewService(nil, nil, nil, nil, limiterStub{err: errors.New("redis unavailable")})

	_, err := service.Send(context.Background(), validBusinessSendInput())

	assertApplicationError(t, err, http.StatusServiceUnavailable, apperror.CodeDependencyUnavailable)
}

func TestForPlatformReturnsRateLimitedWhenLimiterRejects(t *testing.T) {
	service := NewService(nil, nil, nil, nil, limiterStub{allowed: false})

	_, err := service.TestForPlatform(context.Background(), 1, validAdminTestInput())

	assertApplicationError(t, err, http.StatusTooManyRequests, apperror.CodeRateLimited)
}

func TestForPlatformReturnsDependencyUnavailableWhenLimiterFails(t *testing.T) {
	service := NewService(nil, nil, nil, nil, limiterStub{err: errors.New("redis unavailable")})

	_, err := service.TestForPlatform(context.Background(), 1, validAdminTestInput())

	assertApplicationError(t, err, http.StatusServiceUnavailable, apperror.CodeDependencyUnavailable)
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
