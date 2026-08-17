package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	projectmiddleware "admin/server/internal/middleware"
	"github.com/gin-gonic/gin"
)

func TestRequestIDPreservesValidIncomingValue(t *testing.T) {
	router := requestIDRouter()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(projectmiddleware.RequestIDHeader, "request-123")

	router.ServeHTTP(recorder, request)

	if got := recorder.Header().Get(projectmiddleware.RequestIDHeader); got != "request-123" {
		t.Fatalf("response request ID = %q", got)
	}
}

func TestRequestIDGeneratesValueForMissingOrInvalidInput(t *testing.T) {
	for _, incoming := range []string{"", "contains spaces", "<script>", string(make([]byte, 129))} {
		t.Run(incoming, func(t *testing.T) {
			router := requestIDRouter()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set(projectmiddleware.RequestIDHeader, incoming)

			router.ServeHTTP(recorder, request)

			got := recorder.Header().Get(projectmiddleware.RequestIDHeader)
			if !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(got) {
				t.Fatalf("generated request ID = %q", got)
			}
		})
	}
}

func requestIDRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(projectmiddleware.RequestID())
	router.GET("/", func(context *gin.Context) {
		context.String(http.StatusOK, projectmiddleware.GetRequestID(context))
	})
	return router
}
