package rechargepackage

import (
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestActionRoutesRemainIndependentAndOptionsOnlyAuthenticate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	seen := ""
	RegisterAdminRoutes(router.Group("/api/admin/v1"), NewHandler(&Service{}), func(c *gin.Context) { c.Next() }, func(action string) gin.HandlerFunc {
		return func(c *gin.Context) {
			seen = action
			response.Fail(c, apperror.Forbidden(nil))
		}
	})
	for _, route := range []struct{ method, path, action string }{
		{"GET", "", PermissionList}, {"GET", "/1", PermissionDetail}, {"POST", "", PermissionCreate},
		{"PUT", "/1", PermissionUpdate}, {"PATCH", "/1/status", PermissionStatus}, {"DELETE", "/1", PermissionDelete},
	} {
		request := httptest.NewRequest(route.method, "/api/admin/v1/payment/rechargepackage"+route.path, bytes.NewReader([]byte(`{}`)))
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != 403 || seen != route.action {
			t.Fatalf("%s %s: status=%d action=%q want %q", route.method, route.path, recorder.Code, seen, route.action)
		}
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/admin/v1/payment/rechargepackage/options", nil))
	if recorder.Code != 200 {
		t.Fatalf("options required data permission: %d %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data Options `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || len(body.Data.Statuses) != 2 {
		t.Fatalf("options body invalid: %s %v", recorder.Body.String(), err)
	}
}

func TestHandlerRejectsUnknownAndMalformedPackageFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterAdminRoutes(router.Group("/api/admin/v1"), NewHandler(&Service{}), func(c *gin.Context) { c.Next() }, func(string) gin.HandlerFunc { return func(c *gin.Context) { c.Next() } })
	for _, route := range []struct{ method, path, body string }{
		{"POST", "", `{"code":"starter","name":"Starter","amount":"1.00","sortOrder":0,"status":0}`},
		{"POST", "", `{"code":"starter","name":"Starter","amount":"1.00","sortOrder":0,"status":0,"extra":true}`},
		{"PUT", "/1", `{"code":"starter","amount":"1.00","badge":null,"sortOrder":0}`},
		{"PATCH", "/1/status", `{"status":"1"}`},
		{"DELETE", "/1", `{}`},
		{"GET", "?page=1&pageSize=20&status=01", ""},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(route.method, "/api/admin/v1/payment/rechargepackage"+route.path, bytes.NewBufferString(route.body))
		router.ServeHTTP(recorder, request)
		if recorder.Code != 400 {
			t.Fatalf("accepted %s %s: %d %s", route.method, route.path, recorder.Code, recorder.Body.String())
		}
	}
}
