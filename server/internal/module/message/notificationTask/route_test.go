package notificationtask

import (
	"github.com/gin-gonic/gin"
	"testing"
)

func TestRoutesBindIndependentDetailCreateAndUpdatePermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var codes []string
	mw := func(c *gin.Context) { c.Next() }
	require := func(code string) gin.HandlerFunc { codes = append(codes, code); return mw }
	r := gin.New()
	RegisterRoutes(r.Group("/api/admin/v1"), NewHandler(NewService(&taskRepositoryStub{})), mw, require)
	want := []string{PermissionList, PermissionCreate, PermissionUpdate, PermissionUpdate, PermissionDetail, PermissionCreate, PermissionUpdate, PermissionDelete, PermissionSubmit, PermissionCancel, PermissionCopy}
	if len(codes) != len(want) {
		t.Fatalf("codes=%v", codes)
	}
	for i := range want {
		if codes[i] != want[i] {
			t.Fatalf("codes=%v", codes)
		}
	}
	for _, path := range []string{"/api/admin/v1/message/notificationtask/create/option/:kind", "/api/admin/v1/message/notificationtask/update/option/:kind", "/api/admin/v1/message/notificationtask/:id/edit"} {
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
