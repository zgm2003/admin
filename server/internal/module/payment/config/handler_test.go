package config

import (
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestConfigActionRoutesAreIndependent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(NewService(nil, nil))
	router := gin.New()
	RegisterRoutes(router.Group("/api/admin/v1"), handler, func(c *gin.Context) { c.Next() }, func(code string) gin.HandlerFunc {
		return func(c *gin.Context) {
			if code != "payment:config:view" {
				response.Fail(c, apperror.Forbidden(nil))
				c.Abort()
				return
			}
			c.Next()
		}
	})
	for _, r := range []struct{ method, path string }{{"GET", ""}, {"GET", "/1"}, {"POST", ""}, {"PUT", "/1"}, {"POST", "/1/version"}, {"PATCH", "/1/status"}, {"DELETE", "/1"}, {"POST", "/1/test"}} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(r.method, "/api/admin/v1/payment/config"+r.path, bytes.NewBufferString("{}")))
		if w.Code != 403 {
			t.Fatalf("view allowed %s %s: %d", r.method, r.path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/admin/v1/payment/config/options", nil))
	if w.Code != 200 {
		t.Fatal("static options require data access")
	}
	var out struct{ Data Options }
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.Data.Providers) != 1 || out.Data.Providers[0].Value != 1 {
		t.Fatal("options advertised unsupported capability")
	}
}
func TestConfigHandlerRejectsMalformedWithoutRepository(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router.Group("/api/admin/v1"), NewHandler(NewService(nil, nil)), func(c *gin.Context) { c.Next() }, func(string) gin.HandlerFunc { return func(c *gin.Context) { c.Next() } })
	for _, r := range []struct{ method, path, body string }{{"POST", "", `{"provider":"1"}`}, {"POST", "", `{"provider":1,"provider":2}`}, {"POST", "/1/version", `{"notifyUrl":"x","payMethods":[1]}`}, {"PUT", "/1", `{"name":"x","sortOrder":0,"provider":1}`}, {"PUT", "/0", `{}`}, {"PATCH", "/1/status", `{"isEnabled":"1"}`}, {"DELETE", "/1", `{}`}, {"GET", "?page=1&pageSize=20&isEnabled=01", ``}} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(r.method, "/api/admin/v1/payment/config"+r.path, bytes.NewBufferString(r.body)))
		if w.Code != 400 {
			t.Fatalf("malformed %s %s: %d %s", r.method, r.path, w.Code, w.Body.String())
		}
		if !bytes.Contains(w.Body.Bytes(), []byte(`"code":10001`)) {
			t.Fatal("invalid envelope")
		}
	}
}
