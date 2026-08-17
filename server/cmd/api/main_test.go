package main

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"admin/server/internal/module/health"
	"admin/server/internal/module/taskdemo"
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

func TestBuildRouterRegistersFoundationRoutesOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := buildRouter(routerDependencies{
		CORSOrigin: "http://localhost:16300",
		Logger:     logger,
		Health:     health.NewHandler(readyService{}),
		Task:       taskdemo.NewHandler(submitService{}),
	})

	want := map[string]int{
		"GET /health":                1,
		"GET /ready":                 1,
		"POST /api/v1/example-tasks": 1,
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
}
