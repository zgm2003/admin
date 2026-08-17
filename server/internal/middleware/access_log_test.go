package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	projectmiddleware "admin/server/internal/middleware"
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
