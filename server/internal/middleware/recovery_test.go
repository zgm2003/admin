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

func TestRecoveryUsesUnifiedEnvelopeAndLogsPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	router := gin.New()
	router.Use(projectmiddleware.RequestID(), projectmiddleware.AccessLog(logger), projectmiddleware.Recovery(logger))
	router.GET("/panic", func(*gin.Context) {
		panic("panic-check")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	request.Header.Set(projectmiddleware.RequestIDHeader, "request-panic")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	var envelope map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := map[string]any{"code": float64(10000), "data": nil, "message": "服务内部错误"}
	if len(envelope) != len(want) || envelope["code"] != want["code"] || envelope["data"] != want["data"] || envelope["message"] != want["message"] {
		t.Fatalf("response = %#v, want %#v", envelope, want)
	}
	if strings.Contains(recorder.Body.String(), "panic-check") {
		t.Fatal("response leaked panic details")
	}

	var panicLog, accessLog map[string]any
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("decode log: %v; line=%s", err, line)
		}
		switch entry["msg"] {
		case "panic recovered":
			panicLog = entry
		case "http request":
			accessLog = entry
		}
	}
	if panicLog["requestId"] != "request-panic" || panicLog["panic"] != "panic-check" || panicLog["stack"] == "" {
		t.Fatalf("panic log = %#v, want request ID, panic, and stack", panicLog)
	}
	if accessLog["requestId"] != "request-panic" || accessLog["status"] != float64(http.StatusInternalServerError) || accessLog["errorCode"] != float64(10000) {
		t.Fatalf("access log = %#v, want request ID, status, and error code", accessLog)
	}
}
