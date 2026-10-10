package role_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"admin/server/internal/module/permission/role"

	"github.com/gin-gonic/gin"
)

func TestRoleFormOptionsIsAuthOnlyAndDependencyFree(t *testing.T) {
	router := gin.New()
	authenticated := false
	role.RegisterRoutes(router.Group("/api/admin/v1"), role.NewHandler(nil), func(ctx *gin.Context) { authenticated = true }, func(string) gin.HandlerFunc {
		return func(ctx *gin.Context) { ctx.AbortWithStatus(http.StatusForbidden) }
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/v1/permission/role/options", nil))
	if recorder.Code != http.StatusOK || !authenticated || !strings.Contains(recorder.Body.String(), `"codePattern"`) || !strings.Contains(recorder.Body.String(), `"nameMaxLength":64`) {
		t.Fatalf("options: %d %s", recorder.Code, recorder.Body.String())
	}
}
