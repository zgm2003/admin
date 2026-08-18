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
	"admin/server/internal/module/taskdemo"
	"admin/server/internal/module/user"
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
		AuthOrigin:   auth.RequireOrigin("http://localhost:16300"),
		Authenticate: auth.Authenticate(apiAuthService{}),
	})

	want := map[string]int{
		"GET /health":                1,
		"GET /ready":                 1,
		"POST /api/v1/example-tasks": 1,
		"POST /api/v1/auth/register": 1,
		"POST /api/v1/auth/login":    1,
		"POST /api/v1/auth/refresh":  1,
		"POST /api/v1/auth/logout":   1,
		"GET /api/v1/auth/me":        1,
		"GET /api/v1/access":         1,
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
	} {
		recorder = httptest.NewRecorder()
		request = httptest.NewRequest(protectedPath.method, protectedPath.path, nil)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d body=%s", protectedPath.method, protectedPath.path, recorder.Code, recorder.Body)
		}
	}
}
