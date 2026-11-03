package agent

import (
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateRejectsActionFieldsBeforeService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{`{"status":1}`, `{"activeBindingId":"2"}`, `{"actorId":1}`} {
		r := gin.New()
		RegisterAdminRoutes(r.Group(""), NewHandler(nil), func(c *gin.Context) { c.Next() }, func(string) gin.HandlerFunc { return func(c *gin.Context) { c.Next() } })
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/ai/agent/1", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d", body, w.Code)
		}
	}
}
func TestRoutesRequireIndependentActionAndOptionsOnlyAuthenticate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct{ method, path, action string }{{"GET", "/ai/agent", PermissionList}, {"GET", "/ai/agent/1", PermissionDetail}, {"POST", "/ai/agent", PermissionCreate}, {"PUT", "/ai/agent/1", PermissionUpdate}, {"PATCH", "/ai/agent/1/status", PermissionStatus}, {"PATCH", "/ai/agent/1/binding/current", PermissionSwitchBinding}, {"DELETE", "/ai/agent/1", PermissionDelete}} {
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
}
