package access_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"admin/server/internal/module/auth/client"
	"admin/server/internal/module/auth/login"
	"admin/server/internal/module/rbac/access"
	"admin/server/internal/module/user/account"
	"admin/server/internal/shared/apperror"
	"github.com/gin-gonic/gin"
)

func TestAccessHandlerReturnsClosedSnapshot(t *testing.T) {
	pagePath := "/system/users"
	componentPath := "system/users"
	service := &currentAccessService{snapshot: access.Snapshot{
		RoleCodes: []string{"ai_tester", "registered_user"},
		MenuTree: []access.MenuNode{{
			Code: "system", MenuType: access.MenuDirectory, I18nKey: "navigation.system", IsHidden: 0, Children: []access.MenuNode{{
				Code: "account:user:view", MenuType: access.MenuPage, Path: &pagePath, ComponentPath: &componentPath,
				I18nKey: "navigation.systemUsers", IsHidden: 1, Children: []access.MenuNode{},
			}},
		}},
		PermissionCodes: []string{"account:user:create", "account:user:view"},
	}}
	recorder := serveAccessRoute(t, service, "Bearer token")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope) != 3 || envelope["code"] == nil || envelope["data"] == nil || envelope["message"] == nil {
		t.Fatalf("envelope = %v", envelope)
	}
	var data struct {
		RoleCodes       []string                     `json:"roleCodes"`
		MenuTree        []map[string]json.RawMessage `json:"menuTree"`
		PermissionCodes []string                     `json:"permissionCodes"`
	}
	if err := json.Unmarshal(envelope["data"], &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.RoleCodes, service.snapshot.RoleCodes) || !reflect.DeepEqual(data.PermissionCodes, service.snapshot.PermissionCodes) {
		t.Fatalf("data arrays = roles %v permissions %v", data.RoleCodes, data.PermissionCodes)
	}
	if len(data.MenuTree) != 1 {
		t.Fatalf("menu tree = %v", data.MenuTree)
	}
	assertClosedMenuNode(t, data.MenuTree[0])
	var children []map[string]json.RawMessage
	if err := json.Unmarshal(data.MenuTree[0]["children"], &children); err != nil || len(children) != 1 {
		t.Fatalf("children = %v error=%v", children, err)
	}
	assertClosedMenuNode(t, children[0])
	if service.identity.UserID != 41 || service.identity.PlatformID != 17 || service.identity.Platform != "admin" || service.ctx == nil {
		t.Fatalf("service identity=%+v context=%v", service.identity, service.ctx)
	}
}

func TestAccessHandlerRejectsMissingIdentity(t *testing.T) {
	service := &currentAccessService{}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/access", access.NewHandler(service).Current)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/access", nil))

	assertHTTPCode(t, recorder, http.StatusUnauthorized, apperror.CodeUnauthorized)
	if service.calls != 0 {
		t.Fatal("service was called without an authentication identity")
	}
}

func TestAccessHandlerPreservesServiceFailure(t *testing.T) {
	service := &currentAccessService{err: apperror.DependencyUnavailable(errors.New("postgres down"))}
	recorder := serveAccessRoute(t, service, "Bearer token")
	assertHTTPCode(t, recorder, http.StatusServiceUnavailable, apperror.CodeDependencyUnavailable)
}

func TestAccessRouteRequiresAuthentication(t *testing.T) {
	service := &currentAccessService{}
	recorder := serveAccessRoute(t, service, "")
	assertHTTPCode(t, recorder, http.StatusUnauthorized, apperror.CodeUnauthorized)
	if service.calls != 0 {
		t.Fatal("service was called for an unauthenticated request")
	}
}

type currentAccessService struct {
	snapshot access.Snapshot
	err      error
	ctx      context.Context
	identity auth.Identity
	calls    int
}

func (s *currentAccessService) Current(ctx context.Context, identity auth.Identity) (access.Snapshot, error) {
	s.calls++
	s.ctx = ctx
	s.identity = identity
	return s.snapshot, s.err
}

type accessAuthService struct{}

func (accessAuthService) Register(context.Context, auth.RegisterInput) (auth.Registered, error) {
	return auth.Registered{}, nil
}

func (accessAuthService) Login(context.Context, auth.LoginInput) (auth.Credential, error) {
	return auth.Credential{}, nil
}

func (accessAuthService) Refresh(context.Context, auth.RefreshInput) (auth.Credential, error) {
	return auth.Credential{}, nil
}

func (accessAuthService) Authenticate(context.Context, string, authclient.Client) (auth.Identity, error) {
	return auth.Identity{UserID: 41, SessionID: 42, PlatformID: 17, Platform: "admin", Version: 1, PolicyVersion: 4, AccessCacheTTL: time.Hour}, nil
}

func (accessAuthService) Logout(context.Context, auth.Identity, authclient.Client) error {
	return nil
}

func (accessAuthService) CurrentUser(context.Context, auth.Identity) (account.Current, error) {
	return account.Current{}, nil
}

func serveAccessRoute(t *testing.T, service *currentAccessService, authorization string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	access.RegisterRoutes(router.Group("/api/v1", authclient.Require()), access.NewHandler(service), auth.Authenticate(accessAuthService{}))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/access", nil)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	request.Header[authclient.PlatformHeader] = []string{"admin"}
	request.Header[authclient.DeviceIDHeader] = []string{"550e8400-e29b-41d4-a716-446655440000"}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func assertClosedMenuNode(t *testing.T, node map[string]json.RawMessage) {
	t.Helper()
	want := []string{"code", "menuType", "path", "componentPath", "i18nKey", "icon", "isHidden", "children"}
	if len(node) != len(want) {
		t.Fatalf("menu node keys = %v", node)
	}
	for _, key := range want {
		if node[key] == nil {
			t.Fatalf("menu node missing %q: %v", key, node)
		}
	}
}

func assertHTTPCode(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus, wantCode int) {
	t.Helper()
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body)
	}
	var envelope struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil || envelope.Code != wantCode {
		t.Fatalf("code = %d decodeErr=%v body=%s", envelope.Code, err, recorder.Body)
	}
}
