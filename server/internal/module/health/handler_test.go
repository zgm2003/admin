package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"admin/server/internal/module/health"
	"admin/server/internal/shared/apperror"
	"github.com/gin-gonic/gin"
)

type readinessService struct {
	result health.Readiness
	err    error
	ctx    context.Context
	calls  int
}

func (s *readinessService) Readiness(ctx context.Context) (health.Readiness, error) {
	s.calls++
	s.ctx = ctx
	return s.result, s.err
}

func TestHealthDoesNotProbeDependencies(t *testing.T) {
	service := &readinessService{err: errors.New("must not be called")}
	router := testRouter(service)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if service.calls != 0 {
		t.Fatalf("readiness calls = %d", service.calls)
	}
}

func TestReadyReturnsDependencyStateAndPassesRequestContext(t *testing.T) {
	service := &readinessService{result: health.Readiness{PostgreSQL: "up", Redis: "up"}}
	router := testRouter(service)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body)
	}
	if service.ctx != request.Context() {
		t.Fatal("handler did not pass request context")
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data := body["data"].(map[string]any)
	if data["postgresql"] != "up" || data["redis"] != "up" {
		t.Fatalf("data = %#v", data)
	}
}

func TestReadyReturnsUnifiedUnavailableError(t *testing.T) {
	service := &readinessService{err: apperror.DependencyUnavailable(errors.New("redis down"))}
	router := testRouter(service)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["code"] != float64(apperror.CodeDependencyUnavailable) || body["data"] != nil {
		t.Fatalf("body = %#v", body)
	}
}

func testRouter(service *readinessService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	health.RegisterRoutes(router, health.NewHandler(service))
	return router
}
