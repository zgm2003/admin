package account_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"admin/server/internal/module/user/account"

	"github.com/gin-gonic/gin"
)

func TestAccountFormOptionsIsAuthOnlyAndDependencyFree(t *testing.T) {
	router := gin.New()
	authenticated := false
	account.RegisterRoutes(router.Group("/api/admin/v1"), account.NewHandler(nil, nil), func(ctx *gin.Context) { authenticated = true }, func(string) gin.HandlerFunc {
		return func(ctx *gin.Context) { ctx.AbortWithStatus(http.StatusForbidden) }
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/v1/user/account/options", nil))
	if recorder.Code != http.StatusOK || !authenticated || !strings.Contains(recorder.Body.String(), `"usernameMinLength":3`) || !strings.Contains(recorder.Body.String(), `"usernameMaxLength":64`) {
		t.Fatalf("options: %d %s", recorder.Code, recorder.Body.String())
	}
}
