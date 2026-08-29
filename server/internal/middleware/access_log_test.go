package middleware_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	projectmiddleware "admin/server/internal/middleware"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

func TestAccessLogContainsOperationalFieldsButNoSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	router := gin.New()
	router.Use(projectmiddleware.RequestID(), projectmiddleware.AccessLog(logger))
	router.POST("/secrets", func(context *gin.Context) { context.Status(http.StatusNoContent) })
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/secrets", strings.NewReader(`{"password":"do-not-log"}`))
	request.Header.Set("Authorization", "Bearer do-not-log")
	request.Header.Set(projectmiddleware.RequestIDHeader, "request-123")

	router.ServeHTTP(recorder, request)

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("decode log: %v; output=%s", err, output.String())
	}
	for key, want := range map[string]any{
		"requestId": "request-123",
		"method":    http.MethodPost,
		"route":     "/secrets",
		"status":    float64(http.StatusNoContent),
	} {
		if entry[key] != want {
			t.Fatalf("log[%q] = %#v, want %#v", key, entry[key], want)
		}
	}
	if _, ok := entry["latencyMs"]; !ok {
		t.Fatalf("log has no latencyMs: %#v", entry)
	}
	if strings.Contains(output.String(), "do-not-log") || strings.Contains(output.String(), "Authorization") || strings.Contains(output.String(), "password") {
		t.Fatalf("log leaked request data: %s", output.String())
	}
}

func TestAccessLogContainsSafeAuthenticationAndCacheFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	router := gin.New()
	router.Use(projectmiddleware.AccessLog(logger))
	router.GET("/access", func(context *gin.Context) {
		projectmiddleware.SetAuthenticationLog(context, 17, "admin", 7, 11)
		projectmiddleware.SetCacheLog(context, "accessSnapshot", "hit", 4)
		context.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/access", nil)
	request.Header.Set("Authorization", "Bearer access-secret")
	request.Header.Set("Cookie", "admin_refresh_admin=refresh-secret")
	request.Header.Set("X-Device-ID", "550e8400-e29b-41d4-a716-446655440000")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]any{
		"authPlatform": "admin", "userID": float64(7), "sessionID": float64(11),
		"cacheKind": "accessSnapshot", "cacheResult": "hit", "accessVersion": float64(4),
	} {
		if entry[key] != want {
			t.Fatalf("log[%s]=%#v want %#v; entry=%v", key, entry[key], want, entry)
		}
	}
	for _, forbidden := range []string{"access-secret", "refresh-secret", "550e8400-e29b-41d4-a716-446655440000", "Authorization", "Cookie", "X-Device-ID"} {
		if strings.Contains(output.String(), forbidden) {
			t.Fatalf("log leaked %q: %s", forbidden, output.String())
		}
	}
}

func TestAccessLogContainsInternalCauseWithoutLeakingItInResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	router := gin.New()
	router.Use(projectmiddleware.RequestID(), projectmiddleware.AccessLog(logger))
	router.GET("/failure", func(context *gin.Context) {
		response.Fail(context, apperror.Internal(errors.New("database password leaked")))
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/failure", nil)
	request.Header.Set(projectmiddleware.RequestIDHeader, "request-error")
	router.ServeHTTP(recorder, request)

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("decode log: %v; output=%s", err, output.String())
	}
	if entry["requestId"] != "request-error" || entry["errorCode"] != float64(apperror.CodeInternal) || entry["error"] != "database password leaked" {
		t.Fatalf("log entry = %#v, want request ID, error code, and cause", entry)
	}
	if strings.Contains(recorder.Body.String(), "database password leaked") {
		t.Fatal("response leaked internal cause")
	}
}

func TestAccessLogContainsExplicitUserMutationContextAndBusinessCause(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	router := gin.New()
	router.Use(projectmiddleware.RequestID(), projectmiddleware.AccessLog(logger))
	router.PUT("/users/:id", func(context *gin.Context) {
		projectmiddleware.SetAccessLogOperation(context, "user.username.update", 41, 7)
		response.Fail(context, &apperror.Error{HTTPStatus: http.StatusConflict, Code: 16001, MessageKey: "user.usernameConflict", Cause: errors.New("constraint detail")})
	})
	request := httptest.NewRequest(http.MethodPut, "/users/7", strings.NewReader(`{"password":"secret"}`))
	request.Header.Set(projectmiddleware.RequestIDHeader, "request-user-mutation")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]any{"requestId": "request-user-mutation", "operation": "user.username.update", "actorUserId": float64(41), "targetUserId": float64(7), "errorCode": float64(16001), "error": "constraint detail"} {
		if entry[key] != want {
			t.Fatalf("log[%s]=%#v want %#v; entry=%v", key, entry[key], want, entry)
		}
	}
	if strings.Contains(output.String(), "secret") || strings.Contains(output.String(), "password") || strings.Contains(recorder.Body.String(), "constraint detail") {
		t.Fatalf("sensitive data leaked: log=%s response=%s", output.String(), recorder.Body.String())
	}
}
