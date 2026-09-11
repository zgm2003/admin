package recipientRule

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

type stubService struct {
	createCalls int
	createIn    CreateInput
	updateCalls int
	updateIn    UpdateInput
	statusCalls int
	statusVal   yesno.Value
	deleteCalls int
	deleteID    int64
	writeErr    error
}

func (s *stubService) List(context.Context) ([]Safe, error) { return []Safe{}, nil }

func (s *stubService) Create(_ context.Context, input CreateInput) (Safe, error) {
	s.createCalls++
	s.createIn = input
	return Safe{}, s.writeErr
}

func (s *stubService) Update(_ context.Context, id int64, input UpdateInput) (Safe, error) {
	s.updateCalls++
	s.updateIn = input
	return Safe{}, s.writeErr
}

func (s *stubService) UpdateStatus(_ context.Context, id int64, status yesno.Value) error {
	s.statusCalls++
	s.statusVal = status
	return s.writeErr
}

func (s *stubService) Delete(_ context.Context, id int64) error {
	s.deleteCalls++
	s.deleteID = id
	return s.writeErr
}

func TestRegisterRoutesBindsExactRulePermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	seen := make(map[string]int, 5)
	RegisterRoutes(router.Group("/api/admin/v1/message/sms"), NewHandler(&stubService{}),
		func(c *gin.Context) { c.Next() },
		func(code string) gin.HandlerFunc {
			return func(c *gin.Context) {
				seen[code]++
				c.Next()
			}
		})

	for _, request := range []struct{ method, path string }{
		{http.MethodGet, "/api/admin/v1/message/sms/recipient-rule"},
		{http.MethodPost, "/api/admin/v1/message/sms/recipient-rule"},
		{http.MethodPut, "/api/admin/v1/message/sms/recipient-rule/3"},
		{http.MethodPatch, "/api/admin/v1/message/sms/recipient-rule/3/status"},
		{http.MethodDelete, "/api/admin/v1/message/sms/recipient-rule/3"},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(request.method, request.path, nil))
	}
	for _, permission := range []string{PermissionList, PermissionCreate, PermissionUpdate, PermissionStatus, PermissionDelete} {
		if seen[permission] != 1 {
			t.Fatalf("permission %q registered %d times", permission, seen[permission])
		}
	}
}

func TestCreateRequiresEveryField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubService{}
	router := gin.New()
	router.POST("/recipient-rule", NewHandler(service).Create)

	for _, body := range []string{
		`{"scope":"phone","pattern":"+8615671628271","action":"deny","name":"n","isEnabled":1}`,
		`{"scope":"phone","pattern":"+8615671628271","action":"deny","name":"n","remark":"","isEnabled":1,"extra":1}`,
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/recipient-rule", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body)
		}
	}
	if service.createCalls != 0 {
		t.Fatal("invalid create reached the service")
	}

	recorder := httptest.NewRecorder()
	valid := httptest.NewRequest(http.MethodPost, "/recipient-rule", strings.NewReader(
		`{"scope":"prefix","pattern":"+86156","action":"allow","name":"白名单","remark":"","isEnabled":1}`))
	valid.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, valid)
	if recorder.Code != http.StatusCreated || service.createIn.Pattern != "+86156" || service.createIn.Scope != ScopePrefix {
		t.Fatalf("status = %d input = %+v", recorder.Code, service.createIn)
	}
}

func TestUpdateAllowsAnOmittedPattern(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubService{}
	router := gin.New()
	router.PUT("/recipient-rule/:id", NewHandler(service).Update)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/recipient-rule/7", strings.NewReader(
		`{"scope":"phone","action":"deny","name":"改名","remark":"","isEnabled":1}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || service.updateIn.Pattern != nil {
		t.Fatalf("status = %d pattern = %v", recorder.Code, service.updateIn.Pattern)
	}

	withPattern := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPut, "/recipient-rule/7", strings.NewReader(
		`{"scope":"prefix","pattern":"+86156","action":"deny","name":"改名","remark":"","isEnabled":1}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(withPattern, request)
	if withPattern.Code != http.StatusOK || service.updateIn.Pattern == nil || *service.updateIn.Pattern != "+86156" {
		t.Fatalf("status = %d pattern = %v", withPattern.Code, service.updateIn.Pattern)
	}
}

func TestDeleteRejectsANonEmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubService{}
	router := gin.New()
	router.DELETE("/recipient-rule/:id", NewHandler(service).Delete)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/recipient-rule/3", strings.NewReader(`{"force":true}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || service.deleteCalls != 0 {
		t.Fatalf("status = %d calls = %d", recorder.Code, service.deleteCalls)
	}
}
