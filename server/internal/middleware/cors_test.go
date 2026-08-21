package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
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
	if got := allowed.Header().Get("Access-Control-Expose-Headers"); !headerListContains(got, "Content-Language") {
		t.Fatalf("expose headers = %q, want Content-Language", got)
	}

	denied := httptest.NewRecorder()
	deniedRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	deniedRequest.Header.Set("Origin", "https://example.com")
	router.ServeHTTP(denied, deniedRequest)
	if got := denied.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected allow origin = %q", got)
	}
}

func TestCORSAllowsAuthenticationHeadersAndPatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(projectmiddleware.CORS("http://localhost:16300"))
	router.PATCH("/", func(context *gin.Context) { context.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/", nil)
	request.Header.Set("Origin", "http://localhost:16300")
	request.Header.Set("Access-Control-Request-Method", http.MethodPatch)
	request.Header.Set("Access-Control-Request-Headers", "accept-language,x-auth-platform,x-device-id")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Methods"); !headerListContains(got, http.MethodPatch) {
		t.Fatalf("allow methods = %q, want PATCH", got)
	}
	for _, header := range []string{"Accept-Language", "X-Auth-Platform", "X-Device-ID"} {
		if got := recorder.Header().Get("Access-Control-Allow-Headers"); !headerListContains(got, header) {
			t.Fatalf("allow headers = %q, want %s", got, header)
		}
	}
}

func TestCORSRejectsUnlistedRequestHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(projectmiddleware.CORS("http://localhost:16300"))
	router.GET("/", func(context *gin.Context) { context.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/", nil)
	request.Header.Set("Origin", "http://localhost:16300")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	request.Header.Set("Access-Control-Request-Headers", "x-unlisted-header")
	router.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Headers"); headerListContains(got, "X-Unlisted-Header") {
		t.Fatalf("allow headers = %q, must not contain X-Unlisted-Header", got)
	}
}

func headerListContains(value, want string) bool {
	for _, item := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(item), want) {
			return true
		}
	}
	return false
}
