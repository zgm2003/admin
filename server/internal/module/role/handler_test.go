package role_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"admin/server/internal/module/role"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

func TestRoleHandlerListParsesOnlyTheExactQuery(t *testing.T) {
	service := &roleHTTPService{listResult: pagination.Result[role.ListItem]{
		List: []role.ListItem{}, Total: 0, Page: 1, PageSize: 20,
	}}
	recorder := serveRoleRequest(t, service, http.MethodGet, "/api/v1/roles?page=1&pageSize=20&keyword=test&isEnabled=0", nil)
	assertRoleEnvelope(t, recorder, http.StatusOK, 0)
	if service.listCalls != 1 || service.listQuery.Page != 1 || service.listQuery.PageSize != 20 ||
		service.listQuery.Keyword != "test" || service.listQuery.IsEnabled == nil || *service.listQuery.IsEnabled != yesno.No {
		t.Fatalf("list query = %+v calls=%d", service.listQuery, service.listCalls)
	}

	for _, path := range []string{
		"/api/v1/roles",
		"/api/v1/roles?page=0&pageSize=20",
		"/api/v1/roles?page=1&pageSize=101",
		"/api/v1/roles?page=1&pageSize=20&unknown=1",
		"/api/v1/roles?page=1&page=2&pageSize=20",
		"/api/v1/roles?page=1&pageSize=20&isEnabled=2",
	} {
		invalidService := &roleHTTPService{}
		responseRecorder := serveRoleRequest(t, invalidService, http.MethodGet, path, nil)
		assertRoleEnvelope(t, responseRecorder, http.StatusBadRequest, apperror.CodeInvalidRequest)
		if invalidService.listCalls != 0 {
			t.Fatalf("invalid query %q reached Service", path)
		}
	}
}

func TestRoleHandlerMutationsUseClosedContracts(t *testing.T) {
	service := &roleHTTPService{createID: 7, permissionCount: 1}
	tests := []struct {
		method string
		path   string
		body   []byte
		status int
	}{
		{method: http.MethodPost, path: "/api/v1/roles", body: []byte(`{"code":"tester","name":"Tester"}`), status: http.StatusCreated},
		{method: http.MethodPut, path: "/api/v1/roles/7", body: []byte(`{"name":"Updated"}`), status: http.StatusOK},
		{method: http.MethodPatch, path: "/api/v1/roles/7/status", body: []byte(`{"isEnabled":0}`), status: http.StatusOK},
		{method: http.MethodPatch, path: "/api/v1/roles/7/default", status: http.StatusOK},
		{method: http.MethodDelete, path: "/api/v1/roles/7", status: http.StatusOK},
		{method: http.MethodPut, path: "/api/v1/roles/7/permissions", body: []byte(`{"menuIds":[2,3]}`), status: http.StatusOK},
	}
	for _, request := range tests {
		recorder := serveRoleRequest(t, service, request.method, request.path, request.body)
		assertRoleEnvelope(t, recorder, request.status, 0)
	}
	if service.createInput != (role.CreateInput{Code: "tester", Name: "Tester"}) ||
		service.updateID != 7 || service.updateInput.Name != "Updated" ||
		service.statusID != 7 || service.statusValue != yesno.No ||
		service.defaultID != 7 || service.deleteID != 7 ||
		service.permissionID != 7 || !reflect.DeepEqual(service.permissionIDs, []int64{2, 3}) {
		t.Fatalf("mutation calls = %+v", service)
	}

	invalid := []struct {
		method string
		path   string
		body   []byte
	}{
		{method: http.MethodPost, path: "/api/v1/roles", body: []byte(`{"code":"tester"}`)},
		{method: http.MethodPost, path: "/api/v1/roles", body: []byte(`{"code":"tester","name":"Tester","msg":"old"}`)},
		{method: http.MethodPut, path: "/api/v1/roles/0", body: []byte(`{"name":"Tester"}`)},
		{method: http.MethodPatch, path: "/api/v1/roles/7/status", body: []byte(`{"isEnabled":2}`)},
		{method: http.MethodPatch, path: "/api/v1/roles/7/default", body: []byte(`{}`)},
		{method: http.MethodDelete, path: "/api/v1/roles/7", body: []byte(`{}`)},
		{method: http.MethodPut, path: "/api/v1/roles/7/permissions", body: []byte(`{"menuIds":null}`)},
		{method: http.MethodPut, path: "/api/v1/roles/7/permissions", body: []byte(`{"menuIds":[2,2]}`)},
	}
	for _, request := range invalid {
		recorder := serveRoleRequest(t, &roleHTTPService{}, request.method, request.path, request.body)
		assertRoleEnvelope(t, recorder, http.StatusBadRequest, apperror.CodeInvalidRequest)
	}
}

func TestRoleRoutesBindExactPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	permissions := make([]string, 0)
	pass := func(context *gin.Context) { context.Next() }
	role.RegisterRoutes(router.Group("/api/v1"), role.NewHandler(&roleHTTPService{}), pass, func(code string) gin.HandlerFunc {
		permissions = append(permissions, code)
		return pass
	})
	want := []string{
		role.PermissionList,
		role.PermissionCreate,
		role.PermissionUpdate,
		role.PermissionStatus,
		role.PermissionDefault,
		role.PermissionDelete,
		role.PermissionAuthorize,
		role.PermissionAuthorize,
	}
	if !reflect.DeepEqual(permissions, want) {
		t.Fatalf("registered permissions = %v, want %v", permissions, want)
	}
}

type roleHTTPService struct {
	listResult      pagination.Result[role.ListItem]
	listQuery       role.ListQuery
	listCalls       int
	createID        int64
	createInput     role.CreateInput
	updateID        int64
	updateInput     role.UpdateInput
	statusID        int64
	statusValue     yesno.Value
	defaultID       int64
	deleteID        int64
	permissionID    int64
	permissionIDs   []int64
	permissionCount int64
}

func (s *roleHTTPService) List(_ context.Context, query role.ListQuery) (pagination.Result[role.ListItem], error) {
	s.listCalls++
	s.listQuery = query
	return s.listResult, nil
}
func (s *roleHTTPService) Create(_ context.Context, input role.CreateInput) (int64, error) {
	s.createInput = input
	return s.createID, nil
}
func (s *roleHTTPService) Update(_ context.Context, id int64, input role.UpdateInput) error {
	s.updateID = id
	s.updateInput = input
	return nil
}
func (s *roleHTTPService) UpdateStatus(_ context.Context, id int64, value yesno.Value) error {
	s.statusID = id
	s.statusValue = value
	return nil
}
func (s *roleHTTPService) SetDefault(_ context.Context, id int64) error { s.defaultID = id; return nil }
func (s *roleHTTPService) Delete(_ context.Context, id int64) error     { s.deleteID = id; return nil }
func (s *roleHTTPService) Permissions(_ context.Context, _ int64) (role.Permissions, error) {
	return role.Permissions{MenuTree: []role.PermissionTreeNode{}, MenuIDs: []int64{}}, nil
}
func (s *roleHTTPService) UpdatePermissions(_ context.Context, id int64, ids []int64) (int64, error) {
	s.permissionID = id
	s.permissionIDs = ids
	return s.permissionCount, nil
}

func serveRoleRequest(t *testing.T, service *roleHTTPService, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	pass := func(context *gin.Context) { context.Next() }
	role.RegisterRoutes(router.Group("/api/v1"), role.NewHandler(service), pass, func(string) gin.HandlerFunc { return pass })
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func assertRoleEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus, wantCode int) {
	t.Helper()
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope) != 3 {
		t.Fatalf("envelope = %v", envelope)
	}
	var code int
	if err := json.Unmarshal(envelope["code"], &code); err != nil || code != wantCode {
		t.Fatalf("code = %d error=%v", code, err)
	}
}
