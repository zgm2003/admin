package notificationtask

import (
	"github.com/gin-gonic/gin"
	"testing"
)

func TestRoutesBindExactPermissionsAndStaticOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var codes []string
	mw := func(c *gin.Context) { c.Next() }
	require := func(code string) gin.HandlerFunc { codes = append(codes, code); return mw }
	r := gin.New()
	RegisterRoutes(r.Group("/api/admin/v1"), NewHandler(NewService(&taskRepositoryStub{})), mw, require)
	want := []string{PermissionList, PermissionDetail, PermissionDetail, PermissionDetail, PermissionDetail, PermissionCreate, PermissionUpdate, PermissionDelete, PermissionSubmit, PermissionCancel, PermissionCopy}
	if len(codes) != len(want) {
		t.Fatalf("codes=%v", codes)
	}
	for i := range want {
		if codes[i] != want[i] {
			t.Fatalf("codes=%v", codes)
		}
	}
	for _, path := range []string{"/api/admin/v1/message/notificationtask/platform-option", "/api/admin/v1/message/notificationtask/user-option", "/api/admin/v1/message/notificationtask/role-option"} {
		found := false
		for _, route := range r.Routes() {
			if route.Path == path {
				found = true
			}
		}
		if !found {
			t.Fatalf("route %s missing", path)
		}
	}
}
