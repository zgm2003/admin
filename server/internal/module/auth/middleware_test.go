package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"admin/server/internal/shared/apperror"
	"github.com/gin-gonic/gin"
)

func TestRequireOriginAcceptsOnlyExactConfiguredOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		origin string
		want   int
	}{
		{origin: "http://localhost:16300", want: http.StatusNoContent},
		{origin: "http://localhost:16300/", want: http.StatusForbidden},
		{origin: "http://evil.example", want: http.StatusForbidden},
		{origin: "", want: http.StatusForbidden},
	} {
		router := gin.New()
		router.Use(RequireOrigin("http://localhost:16300"))
		router.POST("/", func(context *gin.Context) { context.Status(http.StatusNoContent) })
		request := httptest.NewRequest(http.MethodPost, "/", nil)
		if test.origin != "" {
			request.Header.Set("Origin", test.origin)
		}
		responseRecorder := httptest.NewRecorder()
		router.ServeHTTP(responseRecorder, request)
		if responseRecorder.Code != test.want {
			t.Errorf("origin %q status = %d body=%s", test.origin, responseRecorder.Code, responseRecorder.Body.String())
		}
	}
}

func TestAuthenticateMiddlewareRequiresBearerToken(t *testing.T) {
	for _, header := range []string{"", "Basic abc", "Bearer", "Bearer ", "Bearer one two"} {
		service := &stubAuthenticationService{}
		responseRecorder := serveAuthenticatedRequest(t, service, header, nil)
		if responseRecorder.Code != http.StatusUnauthorized {
			t.Errorf("header %q status = %d", header, responseRecorder.Code)
		}
		if service.authenticateCalls != 0 {
			t.Errorf("header %q called service", header)
		}
	}
}

func TestAuthenticateMiddlewareStoresIdentity(t *testing.T) {
	want := Identity{UserID: 1, SessionID: 2, Version: 3}
	service := &stubAuthenticationService{authenticateIdentity: want}
	responseRecorder := serveAuthenticatedRequest(t, service, "Bearer token", func(context *gin.Context) {
		got, ok := IdentityFromContext(context)
		if !ok || got != want {
			t.Fatalf("IdentityFromContext() = %+v,%v", got, ok)
		}
		context.Status(http.StatusNoContent)
	})
	if responseRecorder.Code != http.StatusNoContent || service.authenticateToken != "token" {
		t.Fatalf("status=%d token=%q", responseRecorder.Code, service.authenticateToken)
	}
}

func TestAuthenticateMiddlewarePreservesDependencyUnavailable(t *testing.T) {
	service := &stubAuthenticationService{authenticateErr: apperror.DependencyUnavailable(errors.New("redis down"))}
	responseRecorder := serveAuthenticatedRequest(t, service, "Bearer token", nil)
	assertEnvelopeKeysAndCode(t, responseRecorder, http.StatusServiceUnavailable, apperror.CodeDependencyUnavailable, nil)
}

func serveAuthenticatedRequest(t *testing.T, service *stubAuthenticationService, authorization string, next gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if next == nil {
		next = func(context *gin.Context) { context.Status(http.StatusNoContent) }
	}
	router := gin.New()
	router.Use(Authenticate(service))
	router.GET("/", next)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	responseRecorder := httptest.NewRecorder()
	router.ServeHTTP(responseRecorder, request)
	return responseRecorder
}

var _ authenticationService = (*stubAuthenticationService)(nil)
var _ = context.Background
