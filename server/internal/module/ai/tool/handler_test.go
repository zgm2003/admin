package tool

import (
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutesRequireIndependentActionsAndExposeNoExecuteEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct{ method, path, action string }{{"GET", "/ai/tool", PermissionList}, {"GET", "/ai/tool/1", PermissionDetail}, {"POST", "/ai/tool", PermissionCreate}, {"PUT", "/ai/tool/1", PermissionUpdate}, {"PATCH", "/ai/tool/1/status", PermissionStatus}, {"DELETE", "/ai/tool/1", PermissionDelete}} {
		r := gin.New()
		seen := ""
		RegisterAdminRoutes(r.Group(""), NewHandler(nil), func(c *gin.Context) { c.Next() }, func(code string) gin.HandlerFunc {
			return func(c *gin.Context) { seen = code; response.Fail(c, apperror.Forbidden(nil)) }
		})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if seen != tc.action || w.Code != 403 {
			t.Fatalf("route %s action=%s code=%d", tc.path, seen, w.Code)
		}
	}
	r := gin.New()
	RegisterAdminRoutes(r.Group(""), NewHandler(nil), func(c *gin.Context) { c.Next() }, func(code string) gin.HandlerFunc { return func(c *gin.Context) { c.Next() } })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/ai/tool/1/execute", nil))
	if w.Code != 404 {
		t.Fatal("general tool execute route exposed")
	}
}
func TestUpdateRejectsStatusAndCiphertextAndActorInjection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{`{"status":1}`, `{"actorId":1}`, `{"version":{"secretCiphertext":"forged"}}`} {
		r := gin.New()
		RegisterAdminRoutes(r.Group(""), NewHandler(nil), func(c *gin.Context) { c.Next() }, func(code string) gin.HandlerFunc { return func(c *gin.Context) { c.Next() } })
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("PUT", "/ai/tool/1", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatalf("injected body %s: %d", body, w.Code)
		}
	}
}
