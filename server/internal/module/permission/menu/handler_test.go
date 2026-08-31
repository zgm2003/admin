package menu_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"admin/server/internal/module/permission/menu"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

func TestMenuHandlerListReturnsClosedTreeResponse(t *testing.T) {
	now := time.Date(2026, 8, 19, 2, 0, 0, 0, time.UTC)
	service := &menuHTTPService{listResult: menu.Catalog{
		Platforms: []menu.PlatformOption{{ID: 1, Code: "admin", Name: "Admin", IsEnabled: yesno.Yes}},
		MenuTree: []menu.ManagedMenu{{
			ID: 1, PlatformID: 1, PlatformCode: "admin", PlatformName: "Admin", MenuType: menu.TypeDirectory, Name: "报表", Code: "reports", I18nKey: menuTestStringPointer("navigation.system"),
			SortOrder: 10, IsEnabled: yesno.Yes, IsHidden: yesno.No, CreatedAt: now, UpdatedAt: now,
			IsProtected: true, Children: []menu.ManagedMenu{},
		}}}}
	recorder := serveMenuRequest(t, service, http.MethodGet, "/api/admin/v1/menus?platformId=1", nil)
	assertMenuEnvelope(t, recorder, http.StatusOK, 0)

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(envelope["data"], &data); err != nil || len(data) != 2 || data["platforms"] == nil || data["menuTree"] == nil {
		t.Fatalf("data = %s error=%v", envelope["data"], err)
	}
	var platforms []map[string]json.RawMessage
	if err := json.Unmarshal(data["platforms"], &platforms); err != nil || len(platforms) != 1 || len(platforms[0]) != 4 {
		t.Fatalf("platforms = %s error=%v", data["platforms"], err)
	}
	for _, key := range []string{"id", "code", "name", "isEnabled"} {
		if platforms[0][key] == nil {
			t.Fatalf("platform response missing %q", key)
		}
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(data["menuTree"], &rows); err != nil || len(rows) != 1 {
		t.Fatalf("menuTree = %s error=%v", data["menuTree"], err)
	}
	wantKeys := []string{"id", "platformId", "platformCode", "platformName", "parentId", "menuType", "name", "code", "i18nKey", "path", "componentPath", "icon", "sortOrder", "isEnabled", "isHidden", "createdAt", "updatedAt", "isProtected", "children"}
	if len(rows[0]) != len(wantKeys) {
		t.Fatalf("menu response keys = %v", rows[0])
	}
	for _, key := range wantKeys {
		if rows[0][key] == nil {
			t.Fatalf("menu response missing %q", key)
		}
	}
	var children []json.RawMessage
	if err := json.Unmarshal(rows[0]["children"], &children); err != nil || children == nil || len(children) != 0 {
		t.Fatalf("leaf children = %s error=%v", rows[0]["children"], err)
	}
	var isProtected int16
	if err := json.Unmarshal(rows[0]["isProtected"], &isProtected); err != nil || isProtected != int16(yesno.Yes) {
		t.Fatalf("isProtected = %s error=%v", rows[0]["isProtected"], err)
	}
	if service.listContext == nil {
		t.Fatal("handler did not pass the request context")
	}
	if service.listQuery.PlatformID == nil || *service.listQuery.PlatformID != 1 {
		t.Fatalf("list query = %+v", service.listQuery)
	}
}

func TestMenuHandlerListRejectsInvalidPlatformQuery(t *testing.T) {
	for _, path := range []string{
		"/api/admin/v1/menus?platformId=0",
		"/api/admin/v1/menus?platformId=abc",
		"/api/admin/v1/menus?platformId=1&platformId=2",
		"/api/admin/v1/menus?platformId=1&unknown=true",
	} {
		service := &menuHTTPService{}
		recorder := serveMenuRequest(t, service, http.MethodGet, path, nil)
		assertMenuEnvelope(t, recorder, http.StatusBadRequest, apperror.CodeInvalidRequest)
		if service.listCalls != 0 {
			t.Fatalf("invalid query reached Service: %s", path)
		}
	}
}

func TestMenuHandlerCreateRequiresEveryFieldAndExplicitNull(t *testing.T) {
	valid := `{"platformId":2,"parentId":null,"menuType":"directory","name":"报表","code":"reports","i18nKey":"reports.root","path":null,"componentPath":null,"icon":"lucide:folder","sortOrder":0,"isEnabled":0,"isHidden":0}`
	service := &menuHTTPService{createID: 41}
	recorder := serveMenuRequest(t, service, http.MethodPost, "/api/admin/v1/menus", []byte(valid))
	assertMenuEnvelope(t, recorder, http.StatusCreated, 0)
	if service.createCalls != 1 || service.createInput.PlatformID != 2 || service.createInput.ParentID != nil || service.createInput.SortOrder != 0 || service.createInput.IsEnabled != yesno.No || service.createInput.IsHidden != yesno.No {
		t.Fatalf("create input = %+v calls=%d", service.createInput, service.createCalls)
	}
	assertMutationData(t, recorder, map[string]float64{"id": 41})

	invalidBodies := []string{
		`{"platformId":0,"parentId":null,"menuType":"directory","name":"报表","code":"reports","i18nKey":"reports.root","path":null,"componentPath":null,"icon":"lucide:folder","sortOrder":0,"isEnabled":0,"isHidden":0}`,
		`{"menuType":"directory","code":"reports","i18nKey":"reports.root","path":null,"componentPath":null,"icon":"Folder","sortOrder":0,"isEnabled":0,"isHidden":0}`,
		`{"parentId":0,"menuType":"directory","code":"reports","i18nKey":"reports.root","path":null,"componentPath":null,"icon":"Folder","sortOrder":0,"isEnabled":0,"isHidden":0}`,
		`{"parentId":"1","menuType":"page","code":"reports:list","i18nKey":"reports.list","path":"/reports","componentPath":"reports","icon":null,"sortOrder":0,"isEnabled":1,"isHidden":0}`,
		`{"parentId":null,"menuType":null,"code":"reports","i18nKey":"reports.root","path":null,"componentPath":null,"icon":null,"sortOrder":0,"isEnabled":1,"isHidden":0}`,
		`{"parentId":null,"menuType":"directory","code":"reports","i18nKey":"reports.root","path":null,"componentPath":null,"icon":null,"sortOrder":null,"isEnabled":1,"isHidden":0}`,
		`{"parentId":null,"menuType":"directory","code":"reports","i18nKey":"reports.root","path":null,"componentPath":null,"icon":null,"sortOrder":0,"isEnabled":2,"isHidden":0}`,
		`{"parentId":null,"menuType":"directory","code":"reports","i18nKey":"reports.root","path":null,"componentPath":null,"icon":null,"sortOrder":0,"isEnabled":1,"isHidden":2}`,
		`{"parentId":null,"menuType":"directory","code":"reports","i18nKey":"reports.root","path":null,"componentPath":null,"icon":null,"sortOrder":0,"isEnabled":1,"isHidden":0,"unknown":true}`,
		`{"parentId":null,"menuType":"directory","code":"reports","i18nKey":"reports.root","path":null,"componentPath":null,"icon":null,"sortOrder":0,"isEnabled":1,"isHidden":0,"msg":"old"}`,
		valid + ` {}`,
	}
	for index, body := range invalidBodies {
		t.Run(fmt.Sprintf("invalid-%d", index), func(t *testing.T) {
			invalidService := &menuHTTPService{}
			responseRecorder := serveMenuRequest(t, invalidService, http.MethodPost, "/api/admin/v1/menus", []byte(body))
			assertMenuEnvelope(t, responseRecorder, http.StatusBadRequest, apperror.CodeInvalidRequest)
			if invalidService.createCalls != 0 {
				t.Fatal("invalid request reached Service")
			}
		})
	}
}

func TestMenuHandlerUpdateStatusAndDeleteUseExactContracts(t *testing.T) {
	service := &menuHTTPService{}
	updateBody := []byte(`{"parentId":1,"menuType":"page","name":"报表列表","i18nKey":"reports.list","path":"/reports","componentPath":"reports","icon":null,"sortOrder":10,"isHidden":0}`)
	recorder := serveMenuRequest(t, service, http.MethodPut, "/api/admin/v1/menus/7", updateBody)
	assertMenuEnvelope(t, recorder, http.StatusOK, 0)
	if service.updateID != 7 || service.updateInput.ParentID == nil || *service.updateInput.ParentID != 1 {
		t.Fatalf("update = id %d input %+v", service.updateID, service.updateInput)
	}
	assertMutationData(t, recorder, map[string]float64{"id": 7})

	recorder = serveMenuRequest(t, service, http.MethodPatch, "/api/admin/v1/menus/7/status", []byte(`{"isEnabled":0}`))
	assertMenuEnvelope(t, recorder, http.StatusOK, 0)
	if service.statusID != 7 || service.statusValue != yesno.No {
		t.Fatalf("status = id %d value %d", service.statusID, service.statusValue)
	}
	assertMutationData(t, recorder, map[string]float64{"id": 7, "isEnabled": 0})

	recorder = serveMenuRequest(t, service, http.MethodDelete, "/api/admin/v1/menus/7", nil)
	assertMenuEnvelope(t, recorder, http.StatusOK, 0)
	if service.deleteID != 7 {
		t.Fatalf("delete id = %d", service.deleteID)
	}
	assertMutationData(t, recorder, map[string]float64{"id": 7})

	invalid := []struct {
		method string
		path   string
		body   []byte
	}{
		{method: http.MethodPut, path: "/api/admin/v1/menus/0", body: updateBody},
		{method: http.MethodPut, path: "/api/admin/v1/menus/7", body: []byte(`{"parentId":1,"menuType":"page","code":"forbidden","i18nKey":"reports.list","path":"/reports","componentPath":"reports","icon":null,"sortOrder":10,"isHidden":0}`)},
		{method: http.MethodPut, path: "/api/admin/v1/menus/7", body: []byte(`{"platformId":2,"parentId":1,"menuType":"page","name":"报表列表","i18nKey":"reports.list","path":"/reports","componentPath":"reports","icon":null,"sortOrder":10,"isHidden":0}`)},
		{method: http.MethodPut, path: "/api/admin/v1/menus/7", body: []byte(`{"parentId":1,"menuType":"page","i18nKey":"reports.list","path":"/reports","componentPath":"reports","icon":null,"sortOrder":10,"isHidden":0,"isEnabled":1}`)},
		{method: http.MethodPatch, path: "/api/admin/v1/menus/7/status", body: []byte(`{"isEnabled":2}`)},
		{method: http.MethodPatch, path: "/api/admin/v1/menus/7/status", body: []byte(`{"isEnabled":0,"other":true}`)},
		{method: http.MethodDelete, path: "/api/admin/v1/menus/7", body: []byte(`{}`)},
	}
	for index, request := range invalid {
		t.Run(fmt.Sprintf("invalid-%d", index), func(t *testing.T) {
			responseRecorder := serveMenuRequest(t, &menuHTTPService{}, request.method, request.path, request.body)
			assertMenuEnvelope(t, responseRecorder, http.StatusBadRequest, apperror.CodeInvalidRequest)
		})
	}
}

func TestMenuHandlerRebuildsAccessCache(t *testing.T) {
	service := &menuHTTPService{rebuildCount: 3}
	recorder := serveMenuRequest(t, service, http.MethodPost, "/api/admin/v1/menus/access-cache/rebuild", nil)
	assertMenuEnvelope(t, recorder, http.StatusOK, 0)
	if service.rebuildCalls != 1 {
		t.Fatalf("rebuild calls = %d", service.rebuildCalls)
	}
	var envelope struct {
		Data map[string]int `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data["rebuiltUsers"] != 3 {
		t.Fatalf("data = %+v", envelope.Data)
	}
}

func menuTestStringPointer(value string) *string {
	return &value
}

func TestMenuHandlerPreservesLocalizedServiceError(t *testing.T) {
	service := &menuHTTPService{listError: &apperror.Error{
		HTTPStatus: http.StatusConflict, Code: menu.CodeMenuParentDisabled,
		MessageKey: "menu.parentDisabled", Params: map[string]string{"code": "reports"},
		Cause: errors.New("internal detail"),
	}}
	recorder := serveMenuRequest(t, service, http.MethodGet, "/api/admin/v1/menus", nil)
	assertMenuEnvelope(t, recorder, http.StatusConflict, menu.CodeMenuParentDisabled)
	var envelope struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Message != "菜单 reports 的父级未全部启用" {
		t.Fatalf("message = %q", envelope.Message)
	}
}

func TestMenuRoutesBindExactPermissionsInMiddlewareOrder(t *testing.T) {
	service := &menuHTTPService{listResult: menu.Catalog{Platforms: []menu.PlatformOption{}, MenuTree: []menu.ManagedMenu{}}}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registeredPermissions := make([]string, 0)
	requestOrder := make([]string, 0)
	authenticate := func(context *gin.Context) {
		requestOrder = append(requestOrder, "authenticate")
		context.Next()
	}
	requirePermission := func(code string) gin.HandlerFunc {
		registeredPermissions = append(registeredPermissions, code)
		return func(context *gin.Context) {
			requestOrder = append(requestOrder, "permission:"+code)
			context.Next()
		}
	}
	menu.RegisterRoutes(router.Group("/api/admin/v1"), menu.NewHandler(service), authenticate, requirePermission)
	wantPermissions := []string{menu.PermissionList, menu.PermissionCreate, menu.PermissionUpdate, menu.PermissionDelete}
	if !reflect.DeepEqual(registeredPermissions, []string{menu.PermissionList, menu.PermissionCreate, menu.PermissionUpdate, menu.PermissionUpdate, menu.PermissionDelete, menu.PermissionRebuildAccessCache}) {
		t.Fatalf("registered permissions = %v", registeredPermissions)
	}
	_ = wantPermissions

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/v1/menus", nil))
	if recorder.Code != http.StatusOK || !reflect.DeepEqual(requestOrder, []string{"authenticate", "permission:" + menu.PermissionList}) {
		t.Fatalf("request order = %v status=%d", requestOrder, recorder.Code)
	}

	wantRoutes := map[string]bool{
		"GET /api/admin/v1/menus": false, "POST /api/admin/v1/menus": false,
		"PUT /api/admin/v1/menus/:id": false, "PATCH /api/admin/v1/menus/:id/status": false,
		"DELETE /api/admin/v1/menus/:id":                false,
		"POST /api/admin/v1/menus/access-cache/rebuild": false,
	}
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, exists := wantRoutes[key]; exists {
			wantRoutes[key] = true
		}
	}
	for route, found := range wantRoutes {
		if !found {
			t.Errorf("route %s is missing", route)
		}
	}
}

func TestMenuRoutesStopBeforeHandlerOnAuthenticationOrPermissionFailure(t *testing.T) {
	tests := []struct {
		name       string
		auth       gin.HandlerFunc
		permission gin.HandlerFunc
		wantStatus int
		wantCode   int
	}{
		{
			name: "unauthenticated",
			auth: func(context *gin.Context) {
				response.Fail(context, apperror.Unauthorized(errors.New("missing identity")))
			},
			permission: func(context *gin.Context) { context.Next() },
			wantStatus: http.StatusUnauthorized, wantCode: apperror.CodeUnauthorized,
		},
		{
			name: "unauthorized",
			auth: func(context *gin.Context) { context.Next() },
			permission: func(context *gin.Context) {
				response.Fail(context, apperror.Forbidden(errors.New("permission denied")))
			},
			wantStatus: http.StatusForbidden, wantCode: apperror.CodeForbidden,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &menuHTTPService{listResult: menu.Catalog{Platforms: []menu.PlatformOption{}, MenuTree: []menu.ManagedMenu{}}}
			gin.SetMode(gin.TestMode)
			router := gin.New()
			menu.RegisterRoutes(router.Group("/api/admin/v1"), menu.NewHandler(service), test.auth, func(string) gin.HandlerFunc { return test.permission })
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/v1/menus", nil))
			assertMenuEnvelope(t, recorder, test.wantStatus, test.wantCode)
			if service.listCalls != 0 {
				t.Fatal("blocked request reached Handler Service")
			}
		})
	}
}

type menuHTTPService struct {
	listResult  menu.Catalog
	listError   error
	listContext context.Context
	listCalls   int
	listQuery   menu.ListQuery

	createID    int64
	createInput menu.CreateInput
	createCalls int

	updateID    int64
	updateInput menu.UpdateInput

	statusID    int64
	statusValue yesno.Value

	deleteID     int64
	rebuildCount int
	rebuildCalls int
}

func (s *menuHTTPService) List(ctx context.Context, query menu.ListQuery) (menu.Catalog, error) {
	s.listCalls++
	s.listContext = ctx
	s.listQuery = query
	return s.listResult, s.listError
}

func (s *menuHTTPService) Create(_ context.Context, input menu.CreateInput) (int64, error) {
	s.createCalls++
	s.createInput = input
	return s.createID, nil
}

func (s *menuHTTPService) Update(_ context.Context, id int64, input menu.UpdateInput) error {
	s.updateID = id
	s.updateInput = input
	return nil
}

func (s *menuHTTPService) UpdateStatus(_ context.Context, id int64, value yesno.Value) error {
	s.statusID = id
	s.statusValue = value
	return nil
}

func (s *menuHTTPService) Delete(_ context.Context, id int64) error {
	s.deleteID = id
	return nil
}

func (s *menuHTTPService) RebuildAccessCache(_ context.Context) (int, error) {
	s.rebuildCalls++
	return s.rebuildCount, nil
}

func serveMenuRequest(t *testing.T, service *menuHTTPService, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	pass := func(context *gin.Context) { context.Next() }
	menu.RegisterRoutes(router.Group("/api/admin/v1"), menu.NewHandler(service), pass, func(string) gin.HandlerFunc { return pass })
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func assertMenuEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus, wantCode int) {
	t.Helper()
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope) != 3 || envelope["code"] == nil || envelope["data"] == nil || envelope["message"] == nil {
		t.Fatalf("envelope = %v", envelope)
	}
	var code int
	if err := json.Unmarshal(envelope["code"], &code); err != nil || code != wantCode {
		t.Fatalf("code = %d error=%v body=%s", code, err, recorder.Body)
	}
}

func assertMutationData(t *testing.T, recorder *httptest.ResponseRecorder, want map[string]float64) {
	t.Helper()
	var envelope struct {
		Data map[string]float64 `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(envelope.Data, want) {
		t.Fatalf("mutation data = %v, want %v", envelope.Data, want)
	}
}
