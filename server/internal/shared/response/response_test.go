package response_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

func TestOKUsesTheOnlySuccessEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	response.OK(context, http.StatusOK, gin.H{"value": "ok"})

	assertJSON(t, recorder, http.StatusOK, map[string]any{
		"code":    float64(0),
		"data":    map[string]any{"value": "ok"},
		"message": "ok",
	})
}

func TestFailUsesStableBusinessError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	response.Fail(context, apperror.InvalidRequest(errors.New("bad input")))

	assertJSON(t, recorder, http.StatusBadRequest, map[string]any{
		"code":    float64(apperror.CodeInvalidRequest),
		"data":    nil,
		"message": "请求参数错误",
	})
}

func TestFailDoesNotExposeUnknownErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	response.Fail(context, errors.New("database password leaked"))

	assertJSON(t, recorder, http.StatusInternalServerError, map[string]any{
		"code":    float64(apperror.CodeInternal),
		"data":    nil,
		"message": "服务内部错误",
	})
	var appErr *apperror.Error
	if len(context.Errors) != 1 || !errors.As(context.Errors.Last().Err, &appErr) {
		t.Fatalf("context errors = %#v, want one apperror.Error", context.Errors)
	}
	if appErr.Cause == nil || appErr.Cause.Error() != "database password leaked" {
		t.Fatalf("cause = %v, want original internal cause", appErr.Cause)
	}
	if strings.Contains(recorder.Body.String(), "database password leaked") {
		t.Fatal("response leaked internal cause")
	}
}

func assertJSON(t *testing.T, recorder *httptest.ResponseRecorder, status int, want map[string]any) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("status = %d, want %d", recorder.Code, status)
	}

	var got map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("response = %#v, want %#v", got, want)
	}
	if len(got) != 3 {
		t.Fatalf("response keys = %v, want exactly code, data, message", reflect.ValueOf(got).MapKeys())
	}
}
