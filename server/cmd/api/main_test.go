package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"admin/server/internal/module/access"
	"admin/server/internal/module/auth"
	"admin/server/internal/module/health"
	"admin/server/internal/module/menu"
	"admin/server/internal/module/role"
	"admin/server/internal/module/taskdemo"
	"admin/server/internal/module/user"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

type readyService struct{}

func (readyService) Readiness(context.Context) (health.Readiness, error) {
	return health.Readiness{PostgreSQL: "up", Redis: "up"}, nil
}

type submitService struct{}

func (submitService) Create(context.Context, string) (taskdemo.Created, error) {
	return taskdemo.Created{TaskID: "task-1"}, nil
}

type apiAccessService struct{}

func (apiAccessService) Current(context.Context, int64) (access.Snapshot, error) {
	return access.Snapshot{RoleCodes: []string{}, MenuTree: []access.MenuNode{}, PermissionCodes: []string{}}, nil
}

type apiMenuService struct{}

type apiRoleService struct{}

func (apiRoleService) List(context.Context, role.ListQuery) (pagination.Result[role.ListItem], error) {
	return pagination.Result[role.ListItem]{List: []role.ListItem{}}, nil
}
func (apiRoleService) Create(context.Context, role.CreateInput) (int64, error) { return 1, nil }
func (apiRoleService) Update(context.Context, int64, role.UpdateInput) error   { return nil }
func (apiRoleService) UpdateStatus(context.Context, int64, yesno.Value) error  { return nil }
func (apiRoleService) SetDefault(context.Context, int64) error                 { return nil }
func (apiRoleService) Delete(context.Context, int64) error                     { return nil }
func (apiRoleService) Permissions(context.Context, int64) (role.Permissions, error) {
	return role.Permissions{MenuTree: []role.PermissionTreeNode{}, MenuIDs: []int64{}}, nil
}
func (apiRoleService) UpdatePermissions(context.Context, int64, []int64) (int64, error) {
	return 0, nil
}

func (apiMenuService) List(context.Context) ([]menu.ManagedMenu, error) {
	return []menu.ManagedMenu{}, nil
}

func (apiMenuService) Create(context.Context, menu.CreateInput) (int64, error) {
	return 1, nil
}

func (apiMenuService) Update(context.Context, int64, menu.UpdateInput) error {
	return nil
}

func (apiMenuService) UpdateStatus(context.Context, int64, yesno.Value) error {
	return nil
}

func (apiMenuService) Delete(context.Context, int64) error {
	return nil
}

type apiAuthService struct{}

func (apiAuthService) Register(context.Context, auth.RegisterInput) (auth.Registered, error) {
	return auth.Registered{}, nil
}

func (apiAuthService) Login(context.Context, auth.LoginInput) (auth.Credential, error) {
	return auth.Credential{}, nil
}

func (apiAuthService) Refresh(context.Context, auth.RefreshInput) (auth.Credential, error) {
	return auth.Credential{}, nil
}

func (apiAuthService) Authenticate(context.Context, string) (auth.Identity, error) {
	return auth.Identity{}, nil
}

func (apiAuthService) Logout(context.Context, auth.Identity) error {
	return nil
}

func (apiAuthService) CurrentUser(context.Context, auth.Identity) (user.Current, error) {
	return user.Current{}, nil
}

func TestBuildRouterRegistersFoundationRoutesOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := buildRouter(routerDependencies{
		CORSOrigin:   "http://localhost:16300",
		Logger:       logger,
		Health:       health.NewHandler(readyService{}),
		Task:         taskdemo.NewHandler(submitService{}),
		Auth:         auth.NewHandler(apiAuthService{}, false),
		Access:       access.NewHandler(apiAccessService{}),
		Menu:         menu.NewHandler(apiMenuService{}),
		Role:         role.NewHandler(apiRoleService{}),
		AuthOrigin:   auth.RequireOrigin("http://localhost:16300"),
		Authenticate: auth.Authenticate(apiAuthService{}),
		RequirePermission: func(string) gin.HandlerFunc {
			return func(context *gin.Context) { context.Next() }
		},
	})

	want := map[string]int{
		"GET /health":                       1,
		"GET /ready":                        1,
		"POST /api/v1/example-tasks":        1,
		"POST /api/v1/auth/register":        1,
		"POST /api/v1/auth/login":           1,
		"POST /api/v1/auth/refresh":         1,
		"POST /api/v1/auth/logout":          1,
		"GET /api/v1/auth/me":               1,
		"GET /api/v1/access":                1,
		"GET /api/v1/menus":                 1,
		"POST /api/v1/menus":                1,
		"PUT /api/v1/menus/:id":             1,
		"PATCH /api/v1/menus/:id/status":    1,
		"DELETE /api/v1/menus/:id":          1,
		"GET /api/v1/roles":                 1,
		"POST /api/v1/roles":                1,
		"PUT /api/v1/roles/:id":             1,
		"PATCH /api/v1/roles/:id/status":    1,
		"PATCH /api/v1/roles/:id/default":   1,
		"DELETE /api/v1/roles/:id":          1,
		"GET /api/v1/roles/:id/permissions": 1,
		"PUT /api/v1/roles/:id/permissions": 1,
	}
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key]--
		}
	}
	for route, remaining := range want {
		if remaining != 0 {
			t.Fatalf("route %s remaining count = %d; routes=%+v", route, remaining, router.Routes())
		}
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("Accept-Language", "en-US")
	router.ServeHTTP(recorder, request)
	if got := recorder.Header().Get("Content-Language"); got != "en-US" {
		t.Fatalf("Content-Language = %q, want en-US", got)
	}

	for _, protectedPath := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/access"},
		{method: http.MethodPost, path: "/api/v1/example-tasks"},
		{method: http.MethodGet, path: "/api/v1/menus"},
	} {
		recorder = httptest.NewRecorder()
		request = httptest.NewRequest(protectedPath.method, protectedPath.path, nil)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d body=%s", protectedPath.method, protectedPath.path, recorder.Code, recorder.Body)
		}
	}
}
