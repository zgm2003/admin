package provider

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderRoutesUseExactActionsAndOptionsNeedOnlyAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	var observed string
	require := func(code string) gin.HandlerFunc {
		return func(c *gin.Context) { observed = code; c.AbortWithStatus(403) }
	}
	RegisterAdminRoutes(router.Group(""), NewHandler(nil, nil, nil, nil, nil), func(c *gin.Context) { c.Next() }, require)
	wCandidate := httptest.NewRecorder()
	router.ServeHTTP(wCandidate, httptest.NewRequest("GET", "/ai/provider/1/candidate/sync", nil))
	if wCandidate.Code != 403 || observed != PermissionSync {
		t.Fatalf("sync candidate must use sync action: status=%d permission=%s", wCandidate.Code, observed)
	}
	for _, tc := range []struct{ method, path, action string }{{"GET", "/ai/provider", "ai:provider:list"}, {"PUT", "/ai/provider/1", "ai:provider:update"}, {"POST", "/ai/provider/1/test", "ai:provider:test"}, {"POST", "/ai/provider/1/model/sync-preview", "ai:provider:sync"}, {"POST", "/ai/provider/1/model/2/price/sync", "ai:provider:syncPrice"}, {"PUT", "/ai/provider/1/model/2/price", "ai:provider:updatePrice"}, {"POST", "/ai/provider/1/model/2/price/3/publish", "ai:provider:updatePrice"}} {
		observed = ""
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != 403 || observed != tc.action {
			t.Fatalf("%s %s action=%s status=%d", tc.method, tc.path, observed, w.Code)
		}
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ai/provider/options", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"value":0`) {
		t.Fatalf("static typed options: %d %s", w.Code, w.Body.String())
	}
}
func TestProviderOrdinaryUpdateRejectsStatusAndPriceFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(nil, nil, nil, nil, nil)
	r.PUT("/ai/provider/:id", h.Update)
	for _, body := range []string{`{"name":"x","status":1}`, `{"name":"x","sourceUnitPrice":"1"}`, `{"name":"x","billingUnitPrice":"1"}`} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("PUT", "/ai/provider/1", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatalf("ordinary update accepted protected fields %d %s", w.Code, w.Body.String())
		}
	}
}
