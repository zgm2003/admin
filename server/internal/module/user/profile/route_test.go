package profile

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterRoutesRequiresProfilePermissions(t *testing.T) {
	if PermissionList != "account:profile:list" || PermissionUpdate != "account:profile:update" || PermissionPasswordUpdate != "account:password:update" {
		t.Fatalf("profile permissions = %q, %q, %q", PermissionList, PermissionUpdate, PermissionPasswordUpdate)
	}

	seen := make([]string, 0, 2)
	router := gin.New()
	RegisterRoutes(
		router.Group("/api/admin/v1"),
		&Handler{},
		func(context *gin.Context) {
			seen = append(seen, "authenticate")
			context.Next()
		},
		func(code string) gin.HandlerFunc {
			return func(context *gin.Context) {
				seen = append(seen, "permission:"+code)
				context.AbortWithStatus(http.StatusNoContent)
			}
		},
	)

	want := []struct {
		method     string
		path       string
		permission string
	}{
		{http.MethodGet, "/api/admin/v1/account/profile", PermissionList},
		{http.MethodPut, "/api/admin/v1/account/profile", PermissionUpdate},
		{http.MethodPost, "/api/admin/v1/account/password", PermissionPasswordUpdate},
	}
	for _, request := range want {
		seen = seen[:0]
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(request.method, request.path, nil))
		wantOrder := []string{"authenticate", "permission:" + request.permission}
		if len(seen) != len(wantOrder) || seen[0] != wantOrder[0] || seen[1] != wantOrder[1] {
			t.Fatalf("%s %s middleware = %v, want %v", request.method, request.path, seen, wantOrder)
		}
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("%s %s status = %d, want %d", request.method, request.path, recorder.Code, http.StatusNoContent)
		}
	}
}
