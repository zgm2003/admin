package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	projectmiddleware "admin/server/internal/middleware"
	"github.com/gin-gonic/gin"
)

func TestCORSAllowsOnlyConfiguredOriginWithCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(projectmiddleware.CORS("http://localhost:16300"))
	router.GET("/", func(context *gin.Context) { context.Status(http.StatusNoContent) })

	allowed := httptest.NewRecorder()
	allowedRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	allowedRequest.Header.Set("Origin", "http://localhost:16300")
	router.ServeHTTP(allowed, allowedRequest)

	if got := allowed.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:16300" {
		t.Fatalf("allow origin = %q", got)
	}
	if got := allowed.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("allow credentials = %q", got)
	}

	denied := httptest.NewRecorder()
	deniedRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	deniedRequest.Header.Set("Origin", "https://example.com")
	router.ServeHTTP(denied, deniedRequest)
	if got := denied.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected allow origin = %q", got)
	}
}
