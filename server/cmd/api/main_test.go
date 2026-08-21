package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"admin/server/internal/module/access"
	"admin/server/internal/module/auth"
	"admin/server/internal/module/authclient"
	"admin/server/internal/module/authplatform"
	"admin/server/internal/module/health"
	"admin/server/internal/module/menu"
	"admin/server/internal/module/operationlog"
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

type panicSubmitService struct{}

func (panicSubmitService) Create(context.Context, string) (taskdemo.Created, error) {
	panic("operation failed unexpectedly")
}

type recordingOperationEnqueuer struct {
	payloads []operationlog.TaskPayload
}

func (e *recordingOperationEnqueuer) Enqueue(_ context.Context, payload operationlog.TaskPayload) error {
	e.payloads = append(e.payloads, payload)
	return nil
}

type apiAccessService struct{}

func (apiAccessService) Current(context.Context, auth.Identity) (access.Snapshot, error) {
	return access.Snapshot{RoleCodes: []string{}, MenuTree: []access.MenuNode{}, PermissionCodes: []string{}}, nil
}

type apiMenuService struct{}

type apiRoleService struct{}
type apiUserService struct{}
type apiAuthPlatformService struct{}
type apiOperationLogService struct{}
type apiSessionAdminService struct{}

func (apiOperationLogService) List(context.Context, operationlog.ListQuery) (operationlog.ListResult, error) {
	return operationlog.ListResult{List: []operationlog.Item{}}, nil
}
func (apiSessionAdminService) ListSessions(context.Context, auth.AdminSessionQuery) ([]auth.AdminSession, int64, error) {
	return []auth.AdminSession{}, 0, nil
}
func (apiSessionAdminService) SessionStats(context.Context) (auth.AdminSessionStats, error) {
	return auth.AdminSessionStats{Platforms: map[string]int64{}}, nil
}
func (apiSessionAdminService) RevokeSession(context.Context, auth.Identity, int64) (auth.AdminRevokeResult, error) {
	return auth.AdminRevokeResult{}, nil
}
func (apiSessionAdminService) RevokeSessions(context.Context, auth.Identity, []int64) (auth.AdminRevokeResult, error) {
	return auth.AdminRevokeResult{}, nil
}

func TestBuildRouterAuditsPanickingOperation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	enqueuer := &recordingOperationEnqueuer{}
	router := buildRouter(routerDependencies{
		CORSOrigin:        "http://localhost:16300",
		Logger:            logger,
		Health:            health.NewHandler(readyService{}),
		Task:              taskdemo.NewHandler(panicSubmitService{}),
		Auth:              auth.NewHandler(apiAuthService{}, false),
		AuthPlatform:      authplatform.NewHandler(apiAuthPlatformService{}),
		Access:            access.NewHandler(apiAccessService{}),
		Menu:              menu.NewHandler(apiMenuService{}),
		Role:              role.NewHandler(apiRoleService{}),
		User:              user.NewHandler(apiUserService{}, func(*gin.Context) (int64, bool) { return 1, true }),
		OperationLog:      operationlog.NewHandler(apiOperationLogService{}),
		OperationEnqueuer: enqueuer,
		SessionAdmin:      auth.NewSessionAdminHandler(apiSessionAdminService{}),
		AuthOrigin:        func(context *gin.Context) { context.Next() },
		Authenticate:      func(context *gin.Context) { context.Next() },
		RequirePermission: func(string) gin.HandlerFunc { return func(context *gin.Context) { context.Next() } },
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/example-tasks", strings.NewReader(`{"message":"panic"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(authclient.PlatformHeader, "admin")
	request.Header.Set(authclient.DeviceIDHeader, "550e8400-e29b-41d4-a716-446655440000")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), `"code":10000`) {
		t.Fatalf("panic response status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(enqueuer.payloads) != 1 {
		t.Fatalf("operation log payload count = %d, want 1", len(enqueuer.payloads))
	}
	payload := enqueuer.payloads[0]
	if payload.StatusCode != http.StatusInternalServerError || payload.IsSuccess != 0 || payload.Action != "task.create" {
		t.Fatalf("panic operation payload = %+v", payload)
	}
}

func (apiAuthPlatformService) CurrentPolicy(context.Context, string) (authplatform.Policy, error) {
	return authplatform.Policy{}, nil
}
func (apiAuthPlatformService) List(context.Context, authplatform.ListQuery) (pagination.Result[authplatform.ListItem], error) {
	return pagination.Result[authplatform.ListItem]{List: []authplatform.ListItem{}}, nil
}
func (apiAuthPlatformService) Deployment(context.Context) (authplatform.Deployment, error) {
	return authplatform.Deployment{}, nil
}
func (apiAuthPlatformService) Create(context.Context, authplatform.CreateInput) (int64, error) {
	return 1, nil
}
func (apiAuthPlatformService) Update(context.Context, int64, authplatform.UpdateInput) error {
	return nil
}
func (apiAuthPlatformService) UpdateStatus(context.Context, int64, yesno.Value) error { return nil }
func (apiAuthPlatformService) Delete(context.Context, int64) error                    { return nil }

func (apiUserService) List(context.Context, user.ListQuery) (pagination.Result[user.ListItem], error) {
	return pagination.Result[user.ListItem]{List: []user.ListItem{}}, nil
}
func (apiUserService) RoleOptions(context.Context) ([]user.RoleSummary, error) {
	return []user.RoleSummary{}, nil
}
func (apiUserService) Update(context.Context, int64, int64, user.UpdateInput) (user.UpdatedUsername, error) {
	return user.UpdatedUsername{}, nil
}
func (apiUserService) UpdateStatus(context.Context, int64, int64, yesno.Value) error { return nil }
func (apiUserService) Delete(context.Context, int64, int64) error                    { return nil }
func (apiUserService) Roles(context.Context, int64) (user.Roles, error) {
	return user.Roles{Roles: []user.RoleSummary{}, RoleIDs: []int64{}}, nil
}
func (apiUserService) UpdateRoles(context.Context, int64, int64, []int64) (int64, error) {
	return 0, nil
}

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

func (apiAuthService) Authenticate(context.Context, string, authclient.Client) (auth.Identity, error) {
	return auth.Identity{}, nil
}

func (apiAuthService) Logout(context.Context, auth.Identity, authclient.Client) error {
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
		AuthPlatform: authplatform.NewHandler(apiAuthPlatformService{}),
		Access:       access.NewHandler(apiAccessService{}),
		Menu:         menu.NewHandler(apiMenuService{}),
		Role:         role.NewHandler(apiRoleService{}),
		User:         user.NewHandler(apiUserService{}, func(*gin.Context) (int64, bool) { return 1, true }),
		OperationLog: operationlog.NewHandler(apiOperationLogService{}),
		SessionAdmin: auth.NewSessionAdminHandler(apiSessionAdminService{}),
		AuthOrigin:   auth.RequireOrigin("http://localhost:16300"),
		Authenticate: auth.Authenticate(apiAuthService{}),
		RequirePermission: func(string) gin.HandlerFunc {
			return func(context *gin.Context) { context.Next() }
		},
	})

	want := map[string]int{
		"GET /health":                             1,
		"GET /ready":                              1,
		"POST /api/v1/example-tasks":              1,
		"GET /api/v1/auth/policy":                 1,
		"POST /api/v1/auth/register":              1,
		"POST /api/v1/auth/login":                 1,
		"POST /api/v1/auth/refresh":               1,
		"POST /api/v1/auth/logout":                1,
		"GET /api/v1/auth/me":                     1,
		"GET /api/v1/auth-platforms":              1,
		"GET /api/v1/auth-platforms/deployment":   1,
		"POST /api/v1/auth-platforms":             1,
		"PUT /api/v1/auth-platforms/:id":          1,
		"PATCH /api/v1/auth-platforms/:id/status": 1,
		"DELETE /api/v1/auth-platforms/:id":       1,
		"GET /api/v1/access":                      1,
		"GET /api/v1/menus":                       1,
		"POST /api/v1/menus":                      1,
		"PUT /api/v1/menus/:id":                   1,
		"PATCH /api/v1/menus/:id/status":          1,
		"DELETE /api/v1/menus/:id":                1,
		"GET /api/v1/roles":                       1,
		"POST /api/v1/roles":                      1,
		"PUT /api/v1/roles/:id":                   1,
		"PATCH /api/v1/roles/:id/status":          1,
		"PATCH /api/v1/roles/:id/default":         1,
		"DELETE /api/v1/roles/:id":                1,
		"GET /api/v1/roles/:id/permissions":       1,
		"PUT /api/v1/roles/:id/permissions":       1,
		"GET /api/v1/users":                       1,
		"GET /api/v1/users/role-options":          1,
		"PUT /api/v1/users/:id":                   1,
		"PATCH /api/v1/users/:id/status":          1,
		"DELETE /api/v1/users/:id":                1,
		"GET /api/v1/users/:id/roles":             1,
		"PUT /api/v1/users/:id/roles":             1,
		"GET /api/v1/operation-logs":              1,
		"GET /api/v1/sessions":                    1,
		"GET /api/v1/sessions/stats":              1,
		"DELETE /api/v1/sessions/:id":             1,
		"DELETE /api/v1/sessions":                 1,
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
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s %s without client headers status = %d body=%s", protectedPath.method, protectedPath.path, recorder.Code, recorder.Body)
		}
		recorder = httptest.NewRecorder()
		request = httptest.NewRequest(protectedPath.method, protectedPath.path, nil)
		request.Header[authclient.PlatformHeader] = []string{"admin"}
		request.Header[authclient.DeviceIDHeader] = []string{"550e8400-e29b-41d4-a716-446655440000"}
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s without Bearer status = %d body=%s", protectedPath.method, protectedPath.path, recorder.Code, recorder.Body)
		}
	}
}
