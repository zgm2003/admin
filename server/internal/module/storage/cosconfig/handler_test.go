package cosconfig

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

type configHTTPService struct {
	listQuery   ListQuery
	createInput CreateInput
	updateInput UpdateInput
	statusValue yesno.Value
	createCalls int
	updateCalls int
	testCalls   int
	deleteCalls int
}

func (s *configHTTPService) List(_ context.Context, query ListQuery) (pagination.Result[SafeValue], error) {
	s.listQuery = query
	endpoint := "https://cos.example.com"
	return pagination.Result[SafeValue]{List: []SafeValue{{ID: 1, Name: "Main", AppID: "1250000000", Bucket: "assets", Region: "ap-guangzhou", Endpoint: &endpoint, IsEnabled: yesno.Yes, HasCredentials: true, CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(2, 0).UTC()}}, Total: 1, Page: query.Page, PageSize: query.PageSize}, nil
}
func (*configHTTPService) Get(context.Context, int64) (SafeValue, error) {
	return SafeValue{ID: 1, Name: "Main", HasCredentials: true}, nil
}
func (s *configHTTPService) Create(_ context.Context, input CreateInput) (int64, error) {
	s.createCalls++
	s.createInput = input
	return 9, nil
}
func (s *configHTTPService) Update(_ context.Context, _ int64, input UpdateInput) error {
	s.updateCalls++
	s.updateInput = input
	return nil
}
func (s *configHTTPService) UpdateStatus(_ context.Context, _ int64, value yesno.Value) error {
	s.statusValue = value
	return nil
}
func (s *configHTTPService) TestConnection(context.Context, int64) error { s.testCalls++; return nil }
func (s *configHTTPService) Delete(context.Context, int64) error         { s.deleteCalls++; return nil }

func TestRoutesUseExactCOSConfigPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	permissions := make([]string, 0)
	pass := func(context *gin.Context) { context.Next() }
	router := gin.New()
	RegisterRoutes(router.Group("/api/admin/v1"), NewHandler(&configHTTPService{}), pass, func(code string) gin.HandlerFunc {
		permissions = append(permissions, code)
		return pass
	})
	want := []string{PermissionList, PermissionCreate, PermissionList, PermissionUpdate, PermissionStatus, PermissionTest, PermissionDelete}
	if !reflect.DeepEqual(permissions, want) {
		t.Fatalf("permissions = %v, want %v", permissions, want)
	}
}

func TestHandlersUseStrictDTOsAndNeverReturnCredentials(t *testing.T) {
	service, router := configRouter(t)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/v1/storage/cos-configs?page=1&pageSize=20&keyword=main&isEnabled=1", nil))
	if recorder.Code != http.StatusOK || service.listQuery.Page != 1 || service.listQuery.PageSize != 20 || service.listQuery.IsEnabled == nil || *service.listQuery.IsEnabled != yesno.Yes {
		t.Fatalf("list status=%d query=%+v body=%s", recorder.Code, service.listQuery, recorder.Body)
	}
	for _, forbidden := range []string{"secretId", "secretKey", "ciphertext"} {
		if strings.Contains(strings.ToLower(recorder.Body.String()), strings.ToLower(forbidden)) {
			t.Fatalf("list response leaks %q: %s", forbidden, recorder.Body)
		}
	}

	validCreate := `{"name":" Main ","appId":"1250000000","secretId":"sid","secretKey":"skey","bucket":"assets","region":"ap-guangzhou","endpoint":"https://cos.example.com","bucketDomain":"https://cdn.example.com","isEnabled":1,"remark":"primary"}`
	recorder = performConfigJSON(router, http.MethodPost, "/api/admin/v1/storage/cos-configs", validCreate)
	if recorder.Code != http.StatusCreated || service.createCalls != 1 || recorder.Body.String() != `{"code":0,"data":{"id":9},"message":"ok"}` {
		t.Fatalf("create status=%d calls=%d body=%s", recorder.Code, service.createCalls, recorder.Body)
	}
	for _, body := range []string{
		`{"name":"Main"}`,
		strings.TrimSuffix(validCreate, "}") + `,"unknown":1}`,
		strings.Replace(validCreate, `"name":" Main "`, `"name":"Main","name":"Other"`, 1),
		strings.Replace(validCreate, `"isEnabled":1`, `"isEnabled":2`, 1),
		strings.Replace(validCreate, `"endpoint":"https://cos.example.com"`, `"endpoint":"ftp://cos.example.com"`, 1),
		strings.Replace(validCreate, `"secretKey":"skey"`, `"secretKey":""`, 1),
	} {
		recorder = performConfigJSON(router, http.MethodPost, "/api/admin/v1/storage/cos-configs", body)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("invalid create status=%d body=%s input=%s", recorder.Code, recorder.Body, body)
		}
	}

	validUpdate := `{"name":"Main","appId":"1250000000","bucket":"assets","region":"ap-guangzhou","endpoint":null,"bucketDomain":null,"remark":"updated"}`
	recorder = performConfigJSON(router, http.MethodPut, "/api/admin/v1/storage/cos-configs/1", validUpdate)
	if recorder.Code != http.StatusOK || service.updateCalls != 1 || service.updateInput.SecretID.Present || service.updateInput.SecretKey.Present {
		t.Fatalf("update status=%d calls=%d input=%+v body=%s", recorder.Code, service.updateCalls, service.updateInput, recorder.Body)
	}
	for _, replacement := range []string{`"secretId":null,`, `"secretId":"",`, `"secretKey":null,`, `"secretKey":"",`} {
		recorder = performConfigJSON(router, http.MethodPut, "/api/admin/v1/storage/cos-configs/1", strings.Replace(validUpdate, "{", "{"+replacement, 1))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("invalid secret replacement status=%d body=%s", recorder.Code, recorder.Body)
		}
	}

	for _, test := range []struct{ method, path, body string }{{http.MethodPost, "/api/admin/v1/storage/cos-configs/0/test", ""}, {http.MethodDelete, "/api/admin/v1/storage/cos-configs/-1", ""}, {http.MethodPost, "/api/admin/v1/storage/cos-configs/1/test", `{}`}, {http.MethodDelete, "/api/admin/v1/storage/cos-configs/1", `{}`}} {
		recorder = performConfigJSON(router, test.method, test.path, test.body)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s %s status=%d body=%s", test.method, test.path, recorder.Code, recorder.Body)
		}
	}
}

func configRouter(t *testing.T) (*configHTTPService, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	service := &configHTTPService{}
	router := gin.New()
	pass := func(context *gin.Context) { context.Next() }
	RegisterRoutes(router.Group("/api/admin/v1"), NewHandler(service), pass, func(string) gin.HandlerFunc { return pass })
	return service, router
}

func performConfigJSON(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}
