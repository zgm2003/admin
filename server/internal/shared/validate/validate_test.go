package validate_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"admin/server/internal/shared/apperror"
	sharedvalidate "admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type input struct {
	Name string `json:"name" binding:"required,min=2"`
}

func TestBindJSONAcceptsOneStrictValidDocument(t *testing.T) {
	var got input
	err := sharedvalidate.BindJSON(contextWithBody(`{"name":"ok"}`), &got)
	if err != nil {
		t.Fatalf("BindJSON() error = %v", err)
	}
	if got.Name != "ok" {
		t.Fatalf("Name = %q", got.Name)
	}
}

func TestBindJSONRejectsInvalidDocuments(t *testing.T) {
	for _, body := range []string{
		``,
		`{"name":""}`,
		`{"name":"ok","unknown":true}`,
		`{"name":"ok"} {"name":"again"}`,
	} {
		t.Run(body, func(t *testing.T) {
			var got input
			err := sharedvalidate.BindJSON(contextWithBody(body), &got)
			if err == nil {
				t.Fatal("expected strict binding error")
			}

			appErr, ok := err.(*apperror.Error)
			if !ok {
				t.Fatalf("error type = %T", err)
			}
			if appErr.Code != apperror.CodeInvalidRequest {
				t.Fatalf("error code = %d", appErr.Code)
			}
		})
	}
}

func contextWithBody(body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	return context
}
