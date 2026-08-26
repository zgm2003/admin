package user_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/user"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

func TestHandlerUserSuccessContracts(t *testing.T) {
	now := time.Date(2026, 8, 20, 1, 2, 3, 4, time.UTC)
	service := &userHTTPService{
		listResult:  pagination.Result[user.ListItem]{List: []user.ListItem{{ID: 7, Username: "alice", Email: "alice@example.com", IsEnabled: yesno.Yes, Roles: []user.RoleSummary{{ID: 2, Code: "member", Name: "Member", IsEnabled: yesno.Yes}}, CreatedAt: now, UpdatedAt: now}}, Total: 1, Page: 1, PageSize: 20},
		roleOptions: []user.RoleSummary{},
		updated:     user.UpdatedUsername{ID: 7, Username: "alice_new", UpdatedAt: now},
		roles:       user.Roles{User: user.Summary{ID: 7, Username: "alice", Email: "alice@example.com", IsEnabled: yesno.Yes}, Roles: []user.RoleSummary{}, RoleIDs: []int64{}},
		roleCount:   2,
	}
	tests := []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/users?page=1&pageSize=20", ""},
		{http.MethodGet, "/api/v1/users/role-options", ""},
		{http.MethodPut, "/api/v1/users/7", `{"username":"alice_new"}`},
		{http.MethodPatch, "/api/v1/users/7/status", `{"isEnabled":0}`},
		{http.MethodDelete, "/api/v1/users/7", ""},
		{http.MethodGet, "/api/v1/users/7/roles", ""},
		{http.MethodPut, "/api/v1/users/7/roles", `{"roleIds":[5,2,5]}`},
	}
	for _, test := range tests {
		recorder := serveUserRequest(t, service, test.method, test.path, test.body, true)
		assertUserEnvelope(t, recorder, http.StatusOK, 0)
	}
	if service.listQuery != (user.ListQuery{Page: 1, PageSize: 20}) || service.actorID != 41 || service.targetID != 7 || service.updateInput.Username != "alice_new" || service.statusValue != yesno.No || !reflect.DeepEqual(service.roleIDs, []int64{5, 2, 5}) {
		t.Fatalf("service calls = %+v", service)
	}
}

func TestHandlerUserRejectsMalformedQueriesIDsBodiesAndIdentity(t *testing.T) {
	tests := []struct {
		method, path, body string
		actor              bool
	}{
		{http.MethodGet, "/api/v1/users", "", true},
		{http.MethodGet, "/api/v1/users?page=1&pageSize=20&unknown=1", "", true},
		{http.MethodGet, "/api/v1/users?page=1&page=2&pageSize=20", "", true},
		{http.MethodGet, "/api/v1/users?page=1&pageSize=20&isEnabled=2", "", true},
		{http.MethodGet, "/api/v1/users?page=1&pageSize=20&roleId=0", "", true},
		{http.MethodPut, "/api/v1/users/0", `{"username":"alice"}`, true},
		{http.MethodPut, "/api/v1/users/+7", `{"username":"alice"}`, true},
		{http.MethodPut, "/api/v1/users/7", `{}`, true},
		{http.MethodPut, "/api/v1/users/7", `{"username":"alice","email":"x"}`, true},
		{http.MethodPatch, "/api/v1/users/7/status", `{"isEnabled":2}`, true},
		{http.MethodDelete, "/api/v1/users/7", `{}`, true},
		{http.MethodPut, "/api/v1/users/7/roles", `{}`, true},
		{http.MethodPut, "/api/v1/users/7/roles", `{"roleIds":[0]}`, true},
		{http.MethodPut, "/api/v1/users/7", `{"username":"alice"}`, false},
	}
	for _, test := range tests {
		service := &userHTTPService{}
		recorder := serveUserRequest(t, service, test.method, test.path, test.body, test.actor)
		wantStatus, wantCode := http.StatusBadRequest, apperror.CodeInvalidRequest
		if !test.actor {
			wantStatus, wantCode = http.StatusUnauthorized, apperror.CodeUnauthorized
		}
		assertUserEnvelope(t, recorder, wantStatus, wantCode)
		if service.calls != 0 {
			t.Fatalf("invalid request reached service: %s %s", test.method, test.path)
		}
	}
}

func TestRegisterRoutesUsesExactUserPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	permissions := make([]string, 0)
	pass := func(context *gin.Context) { context.Next() }
	user.RegisterRoutes(router.Group("/api/v1"), user.NewHandler(&userHTTPService{}, func(*gin.Context) (int64, bool) { return 41, true }), pass, func(code string) gin.HandlerFunc {
		permissions = append(permissions, code)
		return pass
	})
	want := []string{user.PermissionList, user.PermissionList, user.PermissionUpdate, user.PermissionStatus, user.PermissionDelete, user.PermissionRoles, user.PermissionRoles}
	if !reflect.DeepEqual(permissions, want) {
		t.Fatalf("permissions = %v, want %v", permissions, want)
	}
}

type userHTTPService struct {
	calls             int
	listResult        pagination.Result[user.ListItem]
	listQuery         user.ListQuery
	roleOptions       []user.RoleSummary
	updated           user.UpdatedUsername
	roles             user.Roles
	roleCount         int64
	actorID, targetID int64
	updateInput       user.UpdateInput
	statusValue       yesno.Value
	roleIDs           []int64
}

func (s *userHTTPService) List(_ context.Context, query user.ListQuery) (pagination.Result[user.ListItem], error) {
	s.calls++
	s.listQuery = query
	return s.listResult, nil
}
func (s *userHTTPService) RoleOptions(context.Context) ([]user.RoleSummary, error) {
	s.calls++
	return s.roleOptions, nil
}
func (s *userHTTPService) Update(_ context.Context, actor, target int64, input user.UpdateInput) (user.UpdatedUsername, error) {
	s.calls++
	s.actorID, s.targetID, s.updateInput = actor, target, input
	return s.updated, nil
}
func (s *userHTTPService) UpdateStatus(_ context.Context, actor, target int64, value yesno.Value) error {
	s.calls++
	s.actorID, s.targetID, s.statusValue = actor, target, value
	return nil
}
func (s *userHTTPService) Delete(_ context.Context, actor, target int64) error {
	s.calls++
	s.actorID, s.targetID = actor, target
	return nil
}
func (s *userHTTPService) Roles(_ context.Context, target int64) (user.Roles, error) {
	s.calls++
	s.targetID = target
	return s.roles, nil
}
func (s *userHTTPService) UpdateRoles(_ context.Context, actor, target int64, ids []int64) (int64, error) {
	s.calls++
	s.actorID, s.targetID, s.roleIDs = actor, target, ids
	return s.roleCount, nil
}

func serveUserRequest(t *testing.T, service *userHTTPService, method, path, body string, actor bool) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	pass := func(context *gin.Context) { context.Next() }
	actorReader := func(*gin.Context) (int64, bool) { return 41, actor }
	user.RegisterRoutes(router.Group("/api/v1"), user.NewHandler(service, actorReader), pass, func(string) gin.HandlerFunc { return pass })
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func assertUserEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, status, code int) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope) != 3 {
		t.Fatalf("envelope=%v", envelope)
	}
	var got int
	if err := json.Unmarshal(envelope["code"], &got); err != nil || got != code {
		t.Fatalf("code=%d err=%v", got, err)
	}
}

var _ = bytes.NewReader
