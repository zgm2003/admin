package setting

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBrandDisplayRouteRequiresAuthenticationWithoutSettingPermission(t *testing.T) {
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
	RegisterRoutes(router.Group(""), NewHandler(detailHandlerService{brand: BrandSettings{
		TitleZhCN: "智澜", TitleEnUS: "ZHILAN",
	}}), authenticate, requirePermission)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/system/setting/brand", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !authenticated {
		t.Fatal("brand display route must authenticate the current user")
	}
	if len(checkedPermissions) != 0 {
		t.Fatalf("brand display route checked permissions %v", checkedPermissions)
	}
}
