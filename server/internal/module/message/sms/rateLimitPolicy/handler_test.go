package rateLimitPolicy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type stubService struct {
	platforms   []PlatformResponse
	updateCalls int
	updateID    int64
	updateKey   string
	updateLimit int
	updateWin   int
	updateErr   error
}

func (s *stubService) List(context.Context) ([]PlatformResponse, error) { return s.platforms, nil }

func (s *stubService) Update(_ context.Context, platformID int64, key string, limit, windowSeconds int) (PlatformResponse, error) {
	s.updateCalls++
	s.updateID, s.updateKey, s.updateLimit, s.updateWin = platformID, key, limit, windowSeconds
	return PlatformResponse{PlatformID: platformID, PlatformCode: "admin"}, s.updateErr
}

func TestRegisterRoutesBindsExactRateLimitPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	seen := make(map[string]int, 2)
	RegisterRoutes(router.Group("/api/admin/v1/message/sms"), NewHandler(&stubService{}),
		func(c *gin.Context) { c.Next() },
		func(code string) gin.HandlerFunc {
			return func(c *gin.Context) {
				seen[code]++
				c.Next()
			}
		})

	for _, request := range []struct{ method, path string }{
		{http.MethodGet, "/api/admin/v1/message/sms/rate-limit-policy"},
		{http.MethodPut, "/api/admin/v1/message/sms/rate-limit-policy/1/business_phone_minute"},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(request.method, request.path, nil))
	}
	if seen[PermissionList] != 1 || seen[PermissionUpdate] != 1 {
		t.Fatalf("registered permissions = %v", seen)
	}
}

func TestUpdateForwardPlatformKeyAndValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubService{}
	router := gin.New()
	router.PUT("/rate-limit-policy/:platformId/:key", NewHandler(service).Update)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/rate-limit-policy/2/business_phone_10m",
		strings.NewReader(`{"limit":7,"windowSeconds":900}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body)
	}
	if service.updateID != 2 || service.updateKey != KeyTenMin || service.updateLimit != 7 || service.updateWin != 900 {
		t.Fatalf("forwarded %d/%s/%d/%d", service.updateID, service.updateKey, service.updateLimit, service.updateWin)
	}
}

func TestUpdateRejectsInvalidIdentifiersAndBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubService{}
	router := gin.New()
	router.PUT("/rate-limit-policy/:platformId/:key", NewHandler(service).Update)

	for _, test := range []struct{ name, path, body string }{
		{name: "invalid platform", path: "/rate-limit-policy/0/business_phone_minute", body: `{"limit":1,"windowSeconds":60}`},
		{name: "unknown field", path: "/rate-limit-policy/1/business_phone_minute", body: `{"limit":1,"windowSeconds":60,"mode":"business"}`},
		{name: "missing window", path: "/rate-limit-policy/1/business_phone_minute", body: `{"limit":1}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPut, test.path, strings.NewReader(test.body))
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
