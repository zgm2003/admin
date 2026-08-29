package authplatform_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/auth/client"
	"admin/server/internal/module/auth/platform"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

type platformHTTPService struct {
	policyCalls int
	policyCode  string
	listCalls   int
	listQuery   authplatform.ListQuery
	createCalls int
	updateCalls int
	statusCalls int
	deleteCalls int
}

func (s *platformHTTPService) CurrentPolicy(_ context.Context, code string) (authplatform.Policy, error) {
	s.policyCalls++
	s.policyCode = code
	return authplatform.Policy{ID: 1, Code: "admin", Name: "Admin", AllowRegister: true, IsEnabled: true}, nil
}

func (s *platformHTTPService) List(_ context.Context, query authplatform.ListQuery) (pagination.Result[authplatform.ListItem], error) {
	s.listCalls++
	s.listQuery = query
	return pagination.Result[authplatform.ListItem]{List: []authplatform.ListItem{{Platform: authplatform.Platform{
		ID: 1, Code: "admin", Name: "Admin", PolicyVersion: 1,
		AccessTTLSeconds: 900, RefreshTTLSeconds: 1209600,
		SessionCacheTTLSeconds: 1800, AccessCacheTTLSeconds: 1800,
		BindDevice: yesno.No, BindIP: yesno.No, MaxSessions: 1,
		AllowRegister: yesno.Yes, IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes,
		CreatedAt: time.Date(2026, 8, 20, 8, 0, 0, 123, time.UTC),
		UpdatedAt: time.Date(2026, 8, 20, 9, 0, 0, 456, time.UTC),
	}}}, Total: 1, Page: query.Page, PageSize: query.PageSize}, nil
}

func (*platformHTTPService) Deployment(context.Context) (authplatform.Deployment, error) {
	return authplatform.Deployment{CookieSecure: false, CORSOrigin: "http://localhost:16300", TrustedProxyMode: "none", TrustedProxyCount: 0, RedisStatus: "up"}, nil
}

func (s *platformHTTPService) Create(context.Context, authplatform.CreateInput) (int64, error) {
	s.createCalls++
	return 2, nil
}

func (s *platformHTTPService) Update(context.Context, int64, authplatform.UpdateInput) error {
	s.updateCalls++
	return nil
}

func (s *platformHTTPService) UpdateStatus(context.Context, int64, yesno.Value) error {
	s.statusCalls++
	return nil
}

func (s *platformHTTPService) Delete(context.Context, int64) error {
	s.deleteCalls++
	return nil
}

func TestPolicyReturnsOnlyThePublicContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &platformHTTPService{}
	router := gin.New()
	routes := router.Group("/api/v1", authclient.Require())
	authplatform.RegisterPublicRoutes(routes, authplatform.NewHandler(service))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/policy", nil)
	request.Header[authclient.PlatformHeader] = []string{"admin"}
	request.Header[authclient.DeviceIDHeader] = []string{"550e8400-e29b-41d4-a716-446655440000"}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.policyCalls != 1 || service.policyCode != "admin" {
		t.Fatalf("status=%d calls=%d code=%q body=%s", recorder.Code, service.policyCalls, service.policyCode, recorder.Body)
	}
	if got := recorder.Body.String(); got != `{"code":0,"data":{"code":"admin","name":"Admin","allowRegister":1},"message":"ok"}` {
		t.Fatalf("body = %s", got)
	}
}

func TestPolicyRejectsInvalidClientBeforeService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &platformHTTPService{}
	router := gin.New()
	routes := router.Group("/api/v1", authclient.Require())
	authplatform.RegisterPublicRoutes(routes, authplatform.NewHandler(service))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/auth/policy", nil))
	if recorder.Code != http.StatusBadRequest || service.policyCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, service.policyCalls, recorder.Body)
	}
}

func TestManagementListRequiresStrictQueryAndReturnsClosedPage(t *testing.T) {
	service, router := managementRouter(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/v1/auth-platforms?page=1&pageSize=20&keyword=admin&isEnabled=1", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.listCalls != 1 || service.listQuery.Page != 1 || service.listQuery.PageSize != 20 || service.listQuery.Keyword != "admin" || service.listQuery.IsEnabled == nil || *service.listQuery.IsEnabled != yesno.Yes {
		t.Fatalf("status=%d calls=%d query=%+v body=%s", recorder.Code, service.listCalls, service.listQuery, recorder.Body)
	}
	for _, forbidden := range []string{"deletedAt", "redisUrl", "postgres", "cookieSecure"} {
		if strings.Contains(recorder.Body.String(), forbidden) {
			t.Fatalf("list body leaks %q: %s", forbidden, recorder.Body)
		}
	}
	if !strings.Contains(recorder.Body.String(), `"list":[{`) || !strings.Contains(recorder.Body.String(), `"createdAt":"2026-08-20T08:00:00.000000123Z"`) {
		t.Fatalf("list body = %s", recorder.Body)
	}

	for _, path := range []string{
		"/api/admin/v1/auth-platforms?page=1",
		"/api/admin/v1/auth-platforms?page=1&page=2&pageSize=20",
		"/api/admin/v1/auth-platforms?page=1&pageSize=20&unknown=1",
		"/api/admin/v1/auth-platforms?page=0&pageSize=20",
		"/api/admin/v1/auth-platforms?page=1&pageSize=101",
	} {
		recorder = httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("GET %s status=%d body=%s", path, recorder.Code, recorder.Body)
		}
	}
}

func TestManagementJSONContractsRejectMissingAndUnknownFields(t *testing.T) {
	service, router := managementRouter(t)
	validCreate := `{"code":"app","name":"App","accessTTLSeconds":900,"refreshTTLSeconds":1209600,"sessionCacheTTLSeconds":1800,"accessCacheTTLSeconds":1800,"bindDevice":0,"bindIP":0,"maxSessions":1,"allowRegister":1,"isEnabled":1}`
	recorder := performJSON(router, http.MethodPost, "/api/admin/v1/auth-platforms", validCreate)
	if recorder.Code != http.StatusCreated || service.createCalls != 1 || recorder.Body.String() != `{"code":0,"data":{"id":2},"message":"ok"}` {
		t.Fatalf("create status=%d calls=%d body=%s", recorder.Code, service.createCalls, recorder.Body)
	}
	for _, body := range []string{
		`{"code":"app"}`,
		strings.TrimSuffix(validCreate, "}") + `,"unknown":1}`,
		strings.TrimSuffix(validCreate, "}") + `,"code":"other"}`,
		validCreate + `{}`,
	} {
		recorder = performJSON(router, http.MethodPost, "/api/admin/v1/auth-platforms", body)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("invalid create status=%d body=%s input=%s", recorder.Code, recorder.Body, body)
		}
	}

	validUpdate := `{"name":"App 2","accessTTLSeconds":900,"refreshTTLSeconds":1209600,"sessionCacheTTLSeconds":1800,"accessCacheTTLSeconds":1800,"bindDevice":1,"bindIP":0,"maxSessions":2,"allowRegister":0}`
	recorder = performJSON(router, http.MethodPut, "/api/admin/v1/auth-platforms/2", validUpdate)
	if recorder.Code != http.StatusOK || service.updateCalls != 1 {
		t.Fatalf("update status=%d calls=%d body=%s", recorder.Code, service.updateCalls, recorder.Body)
	}
	for _, extra := range []string{`"code":"changed"`, `"isBuiltin":1`, `"policyVersion":2`} {
		body := strings.TrimSuffix(validUpdate, "}") + "," + extra + "}"
		recorder = performJSON(router, http.MethodPut, "/api/admin/v1/auth-platforms/2", body)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("update with %s status=%d body=%s", extra, recorder.Code, recorder.Body)
		}
	}

	recorder = performJSON(router, http.MethodPatch, "/api/admin/v1/auth-platforms/2/status", `{"isEnabled":0}`)
	if recorder.Code != http.StatusOK || service.statusCalls != 1 {
		t.Fatalf("status update=%d calls=%d body=%s", recorder.Code, service.statusCalls, recorder.Body)
	}
	recorder = performJSON(router, http.MethodPatch, "/api/admin/v1/auth-platforms/2/status", `{"isEnabled":0,"name":"bad"}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status unknown field status=%d body=%s", recorder.Code, recorder.Body)
	}
}

func TestManagementDeleteRequiresEmptyBodyAndDeploymentIsSafe(t *testing.T) {
	service, router := managementRouter(t)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/api/admin/v1/auth-platforms/2", nil))
	if recorder.Code != http.StatusOK || service.deleteCalls != 1 {
		t.Fatalf("delete status=%d calls=%d body=%s", recorder.Code, service.deleteCalls, recorder.Body)
	}
	recorder = performJSON(router, http.MethodDelete, "/api/admin/v1/auth-platforms/2", `{}`)
	if recorder.Code != http.StatusBadRequest || service.deleteCalls != 1 {
		t.Fatalf("delete body status=%d calls=%d body=%s", recorder.Code, service.deleteCalls, recorder.Body)
	}

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/v1/auth-platforms/deployment", nil))
	want := `{"code":0,"data":{"cookieSecure":false,"corsOrigin":"http://localhost:16300","trustedProxyMode":"none","trustedProxyCount":0,"redisStatus":"up"},"message":"ok"}`
	if recorder.Code != http.StatusOK || recorder.Body.String() != want {
		t.Fatalf("deployment status=%d body=%s", recorder.Code, recorder.Body)
	}
}

func TestManagementRoutesUseAuthenticationPlatformPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	permissions := make([]string, 0)
	pass := func(context *gin.Context) { context.Next() }
	authplatform.RegisterManagementRoutes(router.Group("/api/admin/v1"), authplatform.NewHandler(&platformHTTPService{}), pass, func(code string) gin.HandlerFunc {
		permissions = append(permissions, code)
		return pass
	})
	want := []string{
		authplatform.PermissionList,
		authplatform.PermissionList,
		authplatform.PermissionCreate,
		authplatform.PermissionUpdate,
		authplatform.PermissionStatus,
		authplatform.PermissionDelete,
	}
	if !reflect.DeepEqual(permissions, want) {
		t.Fatalf("permissions = %v, want %v", permissions, want)
	}
}

func managementRouter(t *testing.T) (*platformHTTPService, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	service := &platformHTTPService{}
	router := gin.New()
	routes := router.Group("/api/admin/v1")
	authplatform.RegisterManagementRoutes(routes, authplatform.NewHandler(service), func(context *gin.Context) { context.Next() }, func(string) gin.HandlerFunc {
		return func(context *gin.Context) { context.Next() }
	})
	return service, router
}

func performJSON(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}
