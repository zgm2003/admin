package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	authclient "admin/server/internal/module/auth/client"
	useremail "admin/server/internal/module/user/email"
	userphone "admin/server/internal/module/user/phone"
	"admin/server/internal/module/user/profile"
	"github.com/gin-gonic/gin"
)

func TestCanvasPersonalRoutesRequireExactActions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	seen := []string{}
	router := buildRouter(routerDependencies{
		CORSOrigin: "http://localhost:16300",
		AuthOrigin: func(c *gin.Context) { c.Next() },
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Account:    &profile.Handler{}, Email: &useremail.Handler{}, Phone: &userphone.Handler{},
		Authenticate: func(c *gin.Context) { seen = append(seen, "authenticate"); c.Next() },
		RequirePermission: func(code string) gin.HandlerFunc {
			return func(c *gin.Context) { seen = append(seen, code); c.AbortWithStatus(http.StatusNoContent) }
		},
	})
	for _, route := range []struct{ method, path, code string }{
		{"GET", "/api/canvas/v1/user/profile", "user:profile:detail"},
		{"PUT", "/api/canvas/v1/user/profile", "user:profile:update"},
		{"POST", "/api/canvas/v1/user/password", "user:password:update"},
		{"POST", "/api/canvas/v1/user/password/set", "user:password:update"},
		{"POST", "/api/canvas/v1/user/password/send-code", "user:password:update"},
		{"PUT", "/api/canvas/v1/user/password/by-code", "user:password:update"},
		{"POST", "/api/canvas/v1/user/email/send-code", "user:email:update"},
		{"PUT", "/api/canvas/v1/user/email", "user:email:update"},
		{"POST", "/api/canvas/v1/user/phone/send-code", "user:phone:update"},
		{"PUT", "/api/canvas/v1/user/phone", "user:phone:update"},
	} {
		seen = seen[:0]
		req := httptest.NewRequest(route.method, route.path, nil)
		req.Header.Set(authclient.PlatformHeader, "canvas")
		req.Header.Set(authclient.DeviceIDHeader, "550e8400-e29b-41d4-a716-446655440000")
		out := httptest.NewRecorder()
		router.ServeHTTP(out, req)
		if out.Code != http.StatusNoContent || len(seen) != 2 || seen[0] != "authenticate" || seen[1] != route.code {
			t.Fatalf("%s %s status=%d middleware=%v", route.method, route.path, out.Code, seen)
		}
	}
	for _, path := range []string{"/api/canvas/v1/user/account/7/email-change-log", "/api/canvas/v1/user/account/7/phone-change-log", "/api/canvas/v1/user/account"} {
		out := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set(authclient.PlatformHeader, "canvas")
		req.Header.Set(authclient.DeviceIDHeader, "550e8400-e29b-41d4-a716-446655440000")
		router.ServeHTTP(out, req)
		if out.Code != 404 {
			t.Fatalf("management endpoint exposed at %s: %d", path, out.Code)
		}
	}
}
