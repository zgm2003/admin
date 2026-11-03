package officialmodel

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestOfficialModelPriceIsReadOnlyAndUsesDetailAction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var code string
	RegisterAdminRoutes(r.Group(""), NewHandler(nil), func(c *gin.Context) { c.Next() }, func(action string) gin.HandlerFunc {
		return func(c *gin.Context) { code = action; c.AbortWithStatus(403) }
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/ai/officialmodel/1/price", nil))
	if code != "ai:officialModel:detail" || w.Code != 403 {
		t.Fatal("price view permission wrong")
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PUT", "/ai/officialmodel/1/price", nil))
	if w.Code != 404 {
		t.Fatal("standard model exposed price writer")
	}
}
