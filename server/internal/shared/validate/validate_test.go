package validate_test

import (
	"net/http"
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

func TestParsePositiveInt64UsesStrictDecimalSyntax(t *testing.T) {
	for _, value := range []string{"1", "001", "9223372036854775807"} {
		got, err := sharedvalidate.ParsePositiveInt64(value, "id")
		if err != nil || got < 1 {
			t.Fatalf("ParsePositiveInt64(%q) = %d,%v", value, got, err)
		}
	}

	for _, value := range []string{"", "+1", "-1", "1.0", "abc", "9223372036854775808"} {
		if _, err := sharedvalidate.ParsePositiveInt64(value, "id"); err == nil {
			t.Fatalf("ParsePositiveInt64(%q) accepted invalid input", value)
		}
	}
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

func TestRequireEmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	emptyRequest := httptest.NewRequest(http.MethodPost, "/", nil)
	emptyContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	emptyContext.Request = emptyRequest
	if err := sharedvalidate.RequireEmptyBody(emptyContext); err != nil {
		t.Fatalf("empty body rejected: %v", err)
	}

	for _, body := range []string{" ", "{}", `{"value":1}`} {
		t.Run(body, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = request
			if err := sharedvalidate.RequireEmptyBody(context); err == nil {
				t.Fatalf("body %q was accepted", body)
			}
		})
	}

	hiddenBodyRequest := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("x"))
	hiddenBodyRequest.ContentLength = 0
	hiddenBodyContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	hiddenBodyContext.Request = hiddenBodyRequest
	if err := sharedvalidate.RequireEmptyBody(hiddenBodyContext); err == nil {
		t.Fatal("non-empty body with zero Content-Length was accepted")
	}
}

func contextWithBody(body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	return context
}
