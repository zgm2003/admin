package operationlog

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	projectmiddleware "admin/server/internal/middleware"
	"github.com/gin-gonic/gin"
)

type failingEnqueuer struct {
	payloads []TaskPayload
}

func (e *failingEnqueuer) Enqueue(_ context.Context, payload TaskPayload) error {
	e.payloads = append(e.payloads, payload)
	return errors.New("Redis is unavailable")
}

func TestRulesMatchOnlyExplicitMutations(t *testing.T) {
	if rule, ok := FindRule(http.MethodPut, "/api/v1/users/:id"); !ok || rule.Action != "user.update" {
		t.Fatalf("user update rule = %+v,%v", rule, ok)
	}
	if rule, ok := FindRule(http.MethodDelete, "/api/v1/sessions/:id"); !ok || rule.Action != "session.revoke" {
		t.Fatalf("session revoke rule = %+v,%v", rule, ok)
	}
	for _, route := range []string{"/api/v1/users", "/api/v1/access", "/api/v1/users/:id/roles"} {
		if _, ok := FindRule(http.MethodGet, route); ok {
			t.Fatalf("read route %s was registered as operation", route)
		}
	}
}

func TestSanitizeSummaryMasksSecrets(t *testing.T) {
	raw := []byte(`{"password":"p","confirmPassword":"cp","accessToken":"at","refreshToken":"rt","authorization":"a","cookie":"c","secret":"s","key":"k","visible":"v","nested":[{"password":"nested"}]}`)
	summary, err := SanitizeJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	text := string(summary)
	for _, value := range []string{`"password":"p"`, `"confirmPassword":"cp"`, `"accessToken":"at"`, `"refreshToken":"rt"`, `"authorization":"a"`, `"cookie":"c"`, `"secret":"s"`, `"key":"k"`} {
		if strings.Contains(text, value) {
			t.Fatalf("sensitive value %s leaked in %s", value, text)
		}
	}
	if !strings.Contains(text, `"visible":"v"`) || !strings.Contains(text, `"password":"***"`) {
		t.Fatalf("sanitized summary = %s", text)
	}
}

func TestMiddlewareKeepsBusinessStatusWhenEnqueueFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	enqueuer := &failingEnqueuer{}
	router := gin.New()
	router.Use(projectmiddleware.RequestID(), Middleware(logger, enqueuer))
	router.PUT("/api/v1/users/:id", func(context *gin.Context) {
		context.JSON(http.StatusCreated, gin.H{"code": 0, "data": gin.H{"id": 7}, "message": "ok"})
	})

	request := httptest.NewRequest(http.MethodPut, "/api/v1/users/7", strings.NewReader(`{"password":"should-not-leak"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
	if !strings.Contains(recorder.Body.String(), `"code":0`) {
		t.Fatalf("business response changed: %s", recorder.Body.String())
	}
	if len(enqueuer.payloads) != 1 {
		t.Fatalf("payload count = %d, want 1", len(enqueuer.payloads))
	}
	if strings.Contains(string(enqueuer.payloads[0].RequestData), "should-not-leak") {
		t.Fatalf("sensitive request value leaked: %s", enqueuer.payloads[0].RequestData)
	}
	for _, fragment := range []string{"requestId", "route", "action", "enqueue operation log"} {
		if !strings.Contains(logs.String(), fragment) {
			t.Fatalf("logs = %s, missing %q", logs.String(), fragment)
		}
	}
}
