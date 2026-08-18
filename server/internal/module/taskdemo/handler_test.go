package taskdemo_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"admin/server/internal/module/taskdemo"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type submissionService struct {
	message string
	ctx     context.Context
	created taskdemo.Created
	err     error
}

func (s *submissionService) Create(ctx context.Context, message string) (taskdemo.Created, error) {
	s.ctx = ctx
	s.message = message
	return s.created, s.err
}

func TestCreateTaskReturnsAcceptedAndPassesRequestContext(t *testing.T) {
	service := &submissionService{created: taskdemo.Created{TaskID: "task-1"}}
	router := taskRouter(service)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/example-tasks", strings.NewReader(`{"message":"foundation-check"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer token")

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body)
	}
	if service.ctx != request.Context() || service.message != "foundation-check" {
		t.Fatalf("service context=%v message=%q", service.ctx, service.message)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["code"] != float64(0) || body["data"].(map[string]any)["taskId"] != "task-1" {
		t.Fatalf("body = %#v", body)
	}
}

func TestCreateTaskRejectsInvalidInputWithoutCallingService(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"message":""}`,
		`{"message":"ok","unknown":true}`,
		`{"message":"` + strings.Repeat("a", 201) + `"}`,
	} {
		t.Run(body, func(t *testing.T) {
			service := &submissionService{err: errors.New("must not be called")}
			router := taskRouter(service)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/example-tasks", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer token")

			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body)
			}
			if service.ctx != nil {
				t.Fatal("service was called")
			}
		})
	}
}

func TestCreateTaskRequiresAuthentication(t *testing.T) {
	service := &submissionService{}
	router := taskRouter(service)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/example-tasks", strings.NewReader(`{"message":"foundation-check"}`))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized || service.ctx != nil {
		t.Fatalf("status=%d service context=%v body=%s", recorder.Code, service.ctx, recorder.Body)
	}
}

func taskRouter(service *submissionService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	authenticate := func(context *gin.Context) {
		if context.GetHeader("Authorization") != "Bearer token" {
			response.Fail(context, apperror.Unauthorized(errors.New("test authentication required")))
			return
		}
		context.Next()
	}
	taskdemo.RegisterRoutes(router.Group("/api/v1"), taskdemo.NewHandler(service), authenticate)
	return router
}
