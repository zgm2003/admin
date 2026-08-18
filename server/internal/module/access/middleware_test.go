package access_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"admin/server/internal/module/access"
	"admin/server/internal/module/auth"
	"admin/server/internal/shared/apperror"
	"github.com/gin-gonic/gin"
)

func TestRequirePermissionRejectsMissingIdentityWithoutCallingService(t *testing.T) {
	service := &permissionAccessService{}
	recorder, called := servePermissionRequest(t, service, false, "system:user:create")
	assertHTTPCode(t, recorder, http.StatusUnauthorized, apperror.CodeUnauthorized)
	if called || service.calls != 0 {
		t.Fatalf("next=%v service calls=%d", called, service.calls)
	}
}

func TestRequirePermissionAllowsTheNextHandler(t *testing.T) {
	service := &permissionAccessService{allowed: true}
	recorder, called := servePermissionRequest(t, service, true, "system:user:create")
	if recorder.Code != http.StatusNoContent || !called {
		t.Fatalf("status=%d next=%v body=%s", recorder.Code, called, recorder.Body)
	}
	if service.userID != 41 || service.code != "system:user:create" || service.ctx == nil {
		t.Fatalf("service userID=%d code=%q context=%v", service.userID, service.code, service.ctx)
	}
}

func TestRequirePermissionReturnsTranslatedForbidden(t *testing.T) {
	service := &permissionAccessService{allowed: false}
	recorder, called := servePermissionRequest(t, service, true, "system:user:delete")
	assertHTTPCode(t, recorder, http.StatusForbidden, apperror.CodeForbidden)
	if called {
		t.Fatal("next handler ran for a denied permission")
	}
}

func TestRequirePermissionPreservesServiceFailure(t *testing.T) {
	service := &permissionAccessService{err: apperror.DependencyUnavailable(errors.New("postgres down"))}
	recorder, called := servePermissionRequest(t, service, true, "system:user:create")
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
	_ = access.RequirePermission(&permissionAccessService{}, " ")
}

type permissionAccessService struct {
	allowed bool
	err     error
	calls   int
	ctx     context.Context
	userID  int64
	code    string
}

func (s *permissionAccessService) Allowed(ctx context.Context, userID int64, code string) (bool, error) {
	s.calls++
	s.ctx = ctx
	s.userID = userID
	s.code = code
	return s.allowed, s.err
}

func servePermissionRequest(t *testing.T, service *permissionAccessService, authenticated bool, permissionCode string) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	called := false
	router := gin.New()
	if authenticated {
		router.Use(auth.Authenticate(accessAuthService{}))
	}
	router.Use(access.RequirePermission(service, permissionCode))
	router.GET("/", func(context *gin.Context) {
		called = true
		context.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	if authenticated {
		request.Header.Set("Authorization", "Bearer token")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder, called
}
