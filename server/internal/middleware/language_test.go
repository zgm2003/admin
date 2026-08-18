package middleware_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	projectmiddleware "admin/server/internal/middleware"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

func TestLanguageNegotiation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name           string
		acceptLanguage string
		wantLanguage   string
		wantMessage    string
	}{
		{name: "missing defaults to Chinese", wantLanguage: "zh-CN", wantMessage: "请求参数错误"},
		{name: "Chinese", acceptLanguage: "zh-CN", wantLanguage: "zh-CN", wantMessage: "请求参数错误"},
		{name: "English", acceptLanguage: "en-US", wantLanguage: "en-US", wantMessage: "Invalid request"},
		{name: "weighted English", acceptLanguage: "en-US,en;q=0.9", wantLanguage: "en-US", wantMessage: "Invalid request"},
		{name: "weighted first range", acceptLanguage: "en-US;q=0.8,zh-CN;q=0.7", wantLanguage: "en-US", wantMessage: "Invalid request"},
		{name: "unsupported returns Chinese error", acceptLanguage: "fr-FR", wantLanguage: "zh-CN", wantMessage: "请求参数错误"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(projectmiddleware.Language())
			router.GET("/", func(context *gin.Context) {
				response.Fail(context, apperror.InvalidRequest(errors.New("invalid test request")))
			})
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("Accept-Language", test.acceptLanguage)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
			if got := recorder.Header().Get("Content-Language"); got != test.wantLanguage {
				t.Fatalf("Content-Language = %q, want %q", got, test.wantLanguage)
			}
			var envelope map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(envelope) != 3 || envelope["code"] == float64(0) || envelope["data"] != nil || envelope["message"] != test.wantMessage {
				t.Fatalf("response = %#v", envelope)
			}
		})
	}
}

func TestLanguageRejectsAnUnsupportedFirstRangeBeforeTheHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	called := false
	router := gin.New()
	router.Use(projectmiddleware.Language())
	router.GET("/", func(context *gin.Context) {
		called = true
		context.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept-Language", "fr-FR,en-US;q=0.9")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if called {
		t.Fatal("handler ran for an unsupported first language range")
	}
	if recorder.Code != http.StatusBadRequest || recorder.Header().Get("Content-Language") != "zh-CN" {
		t.Fatalf("status = %d, Content-Language = %q", recorder.Code, recorder.Header().Get("Content-Language"))
	}
}
