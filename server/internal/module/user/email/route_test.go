package email

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterRoutesRequiresAuthenticationAndExactEmailPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	seen := make([]string, 0, 3)
	router := gin.New()
	RegisterRoutes(router.Group("/api/admin/v1"), &Handler{}, func(c *gin.Context) {
		seen = append(seen, "authenticate")
		c.Next()
	}, func(code string) gin.HandlerFunc {
		return func(c *gin.Context) {
			seen = append(seen, "permission:"+code)
			c.AbortWithStatus(http.StatusNoContent)
		}
	})

	for _, request := range []struct{ method, path string }{
		{http.MethodPost, "/api/admin/v1/user/email/send-code"},
		{http.MethodPut, "/api/admin/v1/user/email"},
		{http.MethodGet, "/api/admin/v1/user/account/7/email-change-log?page=1&pageSize=20"},
	} {
		seen = seen[:0]
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(request.method, request.path, nil))
		want := PermissionUpdate
		if request.method == http.MethodGet {
			want = PermissionDetail
		}
		if recorder.Code != http.StatusNoContent || len(seen) != 2 || seen[0] != "authenticate" || seen[1] != "permission:"+want {
			t.Fatalf("%s %s status=%d middleware=%v", request.method, request.path, recorder.Code, seen)
		}
	}
}
