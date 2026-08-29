package operationlog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeOperationLogService struct {
	result ListResult
	query  ListQuery
	err    error
}

func (f *fakeOperationLogService) List(_ context.Context, query ListQuery) (ListResult, error) {
	f.query = query
	return f.result, f.err
}

func (f *fakeOperationLogService) Process(context.Context, TaskPayload) error {
	return nil
}

func TestListRejectsInvalidPaginationAndSuccessCode(t *testing.T) {
	for _, target := range []string{
		"/api/admin/v1/operation-logs?page=0&pageSize=20",
		"/api/admin/v1/operation-logs?page=1&pageSize=101",
		"/api/admin/v1/operation-logs?page=1&pageSize=20&isSuccess=2",
		"/api/admin/v1/operation-logs?page=1&pageSize=20&from=2026-08-22T00:00:00Z&to=2026-08-21T00:00:00Z",
	} {
		service := &fakeOperationLogService{}
		router := gin.New()
		RegisterRoutes(router.Group("/api/admin/v1"), NewHandler(service), func(context *gin.Context) { context.Next() }, func(string) gin.HandlerFunc { return func(context *gin.Context) { context.Next() } })
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("%s status = %d, want 400; body = %s", target, recorder.Code, recorder.Body.String())
		}
	}
}

func TestListReturnsTypedEnvelope(t *testing.T) {
	service := &fakeOperationLogService{result: ListResult{List: []Item{{RequestID: "request-1", Action: "user.update"}}, Total: 1, Page: 1, PageSize: 20}}
	router := gin.New()
	RegisterRoutes(router.Group("/api/admin/v1"), NewHandler(service), func(context *gin.Context) { context.Next() }, func(string) gin.HandlerFunc { return func(context *gin.Context) { context.Next() } })
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/v1/operation-logs?page=1&pageSize=20", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":0`) || !strings.Contains(body, `"data"`) || !strings.Contains(body, `"message":"ok"`) {
		t.Fatalf("envelope = %s", body)
	}
	if strings.Contains(body, `"msg"`) {
		t.Fatalf("compatibility msg field found: %s", body)
	}
}

func TestRegisterRoutesRequiresOperationLogListPermission(t *testing.T) {
	router := gin.New()
	var permission string
	RegisterRoutes(router.Group("/api/admin/v1"), NewHandler(&fakeOperationLogService{}), func(context *gin.Context) { context.Next() }, func(value string) gin.HandlerFunc {
		permission = value
		return func(context *gin.Context) { context.Next() }
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/v1/operation-logs?page=1&pageSize=20", nil))
	if permission != PermissionList {
		t.Fatalf("permission = %q", permission)
	}
}
