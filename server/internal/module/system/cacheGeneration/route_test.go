package cachegeneration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRouteRequiresAuthenticationAndListPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authenticated := false
	checkedPermissions := make([]string, 0)
	authenticate := func(c *gin.Context) {
		authenticated = true
		c.Next()
	}
	requirePermission := func(code string) gin.HandlerFunc {
		return func(c *gin.Context) {
			checkedPermissions = append(checkedPermissions, code)
			c.Next()
		}
	}

	router := gin.New()
	RegisterRoutes(router.Group(""), NewHandler(&stubService{result: ListResult{Page: 1, PageSize: 20}}), authenticate, requirePermission)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/system/cachegeneration", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if !authenticated {
		t.Fatal("cache generation route must authenticate the current user")
	}
	if len(checkedPermissions) != 1 || checkedPermissions[0] != PermissionList {
		t.Fatalf("checked permissions = %v want [%s]", checkedPermissions, PermissionList)
	}
}
