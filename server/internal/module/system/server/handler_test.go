package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type snapshotStub struct {
	calls int
	ctx   context.Context
	err   error
}

func (s *snapshotStub) Snapshot(ctx context.Context) (Snapshot, error) {
	s.calls++
	s.ctx = ctx
	return Snapshot{Hostname: "node-a"}, s.err
}

func TestServerSnapshotRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, scenario := range []struct {
		name          string
		authenticated bool
		want          int
	}{
		{"anonymous", false, 401}, {"signed-in user", true, 200},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			service := &snapshotStub{}
			router := gin.New()
			authenticate := func(c *gin.Context) {
				if !scenario.authenticated {
					response.Fail(c, apperror.Unauthorized(errors.New("missing session")))
					c.Abort()
					return
				}
				c.Next()
			}
			RegisterAdminRoutes(router.Group("/api/admin/v1"), NewHandler(service), authenticate)
			r := httptest.NewRequest(http.MethodGet, "/api/admin/v1/system/server", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if w.Code != scenario.want {
				t.Fatalf("status=%d body=%s", w.Code, w.Body)
			}
			if scenario.want != 200 {
				if service.calls != 0 {
					t.Fatal("unauthorized collection")
				}
				return
			}
			if service.ctx != r.Context() || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("missing context or private cache policy")
			}
			var body struct {
				Code    int
				Data    Snapshot
				Message string
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Code != 0 || body.Data.Hostname != "node-a" {
				t.Fatalf("body=%s", w.Body)
			}
		})
	}
}

func TestServerSnapshotReturnsUnifiedFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	s := &snapshotStub{err: apperror.DependencyUnavailable(errors.New("private OS error"))}
	router.GET("/snapshot", NewHandler(s).Detail)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/snapshot", nil))
	if w.Code != 503 || !json.Valid(w.Body.Bytes()) || strings.Contains(w.Body.String(), "private OS error") {
		t.Fatalf("response=%d %s", w.Code, w.Body)
	}
}
