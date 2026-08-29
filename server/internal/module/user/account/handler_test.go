package account_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	projectmiddleware "admin/server/internal/middleware"
	"admin/server/internal/module/user/account"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

func TestHandlerUserSuccessContracts(t *testing.T) {
	now := time.Date(2026, 8, 20, 1, 2, 3, 4, time.UTC)
	phone := "+86 138-0000-0000"
	service := &userHTTPService{
		listResult:  pagination.Result[account.ListItem]{List: []account.ListItem{{ID: 7, Username: "alice", Email: "alice@example.com", Phone: &phone, IsEnabled: yesno.Yes, Roles: []account.RoleSummary{{ID: 2, Code: "member", Name: "Member", IsEnabled: yesno.Yes}}, CreatedAt: now, UpdatedAt: now}}, Total: 1, Page: 1, PageSize: 20},
		roleOptions: []account.RoleSummary{},
		updated:     account.UpdatedProfile{ID: 7, Username: "alice_new", Phone: &phone, UpdatedAt: now},
		roles:       account.Roles{User: account.Summary{ID: 7, Username: "alice", Email: "alice@example.com", Phone: &phone, IsEnabled: yesno.Yes}, Roles: []account.RoleSummary{}, RoleIDs: []int64{}},
		roleCount:   2,
	}
	tests := []struct{ method, path, body string }{
		{http.MethodGet, "/api/admin/v1/users?page=1&pageSize=20", ""},
		{http.MethodGet, "/api/admin/v1/users/role-options", ""},
		{http.MethodPut, "/api/admin/v1/users/7", `{"username":"alice_new","phone":"+86 138-0000-0000"}`},
		{http.MethodPatch, "/api/admin/v1/users/7/status", `{"isEnabled":0}`},
		{http.MethodDelete, "/api/admin/v1/users/7", ""},
		{http.MethodGet, "/api/admin/v1/users/7/roles", ""},
		{http.MethodPut, "/api/admin/v1/users/7/roles", `{"roleIds":[5,2,5]}`},
	}
	for _, test := range tests {
		recorder := serveUserRequest(t, service, test.method, test.path, test.body, true)
		assertUserEnvelope(t, recorder, http.StatusOK, 0)
	}
	assertUserEnvelopeDataJSON(t, serveUserRequest(t, service, http.MethodGet, "/api/admin/v1/users?page=1&pageSize=20", "", true), `{"list":[{"id":7,"username":"alice","email":"alice@example.com","phone":"+86 138-0000-0000","isEnabled":1,"roles":[{"id":2,"code":"member","name":"Member","isEnabled":1}],"createdAt":"2026-08-20T01:02:03.000000004Z","updatedAt":"2026-08-20T01:02:03.000000004Z"}],"total":1,"page":1,"pageSize":20}`)
	assertUserEnvelopeDataJSON(t, serveUserRequest(t, service, http.MethodGet, "/api/admin/v1/users/7/roles", "", true), `{"user":{"id":7,"username":"alice","email":"alice@example.com","phone":"+86 138-0000-0000","isEnabled":1},"roles":[],"roleIds":[]}`)
	assertUserEnvelopeDataJSON(t, serveUserRequest(t, service, http.MethodPut, "/api/admin/v1/users/7", `{"username":"alice_new","phone":"+86 138-0000-0000"}`, true), `{"id":7,"username":"alice_new","phone":"+86 138-0000-0000","updatedAt":"2026-08-20T01:02:03.000000004Z"}`)
	if service.listQuery != (account.ListQuery{Page: 1, PageSize: 20}) || service.actorID != 41 || service.targetID != 7 || service.updateInput.Username != "alice_new" || service.updateInput.Phone == nil || *service.updateInput.Phone != phone || service.statusValue != yesno.No || !reflect.DeepEqual(service.roleIDs, []int64{5, 2, 5}) {
		t.Fatalf("service calls = %+v", service)
	}
}

func TestHandlerUserUpdateRequiresNullablePhoneAndReturnsClosedProfiles(t *testing.T) {
	now := time.Date(2026, 8, 20, 1, 2, 3, 4, time.UTC)
	service := &userHTTPService{
		listResult: pagination.Result[account.ListItem]{List: []account.ListItem{{ID: 7, Username: "alice", Email: "alice@example.com", IsEnabled: yesno.Yes, Roles: []account.RoleSummary{}, CreatedAt: now, UpdatedAt: now}}, Total: 1, Page: 1, PageSize: 20},
		updated:    account.UpdatedProfile{ID: 7, Username: "alice", UpdatedAt: now},
		roles:      account.Roles{User: account.Summary{ID: 7, Username: "alice", Email: "alice@example.com", IsEnabled: yesno.Yes}, Roles: []account.RoleSummary{}, RoleIDs: []int64{}},
	}

	for _, test := range []struct {
		body       string
		wantStatus int
	}{
		{`{"username":"alice","phone":"+86 138-0000-0000"}`, http.StatusOK},
		{`{"username":"alice","phone":null}`, http.StatusOK},
		{`{"username":"alice"}`, http.StatusBadRequest},
		{`{"username":"alice","phone":""}`, http.StatusBadRequest},
		{`{"username":"alice","phone":"123\n456"}`, http.StatusBadRequest},
	} {
		recorder := serveUserRequest(t, service, http.MethodPut, "/api/admin/v1/users/7", test.body, true)
		assertUserEnvelope(t, recorder, test.wantStatus, map[bool]int{true: 0, false: apperror.CodeInvalidRequest}[test.wantStatus == http.StatusOK])
	}

	assertUserEnvelopeDataJSON(t, serveUserRequest(t, service, http.MethodGet, "/api/admin/v1/users?page=1&pageSize=20", "", true), `{"list":[{"id":7,"username":"alice","email":"alice@example.com","phone":null,"isEnabled":1,"roles":[],"createdAt":"2026-08-20T01:02:03.000000004Z","updatedAt":"2026-08-20T01:02:03.000000004Z"}],"total":1,"page":1,"pageSize":20}`)
	assertUserEnvelopeDataJSON(t, serveUserRequest(t, service, http.MethodGet, "/api/admin/v1/users/7/roles", "", true), `{"user":{"id":7,"username":"alice","email":"alice@example.com","phone":null,"isEnabled":1},"roles":[],"roleIds":[]}`)
	assertUserEnvelopeDataJSON(t, serveUserRequest(t, service, http.MethodPut, "/api/admin/v1/users/7", `{"username":"alice","phone":null}`, true), `{"id":7,"username":"alice","phone":null,"updatedAt":"2026-08-20T01:02:03.000000004Z"}`)
}

func TestHandlerUserUpdateLogsProfileOperationWithoutPhone(t *testing.T) {
	const phone = "+86 138-0000-0000"
	now := time.Date(2026, 8, 20, 1, 2, 3, 4, time.UTC)
	service := &userHTTPService{updated: account.UpdatedProfile{ID: 7, Username: "alice", Phone: pointerToUserHandler(phone), UpdatedAt: now}}
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(projectmiddleware.AccessLog(logger))
	pass := func(context *gin.Context) { context.Next() }
	account.RegisterRoutes(router.Group("/api/admin/v1"), account.NewHandler(service, func(*gin.Context) (int64, bool) { return 41, true }), pass, func(string) gin.HandlerFunc { return pass })
	request := httptest.NewRequest(http.MethodPut, "/api/admin/v1/users/7", strings.NewReader(`{"username":"alice","phone":"+86 138-0000-0000"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	assertUserEnvelope(t, recorder, http.StatusOK, 0)

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("decode access log: %v; output=%s", err, output.String())
	}
	if entry["operation"] != "user.profile.update" || entry["actorUserId"] != float64(41) || entry["targetUserId"] != float64(7) {
		t.Fatalf("access log entry = %#v", entry)
	}
	if strings.Contains(output.String(), phone) {
		t.Fatalf("access log leaked phone: %s", output.String())
	}
}

func TestHandlerUserRejectsMalformedQueriesIDsBodiesAndIdentity(t *testing.T) {
	tests := []struct {
		method, path, body string
		actor              bool
	}{
		{http.MethodGet, "/api/admin/v1/users", "", true},
		{http.MethodGet, "/api/admin/v1/users?page=1&pageSize=20&unknown=1", "", true},
		{http.MethodGet, "/api/admin/v1/users?page=1&page=2&pageSize=20", "", true},
		{http.MethodGet, "/api/admin/v1/users?page=1&pageSize=20&isEnabled=2", "", true},
		{http.MethodGet, "/api/admin/v1/users?page=1&pageSize=20&roleId=0", "", true},
		{http.MethodPut, "/api/admin/v1/users/0", `{"username":"alice","phone":null}`, true},
		{http.MethodPut, "/api/admin/v1/users/+7", `{"username":"alice","phone":null}`, true},
		{http.MethodPut, "/api/admin/v1/users/7", `{}`, true},
		{http.MethodPut, "/api/admin/v1/users/7", `{"username":"alice","email":"x"}`, true},
		{http.MethodPatch, "/api/admin/v1/users/7/status", `{"isEnabled":2}`, true},
		{http.MethodDelete, "/api/admin/v1/users/7", `{}`, true},
		{http.MethodPut, "/api/admin/v1/users/7/roles", `{}`, true},
		{http.MethodPut, "/api/admin/v1/users/7/roles", `{"roleIds":[0]}`, true},
		{http.MethodPut, "/api/admin/v1/users/7", `{"username":"alice","phone":null}`, false},
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
	account.RegisterRoutes(router.Group("/api/admin/v1"), account.NewHandler(&userHTTPService{}, func(*gin.Context) (int64, bool) { return 41, true }), pass, func(code string) gin.HandlerFunc {
		permissions = append(permissions, code)
		return pass
	})
	want := []string{account.PermissionList, account.PermissionList, account.PermissionUpdate, account.PermissionStatus, account.PermissionDelete, account.PermissionRoles, account.PermissionRoles}
	if !reflect.DeepEqual(permissions, want) {
		t.Fatalf("permissions = %v, want %v", permissions, want)
	}
}

type userHTTPService struct {
	calls             int
	listResult        pagination.Result[account.ListItem]
	listQuery         account.ListQuery
	roleOptions       []account.RoleSummary
	updated           account.UpdatedProfile
	roles             account.Roles
	roleCount         int64
	actorID, targetID int64
	updateInput       account.UpdateInput
	statusValue       yesno.Value
	roleIDs           []int64
}

func (s *userHTTPService) List(_ context.Context, query account.ListQuery) (pagination.Result[account.ListItem], error) {
	s.calls++
	s.listQuery = query
	return s.listResult, nil
}
func (s *userHTTPService) RoleOptions(context.Context) ([]account.RoleSummary, error) {
	s.calls++
	return s.roleOptions, nil
}
func (s *userHTTPService) Update(_ context.Context, actor, target int64, input account.UpdateInput) (account.UpdatedProfile, error) {
	s.calls++
	s.actorID, s.targetID, s.updateInput = actor, target, input
	if _, err := account.NormalizePhone(input.Phone); err != nil {
		return account.UpdatedProfile{}, apperror.InvalidRequest(err)
	}
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
func (s *userHTTPService) Roles(_ context.Context, target int64) (account.Roles, error) {
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
	account.RegisterRoutes(router.Group("/api/admin/v1"), account.NewHandler(service, actorReader), pass, func(string) gin.HandlerFunc { return pass })
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

func assertUserEnvelopeDataJSON(t *testing.T, recorder *httptest.ResponseRecorder, want string) {
	t.Helper()
	assertUserEnvelope(t, recorder, http.StatusOK, 0)
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	var gotValue, wantValue any
	if err := json.Unmarshal(envelope.Data, &gotValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("data=%s want=%s", envelope.Data, want)
	}
}

func pointerToUserHandler(value string) *string {
	return &value
}
