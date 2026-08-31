package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"admin/server/internal/module/auth/client"
	"admin/server/internal/module/auth/login"
	"admin/server/internal/module/auth/platform"
	"admin/server/internal/module/health"
	"admin/server/internal/module/permission/access"
	"admin/server/internal/module/permission/menu"
	"admin/server/internal/module/permission/role"
	"admin/server/internal/module/storage/cosconfig"
	"admin/server/internal/module/storage/uploadrule"
	"admin/server/internal/module/system/operationlog"
	"admin/server/internal/module/user/account"
	usersession "admin/server/internal/module/user/session"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"

	"github.com/gin-gonic/gin"
)

type readyService struct{}

func (readyService) Readiness(context.Context) (health.Readiness, error) {
	return health.Readiness{PostgreSQL: "up", Redis: "up"}, nil
}

type recordingOperationEnqueuer struct {
	payloads []operationlog.TaskPayload
}

func (e *recordingOperationEnqueuer) Enqueue(_ context.Context, payload operationlog.TaskPayload) error {
	e.payloads = append(e.payloads, payload)
	return nil
}

type apiAccessService struct{}

func (apiAccessService) Current(context.Context, auth.Identity) (permission.Snapshot, error) {
	return permission.Snapshot{RoleCodes: []string{}, MenuTree: []permission.MenuNode{}, PermissionCodes: []string{}}, nil
}

type apiMenuService struct{}

func (apiMenuService) RebuildAccessCache(context.Context) (int, error) { return 0, nil }

type apiRoleService struct{}
type apiUserService struct{}
type apiAuthPlatformService struct{}
type apiOperationLogService struct{}
type apiSessionAdminService struct{}

func apiSessionActor(*gin.Context) (usersession.Actor, bool) {
	return usersession.Actor{UserID: 1, SessionID: 2}, true
}

func (apiOperationLogService) List(context.Context, operationlog.ListQuery) (operationlog.ListResult, error) {
	return operationlog.ListResult{List: []operationlog.Item{}}, nil
}
func (apiSessionAdminService) ListSessions(context.Context, usersession.AdminSessionQuery) ([]usersession.AdminSession, int64, error) {
	return []usersession.AdminSession{}, 0, nil
}
func (apiSessionAdminService) SessionStats(context.Context) (usersession.AdminSessionStats, error) {
	return usersession.AdminSessionStats{Platforms: map[string]int64{}}, nil
}
func (apiSessionAdminService) RevokeSession(context.Context, usersession.Actor, int64) (usersession.AdminRevokeResult, error) {
	return usersession.AdminRevokeResult{}, nil
}
func (apiSessionAdminService) RevokeSessions(context.Context, usersession.Actor, []int64) (usersession.AdminRevokeResult, error) {
	return usersession.AdminRevokeResult{}, nil
}

func TestBuildRouterDoesNotRegisterExampleTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	enqueuer := &recordingOperationEnqueuer{}
	router := buildRouter(routerDependencies{
		CORSOrigin:        "http://localhost:16300",
		Logger:            logger,
		Health:            health.NewHandler(readyService{}),
		Auth:              auth.NewHandler(apiAuthService{}, false),
		AuthPlatform:      authplatform.NewHandler(apiAuthPlatformService{}),
		Permission:        permission.NewHandler(apiAccessService{}),
		Menu:              menu.NewHandler(apiMenuService{}),
		Role:              role.NewHandler(apiRoleService{}),
		User:              account.NewHandler(apiUserService{}, func(*gin.Context) (int64, bool) { return 1, true }),
		COSConfig:         cosconfig.NewHandler(cosconfig.NewService(nil, nil, nil)),
		UploadRule:        uploadrule.NewHandler(uploadrule.NewService(nil, nil, nil)),
		OperationLog:      operationlog.NewHandler(apiOperationLogService{}),
		OperationEnqueuer: enqueuer,
		SessionAdmin:      usersession.NewSessionAdminHandler(apiSessionAdminService{}, apiSessionActor),
		AuthOrigin:        func(context *gin.Context) { context.Next() },
		Authenticate:      func(context *gin.Context) { context.Next() },
		RequirePermission: func(string) gin.HandlerFunc { return func(context *gin.Context) { context.Next() } },
	})

	request := httptest.NewRequest(http.MethodPost, "/api/admin/v1/example-tasks", strings.NewReader(`{"message":"removed"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(authclient.PlatformHeader, "admin")
	request.Header.Set(authclient.DeviceIDHeader, "550e8400-e29b-41d4-a716-446655440000")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("removed route status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(enqueuer.payloads) != 0 {
		t.Fatalf("removed route operation log payload count = %d", len(enqueuer.payloads))
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

func (apiUserService) List(context.Context, account.ListQuery) (pagination.Result[account.ListItem], error) {
	return pagination.Result[account.ListItem]{List: []account.ListItem{}}, nil
}
func (apiUserService) RoleOptions(context.Context) ([]account.RoleSummary, error) {
	return []account.RoleSummary{}, nil
}
func (apiUserService) Update(context.Context, int64, int64, account.UpdateInput) (account.UpdatedProfile, error) {
	return account.UpdatedProfile{}, nil
}
func (apiUserService) UpdateStatus(context.Context, int64, int64, yesno.Value) error { return nil }
func (apiUserService) Delete(context.Context, int64, int64) error                    { return nil }
func (apiUserService) Roles(context.Context, int64) (account.Roles, error) {
	return account.Roles{Roles: []account.RoleSummary{}, RoleIDs: []int64{}}, nil
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
	return role.Permissions{Platforms: []role.PermissionPlatform{}, MenuIDs: []int64{}}, nil
}
func (apiRoleService) UpdatePermissions(context.Context, int64, []int64) (int64, error) {
	return 0, nil
}

func (apiMenuService) List(context.Context, menu.ListQuery) (menu.Catalog, error) {
	return menu.Catalog{Platforms: []menu.PlatformOption{}, MenuTree: []menu.ManagedMenu{}}, nil
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

func (apiAuthService) CurrentUser(context.Context, auth.Identity) (account.Current, error) {
	return account.Current{}, nil
}

func routeSetDiff(routes []gin.RouteInfo, want map[string]int) (map[string]int, []string) {
	remaining := make(map[string]int, len(want))
	for route, count := range want {
		remaining[route] = count
	}
	var unexpected []string
	for _, route := range routes {
		key := route.Method + " " + route.Path
		if count, ok := remaining[key]; ok && count > 0 {
			remaining[key]--
			continue
		}
		unexpected = append(unexpected, key)
	}
	return remaining, unexpected
}

func TestRouteSetDiffRejectsUnexpectedRoutes(t *testing.T) {
	remaining, unexpected := routeSetDiff([]gin.RouteInfo{
		{Method: http.MethodGet, Path: "/health"},
		{Method: http.MethodGet, Path: "/health"},
		{Method: http.MethodGet, Path: "/api/v1/unexpected"},
	}, map[string]int{
		"GET /health": 1,
		"GET /ready":  1,
	})
	if remaining["GET /ready"] != 1 {
		t.Fatalf("missing route count = %d", remaining["GET /ready"])
	}
	if !reflect.DeepEqual(unexpected, []string{"GET /health", "GET /api/v1/unexpected"}) {
		t.Fatalf("unexpected routes = %#v", unexpected)
	}
}

func TestBuildRouterRegistersFoundationRoutesOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := buildRouter(routerDependencies{
		CORSOrigin:   "http://localhost:16300",
		Logger:       logger,
		Health:       health.NewHandler(readyService{}),
		Auth:         auth.NewHandler(apiAuthService{}, false),
		AuthPlatform: authplatform.NewHandler(apiAuthPlatformService{}),
		Permission:   permission.NewHandler(apiAccessService{}),
		Menu:         menu.NewHandler(apiMenuService{}),
		Role:         role.NewHandler(apiRoleService{}),
		User:         account.NewHandler(apiUserService{}, func(*gin.Context) (int64, bool) { return 1, true }),
		COSConfig:    cosconfig.NewHandler(cosconfig.NewService(nil, nil, nil)),
		UploadRule:   uploadrule.NewHandler(uploadrule.NewService(nil, nil, nil)),
		OperationLog: operationlog.NewHandler(apiOperationLogService{}),
		SessionAdmin: usersession.NewSessionAdminHandler(apiSessionAdminService{}, apiSessionActor),
		AuthOrigin:   auth.RequireOrigin("http://localhost:16300"),
		Authenticate: auth.Authenticate(apiAuthService{}),
		RequirePermission: func(string) gin.HandlerFunc {
			return func(context *gin.Context) { context.Next() }
		},
	})

	want := map[string]int{
		"GET /health":                                         1,
		"GET /ready":                                          1,
		"GET /api/v1/auth/policy":                             1,
		"POST /api/v1/auth/register":                          1,
		"POST /api/v1/auth/login":                             1,
		"POST /api/v1/auth/refresh":                           1,
		"POST /api/v1/auth/logout":                            1,
		"GET /api/v1/auth/me":                                 1,
		"GET /api/v1/access":                                  1,
		"POST /api/v1/storage/upload-credentials":             1,
		"GET /api/admin/v1/auth-platforms":                    1,
		"GET /api/admin/v1/auth-platforms/deployment":         1,
		"GET /api/admin/v1/account/profile":                   1,
		"POST /api/admin/v1/auth-platforms":                   1,
		"POST /api/admin/v1/account/password":                 1,
		"PUT /api/admin/v1/auth-platforms/:id":                1,
		"PUT /api/admin/v1/account/profile":                   1,
		"PATCH /api/admin/v1/auth-platforms/:id/status":       1,
		"DELETE /api/admin/v1/auth-platforms/:id":             1,
		"GET /api/admin/v1/menus":                             1,
		"POST /api/admin/v1/menus":                            1,
		"PUT /api/admin/v1/menus/:id":                         1,
		"PATCH /api/admin/v1/menus/:id/status":                1,
		"DELETE /api/admin/v1/menus/:id":                      1,
		"POST /api/admin/v1/menus/access-cache/rebuild":       1,
		"GET /api/admin/v1/roles":                             1,
		"POST /api/admin/v1/roles":                            1,
		"PUT /api/admin/v1/roles/:id":                         1,
		"PATCH /api/admin/v1/roles/:id/status":                1,
		"PATCH /api/admin/v1/roles/:id/default":               1,
		"DELETE /api/admin/v1/roles/:id":                      1,
		"GET /api/admin/v1/roles/:id/permissions":             1,
		"PUT /api/admin/v1/roles/:id/permissions":             1,
		"GET /api/admin/v1/users":                             1,
		"GET /api/admin/v1/users/role-options":                1,
		"PUT /api/admin/v1/users/:id":                         1,
		"PATCH /api/admin/v1/users/:id/status":                1,
		"DELETE /api/admin/v1/users/:id":                      1,
		"GET /api/admin/v1/users/:id/roles":                   1,
		"PUT /api/admin/v1/users/:id/roles":                   1,
		"GET /api/admin/v1/sessions":                          1,
		"GET /api/admin/v1/sessions/stats":                    1,
		"DELETE /api/admin/v1/sessions/:id":                   1,
		"DELETE /api/admin/v1/sessions":                       1,
		"GET /api/admin/v1/operation-logs":                    1,
		"GET /api/admin/v1/users/login-logs":                  1,
		"GET /api/admin/v1/users/login-logs/page-init":        1,
		"GET /api/admin/v1/storage/cos-configs":               1,
		"POST /api/admin/v1/storage/cos-configs":              1,
		"GET /api/admin/v1/storage/cos-configs/:id":           1,
		"PUT /api/admin/v1/storage/cos-configs/:id":           1,
		"PATCH /api/admin/v1/storage/cos-configs/:id/status":  1,
		"POST /api/admin/v1/storage/cos-configs/:id/test":     1,
		"DELETE /api/admin/v1/storage/cos-configs/:id":        1,
		"GET /api/admin/v1/storage/upload-rules":              1,
		"GET /api/admin/v1/storage/upload-rules/page-init":    1,
		"POST /api/admin/v1/storage/upload-rules":             1,
		"GET /api/admin/v1/storage/upload-rules/:id":          1,
		"PUT /api/admin/v1/storage/upload-rules/:id":          1,
		"PATCH /api/admin/v1/storage/upload-rules/:id/status": 1,
		"DELETE /api/admin/v1/storage/upload-rules/:id":       1,
		"POST /api/v1/storage/object-url":                     1,
	}
	remaining, unexpected := routeSetDiff(router.Routes(), want)
	if len(unexpected) != 0 {
		t.Fatalf("unexpected routes = %#v; routes=%+v", unexpected, router.Routes())
	}
	for route, remaining := range remaining {
		if remaining != 0 {
			t.Fatalf("route %s remaining count = %d; routes=%+v", route, remaining, router.Routes())
		}
	}

	for _, oldRoute := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/auth-platforms"},
		{method: http.MethodGet, path: "/api/v1/menus"},
		{method: http.MethodGet, path: "/api/v1/roles"},
		{method: http.MethodGet, path: "/api/v1/users"},
		{method: http.MethodGet, path: "/api/v1/sessions"},
		{method: http.MethodGet, path: "/api/v1/operation-logs"},
		{method: http.MethodPost, path: "/api/v1/example-tasks"},
		{method: http.MethodPost, path: "/api/admin/v1/example-tasks"},
	} {
		request := httptest.NewRequest(oldRoute.method, oldRoute.path, nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("old route %s %s status=%d", oldRoute.method, oldRoute.path, recorder.Code)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/api/admin/v1/menus", nil)
	request.Header.Set(authclient.PlatformHeader, "portal")
	request.Header.Set(authclient.DeviceIDHeader, "550e8400-e29b-41d4-a716-446655440000")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), `"code":10003`) {
		t.Fatalf("non-admin response status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/health", nil)
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
		{method: http.MethodGet, path: "/api/admin/v1/menus"},
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

func TestRunDoesNotMutatePersistentStateDuringStartup(t *testing.T) {
	content, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(content)
	forbidden := []string{
		"database.PrepareDomainNames(",
		"auth.PrepareSessionSchema(",
		"operationlog.PrepareSchema(",
		"menu.PrepareSchema(",
		"database.AutoMigrate(",
		"authplatform.EnsureSchema(",
		"authplatform.EnsureCanvasPreset(",
		"menu.PreparePlatformSchema(",
		"role.EnsureSchema(",
		"auth.EnsureSchema(",
		"menu.EnsureSchema(",
		"permission.EnsureSchema(",
		"operationlog.EnsureSchema(",
		"authplatform.ClearBuiltinPolicies(",
		"auth.CleanupLegacySessionPointers(",
		"menuService.EnsureFoundation(",
		"menuService.EnsurePlatformFoundation(",
		"roleService.EnsureSystemRoles(",
	}
	for _, fragment := range forbidden {
		if strings.Contains(source, fragment) {
			t.Errorf("runtime startup contains state mutation %s", fragment)
		}
	}
	wantOrder := []string{
		"database.Open(",
		"projectredis.Open(",
		"queue.NewClient(",
		"buildRouter(",
	}
	previous := -1
	for _, fragment := range wantOrder {
		position := strings.Index(source, fragment)
		if position < 0 {
			t.Fatalf("run source lacks %s", fragment)
		}
		if position <= previous {
			t.Fatalf("run source order is invalid at %s", fragment)
		}
		previous = position
	}
}
