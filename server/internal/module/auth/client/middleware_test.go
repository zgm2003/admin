package authclient_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"admin/server/internal/module/auth/client"
	"github.com/gin-gonic/gin"
)

func TestRequireStoresExactCanonicalClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(authclient.Require())
	router.GET("/", func(context *gin.Context) {
		client, ok := authclient.FromContext(context)
		if !ok {
			t.Fatal("client metadata is missing")
		}
		if client.Platform != "admin" || client.DeviceID != "550e8400-e29b-41d4-a716-446655440000" || client.ClientIP != "192.0.2.8" || client.UserAgent != "test-agent" {
			t.Fatalf("client = %+v", client)
		}
		context.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.8:43210"
	request.Header[authclient.PlatformHeader] = []string{"admin"}
	request.Header[authclient.DeviceIDHeader] = []string{"550e8400-e29b-41d4-a716-446655440000"}
	request.Header.Set("User-Agent", "test-agent")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body)
	}
}

func TestRequireRejectsInvalidPlatformAndDeviceHeaders(t *testing.T) {
	tests := []struct {
		name      string
		platforms []string
		devices   []string
	}{
		{name: "missing platform", devices: []string{"550e8400-e29b-41d4-a716-446655440000"}},
		{name: "repeated platform", platforms: []string{"admin", "app"}, devices: []string{"550e8400-e29b-41d4-a716-446655440000"}},
		{name: "padded platform", platforms: []string{" admin"}, devices: []string{"550e8400-e29b-41d4-a716-446655440000"}},
		{name: "malformed platform", platforms: []string{"Admin"}, devices: []string{"550e8400-e29b-41d4-a716-446655440000"}},
		{name: "missing device", platforms: []string{"admin"}},
		{name: "repeated device", platforms: []string{"admin"}, devices: []string{"550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440001"}},
		{name: "padded device", platforms: []string{"admin"}, devices: []string{" 550e8400-e29b-41d4-a716-446655440000"}},
		{name: "uppercase device", platforms: []string{"admin"}, devices: []string{"550E8400-E29B-41D4-A716-446655440000"}},
		{name: "malformed device", platforms: []string{"admin"}, devices: []string{"device"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			calls := 0
			router := gin.New()
			router.Use(authclient.Require())
			router.GET("/", func(context *gin.Context) { calls++; context.Status(http.StatusNoContent) })
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header[authclient.PlatformHeader] = tt.platforms
			request.Header[authclient.DeviceIDHeader] = tt.devices
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest || calls != 0 || recorder.Body.String() != `{"code":10001,"data":null,"message":"请求参数错误"}` {
				t.Fatalf("status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body)
			}
		})
	}
}

func TestRequireAdminPlatformAllowsOnlyAdmin(t *testing.T) {
	tests := []struct {
		platform   string
		wantStatus int
		wantCalls  int
	}{
		{platform: "admin", wantStatus: http.StatusNoContent, wantCalls: 1},
		{platform: "portal", wantStatus: http.StatusForbidden, wantCalls: 0},
	}
	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			calls := 0
			router := gin.New()
			router.Use(authclient.Require(), authclient.RequireAdminPlatform())
			router.GET("/", func(context *gin.Context) {
				calls++
				context.Status(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set(authclient.PlatformHeader, tt.platform)
			request.Header.Set(authclient.DeviceIDHeader, "550e8400-e29b-41d4-a716-446655440000")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != tt.wantStatus || calls != tt.wantCalls {
				t.Fatalf("status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body.String())
			}
			if tt.wantStatus == http.StatusForbidden && recorder.Body.String() != `{"code":10003,"data":null,"message":"无权执行该操作"}` {
				t.Fatalf("body=%s", recorder.Body.String())
			}
		})
	}
}
