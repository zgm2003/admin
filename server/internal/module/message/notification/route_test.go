package notification

import (
	"github.com/gin-gonic/gin"
	"testing"
)

func TestRoutesBindExactNotificationPermissionsAndStaticReadAll(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var codes []string
	middleware := func(string) gin.HandlerFunc { return func(c *gin.Context) { c.Next() } }
	require := func(code string) gin.HandlerFunc { codes = append(codes, code); return middleware(code) }
	router := gin.New()
	RegisterRoutes(router.Group("/api/v1"), NewHandler(mailboxServiceStub{}), middleware("auth"), require)
	want := []string{PermissionList, PermissionList, PermissionRead, PermissionRead, PermissionDelete}
	if len(codes) != len(want) {
		t.Fatalf("codes=%v", codes)
	}
	for i := range want {
		if codes[i] != want[i] {
			t.Fatalf("codes=%v", codes)
		}
	}
	var readAll bool
	for _, route := range router.Routes() {
		if route.Path == "/api/v1/message/notification/read-all" && route.Method == "PATCH" {
			readAll = true
		}
	}
	if !readAll {
		t.Fatal("read-all static route missing")
	}
}
