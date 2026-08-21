package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeSessionAdminService struct {
	listContext context.Context
	listRows    []AdminSession
	revokeCalls int
}

func (f *fakeSessionAdminService) ListSessions(ctx context.Context, query AdminSessionQuery) ([]AdminSession, int64, error) {
	f.listContext = ctx
	return f.listRows, int64(len(f.listRows)), nil
}

func (f *fakeSessionAdminService) SessionStats(context.Context) (AdminSessionStats, error) {
	return AdminSessionStats{Platforms: map[string]int64{}}, nil
}

func (f *fakeSessionAdminService) RevokeSession(context.Context, Identity, int64) (AdminRevokeResult, error) {
	f.revokeCalls++
	return AdminRevokeResult{}, nil
}

func (f *fakeSessionAdminService) RevokeSessions(context.Context, Identity, []int64) (AdminRevokeResult, error) {
	f.revokeCalls++
	return AdminRevokeResult{}, nil
}

func TestSessionRoutesUseExactPermissions(t *testing.T) {
	router := gin.New()
	service := &fakeSessionAdminService{}
	var permissions []string
	RegisterSessionAdminRoutes(router.Group("/api/v1"), NewSessionAdminHandler(service), func(context *gin.Context) {
		context.Set(identityContextKey, Identity{UserID: 1, SessionID: 2, Platform: "admin", Version: 1})
		context.Next()
	}, func(permission string) gin.HandlerFunc {
		permissions = append(permissions, permission)
		return func(context *gin.Context) { context.Next() }
	})
	paths := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/sessions?page=1&pageSize=20"},
		{http.MethodGet, "/api/v1/sessions/stats"},
		{http.MethodDelete, "/api/v1/sessions/7"},
		{http.MethodDelete, "/api/v1/sessions"},
	}
	for _, path := range paths {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(path.method, path.path, strings.NewReader(`{"ids":[7]}`))
		if path.method == http.MethodDelete && path.path != "/api/v1/sessions" {
			request = httptest.NewRequest(path.method, path.path, nil)
		}
		router.ServeHTTP(recorder, request)
	}
	want := []string{"system:session:list", "system:session:list", "system:session:revoke", "system:session:revoke"}
	if len(permissions) != len(want) {
		t.Fatalf("permissions = %v", permissions)
	}
	for index := range want {
		if permissions[index] != want[index] {
			t.Fatalf("permission[%d] = %q, want %q", index, permissions[index], want[index])
		}
	}
}

func TestBulkRevokeRequiresExactIDsBody(t *testing.T) {
	for _, body := range []string{`{}`, `{"ids":[]}`, `{"ids":[1],"unknown":true}`} {
		router := gin.New()
		router.DELETE("/api/v1/sessions", NewSessionAdminHandler(&fakeSessionAdminService{}).RevokeMany)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("body %s status = %d, want 400", body, recorder.Code)
		}
	}
	ids := make([]string, 101)
	for index := range ids {
		ids[index] = "1"
	}
	router := gin.New()
	router.DELETE("/api/v1/sessions", NewSessionAdminHandler(&fakeSessionAdminService{}).RevokeMany)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions", strings.NewReader(`{"ids":[`+strings.Join(ids, ",")+`]}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("over limit status = %d, want 400", recorder.Code)
	}
}

func TestSessionHandlerUsesRequestContext(t *testing.T) {
	service := &fakeSessionAdminService{}
	handler := NewSessionAdminHandler(service)
	router := gin.New()
	router.GET("/api/v1/sessions", func(context *gin.Context) {
		context.Set(identityContextKey, Identity{UserID: 1, SessionID: 2, Platform: "admin", Version: 1})
		context.Next()
	}, handler.List)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/sessions?page=1&pageSize=20", nil).WithContext(context.WithValue(context.Background(), struct{}{}, "request"))
	router.ServeHTTP(httptest.NewRecorder(), request)
	if service.listContext == nil || service.listContext.Value(struct{}{}) != "request" {
		t.Fatal("request context was not passed to service")
	}
}

func TestSessionListMarksCurrentSession(t *testing.T) {
	service := &fakeSessionAdminService{listRows: []AdminSession{
		{ID: 2, UserID: 1, Username: "admin", Platform: "admin"},
		{ID: 3, UserID: 2, Username: "member", Platform: "admin"},
	}}
	handler := NewSessionAdminHandler(service)
	router := gin.New()
	router.GET("/api/v1/sessions", func(context *gin.Context) {
		context.Set(identityContextKey, Identity{UserID: 1, SessionID: 2, Platform: "admin", Version: 1})
		context.Next()
	}, handler.List)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/sessions?page=1&pageSize=20", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"status":"","isCurrent":true`) {
		t.Fatalf("current session was not marked: %s", body)
	}
	if !strings.Contains(body, `"status":"","isCurrent":false`) {
		t.Fatalf("other session was not marked: %s", body)
	}
}
