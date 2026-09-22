package setting

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestLegalDocumentRoutesSeparatePublicReadFromProtectedUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authenticated := 0
	checkedPermissions := make([]string, 0)
	authenticate := func(c *gin.Context) {
		authenticated++
		c.Next()
	}
	requirePermission := func(code string) gin.HandlerFunc {
		return func(c *gin.Context) {
			checkedPermissions = append(checkedPermissions, code)
			c.Next()
		}
	}
	service := detailHandlerService{legal: LegalDocument{
		Kind: LegalDocumentPrivacyPolicy, ContentHTML: "<p>Privacy</p>",
	}}
	router := gin.New()
	handler := NewHandler(service)
	RegisterPublicRoutes(router.Group("/api/v1"), handler)
	RegisterRoutes(router.Group("/api/admin/v1"), handler, authenticate, requirePermission)

	publicRecorder := httptest.NewRecorder()
	router.ServeHTTP(publicRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/system/setting/legal/privacyPolicy", nil))
	if publicRecorder.Code != http.StatusOK {
		t.Fatalf("public status=%d body=%s", publicRecorder.Code, publicRecorder.Body.String())
	}
	if authenticated != 0 || len(checkedPermissions) != 0 {
		t.Fatalf("public read used admin middleware: auth=%d permissions=%v", authenticated, checkedPermissions)
	}

	request := httptest.NewRequest(http.MethodPut, "/api/admin/v1/system/setting/legal/privacyPolicy", strings.NewReader(`{"contentHtml":"<p>Privacy</p>"}`))
	request.Header.Set("Content-Type", "application/json")
	protectedRecorder := httptest.NewRecorder()
	router.ServeHTTP(protectedRecorder, request)
	if protectedRecorder.Code != http.StatusOK {
		t.Fatalf("protected status=%d body=%s", protectedRecorder.Code, protectedRecorder.Body.String())
	}
	if authenticated != 1 || len(checkedPermissions) != 1 || checkedPermissions[0] != PermissionUpdate {
		t.Fatalf("protected middleware: auth=%d permissions=%v", authenticated, checkedPermissions)
	}
}
