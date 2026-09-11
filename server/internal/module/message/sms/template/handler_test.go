package template

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
	listSafe  []Safe
	listErr   error
	updateID  int64
	updateIn  UpdateInput
	updateErr error
	statusID  int64
	statusVal yesno.Value
	statusErr error
}

func (s *stubService) List(context.Context) ([]Safe, error) { return s.listSafe, s.listErr }

func (s *stubService) Update(_ context.Context, id int64, input UpdateInput) (Safe, error) {
	s.updateID = id
	s.updateIn = input
	return Safe{}, s.updateErr
}

func (s *stubService) UpdateStatus(_ context.Context, id int64, status yesno.Value) error {
	s.statusID = id
	s.statusVal = status
	return s.statusErr
}

func TestRegisterRoutesBindsExactTemplatePermissions(t *testing.T) {
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
		{http.MethodGet, "/api/admin/v1/message/sms/template"},
		{http.MethodPut, "/api/admin/v1/message/sms/template/2"},
		{http.MethodPatch, "/api/admin/v1/message/sms/template/2/status"},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(request.method, request.path, nil))
	}

	if seen[PermissionList] != 1 || seen[PermissionUpdate] != 1 || seen[PermissionStatus] != 1 {
		t.Fatalf("registered permissions = %v", seen)
	}
}

func TestUpdateRejectsUnknownAndIncompleteBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubService{}
	router := gin.New()
	router.PUT("/template/:id", NewHandler(service).Update)

	for _, test := range []struct{ name, body string }{
		{
			name: "unknown field",
			body: `{"scene":"login","name":"n","tencentTemplateId":"1","parameterKeys":["code","ttl_minutes"],"exampleVariables":{"code":"1","ttl_minutes":"5"},"subject":"x"}`,
		},
		{
			name: "missing scene",
			body: `{"name":"n","tencentTemplateId":"1","parameterKeys":["code","ttl_minutes"],"exampleVariables":{"code":"1","ttl_minutes":"5"}}`,
		},
		{
			name: "invalid identifier",
			body: `{"scene":"login","name":"n","tencentTemplateId":"1","parameterKeys":["code","ttl_minutes"],"exampleVariables":{"code":"1","ttl_minutes":"5"}}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			path := "/template/2"
			if test.name == "invalid identifier" {
				path = "/template/not-a-number"
			}
			request := httptest.NewRequest(http.MethodPut, path, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body)
			}
			if service.updateID != 0 {
				t.Fatal("invalid request reached the service")
			}
		})
	}
}

func TestUpdateForwardsTheExactSceneAndParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubService{}
	router := gin.New()
	router.PUT("/template/:id", NewHandler(service).Update)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/template/3", strings.NewReader(
		`{"scene":"bind_phone","name":"绑定/换绑手机","tencentTemplateId":"7654321","parameterKeys":["code","ttl_minutes"],"exampleVariables":{"code":"123456","ttl_minutes":"5"}}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body)
	}
	if service.updateID != 3 || service.updateIn.Scene != SceneBindPhone ||
		strings.Join(service.updateIn.ParameterKeys, ",") != "code,ttl_minutes" ||
		service.updateIn.ExampleVariables["ttl_minutes"] != "5" {
		t.Fatalf("forwarded id=%d input=%+v", service.updateID, service.updateIn)
	}
}

func TestStatusForwardsAValidatedFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubService{}
	router := gin.New()
	router.PATCH("/template/:id/status", NewHandler(service).Status)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/template/4/status", strings.NewReader(`{"isEnabled":1}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.statusID != 4 || service.statusVal != yesno.Yes {
		t.Fatalf("status = %d forwarded = %d/%d", recorder.Code, service.statusID, service.statusVal)
	}

	invalid := httptest.NewRecorder()
	badRequest := httptest.NewRequest(http.MethodPatch, "/template/4/status", strings.NewReader(`{"isEnabled":7}`))
	badRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(invalid, badRequest)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid status code = %d", invalid.Code)
	}
}
