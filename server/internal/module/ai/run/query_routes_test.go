package run

import (
	"admin/server/internal/authcontext"
	"admin/server/internal/shared/apperror"
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRunRoutesSeparateManagementAndOwnerActions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	action := ""
	auth := func(c *gin.Context) { authcontext.Set(c, authcontext.Identity{UserID: 3, PlatformID: 2}) }
	require := func(code string) gin.HandlerFunc {
		return func(c *gin.Context) { action = code; c.AbortWithStatus(204) }
	}
	RegisterAdminRoutes(r.Group(""), NewHandler(nil), auth, require)
	for _, row := range []struct{ method, path, code string }{{"GET", "/ai/run", "ai:run:list"}, {"GET", "/ai/run/summary", "ai:run:list"}, {"GET", "/ai/run/1", "ai:run:detail"}, {"GET", "/ai/run/1/request", "ai:run:detail"}, {"GET", "/ai/run/1/tool-call", "ai:run:detail"}, {"GET", "/ai/run/1/usage-charge", "ai:run:detail"}, {"GET", "/ai/run/1/media", "ai:run:detail"}, {"GET", "/ai/run/1/media/2/preview", "ai:run:detail"}, {"POST", "/ai/chat/run", "ai:chat:generate"}, {"GET", "/ai/chat/run", "ai:chat:list"}, {"GET", "/ai/chat/run/1", "ai:chat:detail"}, {"GET", "/ai/chat/run/1/event", "ai:chat:detail"}, {"POST", "/ai/chat/run/1/cancel", "ai:chat:cancel"}, {"POST", "/ai/chat/run/1/media/retry", "ai:chat:retryTransfer"}} {
		action = ""
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(row.method, row.path, nil))
		if w.Code != 204 || action != row.code {
			t.Errorf("%s %s status=%d action=%q", row.method, row.path, w.Code, action)
		}
	}
	for _, path := range []string{"/ai/run/options", "/ai/chat/options"} {
		action = ""
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != 200 || action != "" {
			t.Errorf("options %s %d %s", path, w.Code, action)
		}
	}
}
func TestQueryPagesPreserveCursorAndBoundRows(t *testing.T) {
	p := page([]int64{6, 5, 4}, 2, func(v int64) int64 { return v })
	if len(p.List) != 2 || !p.HasMore || p.NextBeforeID != "5" {
		t.Fatalf("%+v", p)
	}
	p = page([]int64{}, 20, func(v int64) int64 { return v })
	b, _ := json.Marshal(p)
	if string(b) == "null" || p.List == nil {
		t.Fatal(string(b))
	}
}

type cancelRepositoryProbe struct {
	user, platform int64
	called         bool
	err            error
}

func (p *cancelRepositoryProbe) FindByRequest(context.Context, int64, int64, string) (Model, bool, error) {
	return Model{}, false, nil
}
func (p *cancelRepositoryProbe) Create(context.Context, CreateInput, RuntimeSnapshot, json.RawMessage, time.Time) (Model, bool, error) {
	return Model{}, false, nil
}
func (p *cancelRepositoryProbe) Cancel(ctx context.Context, id, u, platform int64, at time.Time) error {
	p.called = true
	p.user = u
	p.platform = platform
	return p.err
}
func TestCancelServiceCarriesOwnerAndMapsDependencyError(t *testing.T) {
	p := &cancelRepositoryProbe{}
	s := NewService(p, nil)
	if e := s.Cancel(context.Background(), 1, 2, 3); e != nil {
		t.Fatal(e)
	}
	if !p.called || p.user != 2 || p.platform != 3 {
		t.Fatalf("%+v", p)
	}
	p.called = false
	if s.Cancel(context.Background(), 0, 2, 3) == nil || p.called {
		t.Fatal("invalid cancellation reached repository")
	}
	p.err = errors.New("database secret")
	if s.Cancel(context.Background(), 1, 2, 3) == nil {
		t.Fatal("dependency failure swallowed")
	}
}

func TestPublicCreateErrorsHaveStableSafeSemantics(t *testing.T) {
	for _, row := range []struct {
		in     error
		status int
	}{{ErrGateDenied, 429}, {ErrAdmissionDenied, 409}, {ErrUnknownInflight, 409}, {ErrIdempotencyConflict, 409}, {ErrRuntimeUnavailable, 503}, {ErrGateUnavailable, 503}, {ErrPolicyInvalid, 503}, {ErrInvalidSnapshot, 503}, {errors.New("password=secret SQL query"), 503}, {apperror.InvalidRequest(nil), 400}} {
		var out *apperror.Error
		if !errors.As(publicCreateError(row.in), &out) || out.HTTPStatus != row.status {
			t.Errorf("%v: %+v", row.in, out)
		}
	}
}
