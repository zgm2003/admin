package agent

import (
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestDynamicCandidateRoutesKeepCreateAndUpdatePermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct{ path, permission string }{{"/ai/agent/candidate/create", PermissionCreate}, {"/ai/agent/1/candidate/update", PermissionUpdate}} {
		r := gin.New()
		seen := ""
		RegisterAdminRoutes(r.Group(""), NewHandler(nil), func(c *gin.Context) { c.Next() }, func(p string) gin.HandlerFunc {
			return func(c *gin.Context) { seen = p; response.Fail(c, apperror.Forbidden(nil)) }
		})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", tc.path+"?kind=tool", nil))
		if seen != tc.permission || w.Code != 403 {
			t.Fatalf("candidate action=%s status=%d", seen, w.Code)
		}
	}
}
