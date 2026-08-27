package operationlog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
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
	if rule, ok := FindRule(http.MethodPut, "/api/admin/v1/users/:id"); !ok || rule.Action != "user.update" {
		t.Fatalf("user update rule = %+v,%v", rule, ok)
	}
	if _, ok := FindRule(http.MethodPut, "/api/v1/users/:id"); ok {
		t.Fatal("legacy user update rule remains registered")
	}
	if _, ok := FindRule(http.MethodPost, "/api/v1/auth/login"); !ok {
		t.Fatal("shared login audit rule is missing")
	}
	if rule, ok := FindRule(http.MethodDelete, "/api/admin/v1/sessions/:id"); !ok || rule.Action != "session.revoke" {
		t.Fatalf("session revoke rule = %+v,%v", rule, ok)
	}
	for _, route := range []string{"/api/admin/v1/users", "/api/v1/access", "/api/admin/v1/users/:id/roles"} {
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
	router.PUT("/api/admin/v1/users/:id", func(context *gin.Context) {
		context.JSON(http.StatusCreated, gin.H{"code": 0, "data": gin.H{"id": 7}, "message": "ok"})
	})

	request := httptest.NewRequest(http.MethodPut, "/api/admin/v1/users/7", strings.NewReader(`{"password":"should-not-leak"}`))
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

func TestMiddlewareGeneratesDistinctEventIDsForRepeatedRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enqueuer := &failingEnqueuer{}
	router := gin.New()
	router.Use(projectmiddleware.RequestID(), Middleware(slog.Default(), enqueuer))
	router.PUT("/api/admin/v1/users/:id", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{"code": 0, "data": nil, "message": "ok"})
	})

	for _, id := range []string{"7", "8"} {
		request := httptest.NewRequest(http.MethodPut, "/api/admin/v1/users/"+id, strings.NewReader(`{"username":"member"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set(projectmiddleware.RequestIDHeader, "client-reused-request")
		router.ServeHTTP(httptest.NewRecorder(), request)
	}

	if len(enqueuer.payloads) != 2 {
		t.Fatalf("payload count = %d, want 2", len(enqueuer.payloads))
	}
	first, second := enqueuer.payloads[0], enqueuer.payloads[1]
	if first.RequestID != "client-reused-request" || second.RequestID != first.RequestID {
		t.Fatalf("request IDs = %q, %q", first.RequestID, second.RequestID)
	}
	if first.EventID == "" || second.EventID == "" || first.EventID == second.EventID {
		t.Fatalf("event IDs = %q, %q", first.EventID, second.EventID)
	}
}

func TestSanitizeSummaryLimitReturnsBoundedValidJSON(t *testing.T) {
	raw := []byte(`{"visible":"` + strings.Repeat("界", maxSummaryBytes) + `"}`)
	summary, err := SanitizeJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary) > maxSummaryBytes || !json.Valid(summary) {
		t.Fatalf("summary length = %d, valid = %v", len(summary), json.Valid(summary))
	}
	if string(summary) != `{"truncated":true}` {
		t.Fatalf("summary = %s", summary)
	}
}

func TestSummaryWriterBoundsCapturedResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	writer := &summaryWriter{ResponseWriter: context.Writer}
	body := []byte(strings.Repeat("x", maxSummaryBytes*3))
	written, err := writer.Write(body)
	if err != nil {
		t.Fatal(err)
	}
	if written != len(body) || writer.body.Len() > maxSummaryBytes || !writer.truncated {
		t.Fatalf("written=%d captured=%d truncated=%v", written, writer.body.Len(), writer.truncated)
	}
}

func TestReadRequestSummaryMarksLargeBodyAndPreservesRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := []byte(`{"visible":"` + strings.Repeat("x", maxSummaryBytes) + `"}`)
	request := httptest.NewRequest(http.MethodPut, "/api/admin/v1/users/7", bytes.NewReader(original))
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	summary := readRequestSummary(context)
	if string(summary) != `{"truncated":true}` || len(summary) > maxSummaryBytes {
		t.Fatalf("summary = %s", summary)
	}
	restored, err := io.ReadAll(context.Request.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, original) {
		t.Fatalf("restored request length = %d, want %d", len(restored), len(original))
	}
}

func TestMiddlewareSanitizesCapturedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enqueuer := &failingEnqueuer{}
	router := gin.New()
	router.Use(projectmiddleware.RequestID(), Middleware(slog.Default(), enqueuer))
	router.PUT("/api/admin/v1/users/:id", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"secretKey": "must-not-leak"}, "message": "ok"})
	})
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/api/admin/v1/users/7", strings.NewReader(`{"username":"member"}`)))

	if len(enqueuer.payloads) != 1 {
		t.Fatalf("payload count = %d", len(enqueuer.payloads))
	}
	response := string(enqueuer.payloads[0].ResponseData)
	if strings.Contains(response, "must-not-leak") || !strings.Contains(response, `"secretKey":"***"`) {
		t.Fatalf("response summary = %s", response)
	}
}
