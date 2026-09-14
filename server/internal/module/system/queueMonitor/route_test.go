package queuemonitor

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterGrantRouteBindsListPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api/admin/v1")
	gotPermission := ""
	RegisterGrantRoute(group, NewHandler(nil, false, nil), func(c *gin.Context) { c.Next() }, func(c *gin.Context) { c.Next() }, func(code string) gin.HandlerFunc {
		gotPermission = code
		return func(c *gin.Context) { c.AbortWithStatus(http.StatusNoContent) }
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/admin/v1/system/queuemonitor/grant", nil))
	if gotPermission != PermissionList || recorder.Code != http.StatusNoContent {
		t.Fatalf("permission=%q status=%d", gotPermission, recorder.Code)
	}
}

func TestRegisterUIRoutesIncludesRootAndChildren(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterUIRoutes(router, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for _, target := range []string{UIPath, UIPath + "/", UIPath + "/api/queues"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("%s status=%d", target, recorder.Code)
		}
	}
}
