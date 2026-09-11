package phone

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type routeHandlerStub struct{}

func (routeHandlerStub) SendCode(*gin.Context)     {}
func (routeHandlerStub) BindOrChange(*gin.Context) {}

func TestRegisterRoutesRequiresAuthenticationAndExactPhonePermission(t *testing.T) {
	seen := make([]string, 0, 2)
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
		{http.MethodPost, "/api/admin/v1/user/phone/send-code"},
		{http.MethodPut, "/api/admin/v1/user/phone"},
	} {
		seen = seen[:0]
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(request.method, request.path, nil))
		if recorder.Code != http.StatusNoContent || len(seen) != 2 || seen[0] != "authenticate" || seen[1] != "permission:"+PermissionUpdate {
			t.Fatalf("%s %s status=%d middleware=%v", request.method, request.path, recorder.Code, seen)
		}
	}
}
