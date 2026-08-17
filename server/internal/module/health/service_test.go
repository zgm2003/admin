package health_test

import (
	"context"
	"errors"
	"testing"

	"admin/server/internal/module/health"
	"admin/server/internal/shared/apperror"
)

type contextKey string

type probe struct {
	err       error
	calls     int
	contextID string
}

func (p *probe) Ping(ctx context.Context) error {
	p.calls++
	p.contextID, _ = ctx.Value(contextKey("requestID")).(string)
	return p.err
}

func TestReadinessChecksBothDependenciesWithTheRequestContext(t *testing.T) {
	postgres := &probe{}
	redis := &probe{}
	service := health.NewService(postgres, redis)
	ctx := context.WithValue(context.Background(), contextKey("requestID"), "request-1")

	got, err := service.Readiness(ctx)
	if err != nil {
		t.Fatalf("Readiness() error = %v", err)
	}
	if got.PostgreSQL != "up" || got.Redis != "up" {
		t.Fatalf("Readiness() = %+v", got)
	}
	if postgres.calls != 1 || redis.calls != 1 || postgres.contextID != "request-1" || redis.contextID != "request-1" {
		t.Fatalf("probe calls postgres=%+v redis=%+v", postgres, redis)
	}
}

func TestReadinessReturnsStableErrorAndStillChecksBothDependencies(t *testing.T) {
	postgres := &probe{err: errors.New("postgres down")}
	redis := &probe{}
	service := health.NewService(postgres, redis)

	_, err := service.Readiness(context.Background())
	if err == nil {
		t.Fatal("expected readiness failure")
	}
	appErr, ok := err.(*apperror.Error)
	if !ok || appErr.Code != apperror.CodeDependencyUnavailable {
		t.Fatalf("error = %#v", err)
	}
	if postgres.calls != 1 || redis.calls != 1 {
		t.Fatalf("probe calls postgres=%d redis=%d", postgres.calls, redis.calls)
	}
}
