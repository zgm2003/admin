package recipientrule

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"admin/server/internal/shared/apperror"
	"github.com/gin-gonic/gin"
)

func TestCSVImportRoutesRequireIndependentActionBeforeBinding(t *testing.T) {
	for _, endpoint := range []struct{ method, path string }{
		{http.MethodGet, "/recipient-rule/import-template"},
		{http.MethodPost, "/recipient-rule/import/preview"},
		{http.MethodPost, "/recipient-rule/import"},
	} {
		t.Run(endpoint.path, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			authenticated := false
			permission := ""
			RegisterRoutes(router.Group(""), NewHandler(nil), func(ctx *gin.Context) {
				authenticated = true
				ctx.Next()
			}, func(code string) gin.HandlerFunc {
				return func(ctx *gin.Context) { permission = code; ctx.AbortWithStatus(http.StatusForbidden) }
			})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(endpoint.method, endpoint.path, strings.NewReader("bad json")))
			if response.Code != http.StatusForbidden || !authenticated || permission != PermissionImport {
				t.Fatalf("status=%d authenticated=%v permission=%q", response.Code, authenticated, permission)
			}
		})
	}
}

func TestCSVImportHandlerRejectsMalformedRequestsWithExactEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	pass := func(ctx *gin.Context) { ctx.Next() }
	RegisterRoutes(router.Group(""), NewHandler(nil), pass, func(string) gin.HandlerFunc { return pass })
	for _, path := range []string{"/recipient-rule/import/preview", "/recipient-rule/import"} {
		for _, body := range []string{"", "{}", `{"content":null}`, `{"content":12}`, `{"content":"x","extra":1}`, `{"content":"x","content":"y"}`, `{"content":"x"} {}`} {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
			var envelope map[string]json.RawMessage
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			var code int
			_ = json.Unmarshal(envelope["code"], &code)
			if response.Code != http.StatusBadRequest || len(envelope) != 3 || code != apperror.CodeInvalidRequest || string(envelope["data"]) != "null" || envelope["message"] == nil {
				t.Fatalf("path=%s body=%q status=%d response=%s", path, body, response.Code, response.Body.String())
			}
		}
	}
}

func TestCSVBindingRejectsInvalidUnicodeInsteadOfReplacingCSVBytes(t *testing.T) {
	for _, body := range []string{`{"content":"` + string([]byte{0xff}) + `"}`, `{"content":"\ud800"}`, `{"content":"\udc00"}`, `{"content":"\ud800\u0041"}`} {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest(http.MethodPost, "/recipient-rule/import/preview", strings.NewReader(body))
		if value, err := bindCSVContent(ctx); err == nil {
			t.Fatalf("invalid Unicode silently changed to %q for body=%q", value, body)
		}
	}
	for _, tc := range []struct{ body, want string }{
		{`{"content":"\ud83d\ude00"}`, "😀"},
		{`{"content":"\\ud800"}`, `\ud800`},
	} {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest(http.MethodPost, "/recipient-rule/import/preview", strings.NewReader(tc.body))
		if value, err := bindCSVContent(ctx); err != nil || value != tc.want {
			t.Fatalf("value=%q want=%q err=%v", value, tc.want, err)
		}
	}
}
