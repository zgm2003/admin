package config

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
	loadSafe    Safe
	loadErr     error
	updateCalls int
	updateInput Input
	updateSafe  Safe
	updateErr   error
	deleteCalls int
	deleteErr   error
}

func (s *stubService) Load(context.Context) (Safe, error) { return s.loadSafe, s.loadErr }

func (s *stubService) Update(_ context.Context, input Input) (Safe, error) {
	s.updateCalls++
	s.updateInput = input
	return s.updateSafe, s.updateErr
}

func (s *stubService) Delete(context.Context) error {
	s.deleteCalls++
	return s.deleteErr
}

func TestRegisterRoutesBindsExactConfigPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	seen := make(map[string]int, 3)
	RegisterRoutes(router.Group("/api/admin/v1/message/sms"), NewHandler(&stubService{}),
		func(c *gin.Context) { c.Next() },
		func(code string) gin.HandlerFunc {
			return func(c *gin.Context) {
				seen[code]++
				c.Next()
			}
		})

	for _, request := range []struct{ method, path string }{
		{http.MethodGet, "/api/admin/v1/message/sms/config"},
		{http.MethodPut, "/api/admin/v1/message/sms/config"},
		{http.MethodDelete, "/api/admin/v1/message/sms/config"},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(request.method, request.path, nil))
	}

	if seen[PermissionList] != 1 || seen[PermissionUpdate] != 1 || seen[PermissionDelete] != 1 {
		t.Fatalf("registered permissions = %v", seen)
	}
	if seen["message:sms:view"] != 0 {
		t.Fatal("config routes must not reuse the page permission")
	}
}

func TestUpdateRejectsUnknownAndMissingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubService{}
	router := gin.New()
	router.PUT("/config", NewHandler(service).Update)

	for _, test := range []struct{ name, body string }{
		{
			name: "unknown field",
			body: `{"secretId":"a","secretKey":"b","smsSdkAppId":"c","signName":"d","region":"e","endpoint":"","ttlMinutes":5,"isEnabled":1,"extra":true}`,
		},
		{
			name: "missing field",
			body: `{"secretId":"a","secretKey":"b","smsSdkAppId":"c","signName":"d","region":"e","endpoint":"","ttlMinutes":5}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPut, "/config", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body)
			}
			if service.updateCalls != 0 {
				t.Fatal("invalid request reached the service")
			}
		})
	}
}

func TestUpdateForwardsEveryFieldToTheService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubService{updateSafe: Safe{Configured: true, TTLMinutes: 5, IsEnabled: yesno.Yes}}
	router := gin.New()
	router.PUT("/config", NewHandler(service).Update)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/config", strings.NewReader(
		`{"secretId":"id","secretKey":"key","smsSdkAppId":"1400006666","signName":"签名","region":"ap-guangzhou","endpoint":"","ttlMinutes":5,"isEnabled":1}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body)
	}
	want := Input{SecretID: "id", SecretKey: "key", SDKAppID: "1400006666", SignName: "签名", Region: "ap-guangzhou", Endpoint: "", TTLMinutes: 5, IsEnabled: yesno.Yes}
	if service.updateInput != want {
		t.Fatalf("input = %+v, want %+v", service.updateInput, want)
	}
	if strings.Contains(recorder.Body.String(), "key") {
		t.Fatalf("response leaked a secret: %s", recorder.Body)
	}
}

func TestDeleteRequiresAnEmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubService{}
	router := gin.New()
	router.DELETE("/config", NewHandler(service).Delete)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/config", strings.NewReader(`{"force":true}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || service.deleteCalls != 0 {
		t.Fatalf("status = %d calls = %d", recorder.Code, service.deleteCalls)
	}
}
