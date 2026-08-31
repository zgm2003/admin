package permission_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"admin/server/internal/module/auth/client"
	"admin/server/internal/module/auth/login"
	"admin/server/internal/module/permission/access"
	"admin/server/internal/shared/apperror"
	"github.com/gin-gonic/gin"
)

func TestRequirePermissionRejectsMissingIdentityWithoutCallingService(t *testing.T) {
	service := &permissionAccessService{}
	recorder, called := servePermissionRequest(t, service, false, "account:user:create")
	assertHTTPCode(t, recorder, http.StatusUnauthorized, apperror.CodeUnauthorized)
	if called || service.calls != 0 {
		t.Fatalf("next=%v service calls=%d", called, service.calls)
	}
}

func TestRequirePermissionAllowsTheNextHandler(t *testing.T) {
	service := &permissionAccessService{allowed: true}
	recorder, called := servePermissionRequest(t, service, true, "account:user:create")
	if recorder.Code != http.StatusNoContent || !called {
		t.Fatalf("status=%d next=%v body=%s", recorder.Code, called, recorder.Body)
	}
	if service.identity.UserID != 41 || service.identity.PlatformID != 17 || service.identity.Platform != "admin" || service.code != "account:user:create" || service.ctx == nil {
		t.Fatalf("service identity=%+v code=%q context=%v", service.identity, service.code, service.ctx)
	}
}

func TestRequirePermissionReturnsTranslatedForbidden(t *testing.T) {
	service := &permissionAccessService{allowed: false}
	recorder, called := servePermissionRequest(t, service, true, "account:user:delete")
	assertHTTPCode(t, recorder, http.StatusForbidden, apperror.CodeForbidden)
	if called {
		t.Fatal("next handler ran for a denied permission")
	}
}

func TestRequirePermissionPreservesServiceFailure(t *testing.T) {
	service := &permissionAccessService{err: apperror.DependencyUnavailable(errors.New("postgres down"))}
	recorder, called := servePermissionRequest(t, service, true, "account:user:create")
	assertHTTPCode(t, recorder, http.StatusServiceUnavailable, apperror.CodeDependencyUnavailable)
	if called {
		t.Fatal("next handler ran after a permission service failure")
	}
}

func TestRequirePermissionPanicsForEmptyPermissionCode(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("RequirePermission accepted an empty permission code")
		}
	}()
	_ = permission.RequirePermission(&permissionAccessService{}, " ")
}

type permissionAccessService struct {
	allowed  bool
	err      error
	calls    int
	ctx      context.Context
	identity auth.Identity
	code     string
}

func (s *permissionAccessService) Allowed(ctx context.Context, identity auth.Identity, code string) (bool, error) {
	s.calls++
	s.ctx = ctx
	s.identity = identity
	s.code = code
	return s.allowed, s.err
}

func servePermissionRequest(t *testing.T, service *permissionAccessService, authenticated bool, permissionCode string) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	called := false
	router := gin.New()
	if authenticated {
		router.Use(authclient.Require(), auth.Authenticate(accessAuthService{}))
	}
	router.Use(permission.RequirePermission(service, permissionCode))
	router.GET("/", func(context *gin.Context) {
		called = true
		context.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	if authenticated {
		request.Header.Set("Authorization", "Bearer token")
		request.Header[authclient.PlatformHeader] = []string{"admin"}
		request.Header[authclient.DeviceIDHeader] = []string{"550e8400-e29b-41d4-a716-446655440000"}
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder, called
}
