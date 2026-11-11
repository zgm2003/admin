package providermodel

import (
	"testing"

	aiprovider "admin/server/internal/module/ai/provider"
	"github.com/gin-gonic/gin"
)

func TestRegisterAdminRoutesSharesProviderIDParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api/admin/v1")
	authenticate := func(c *gin.Context) { c.Next() }
	requirePermission := func(string) gin.HandlerFunc {
		return func(c *gin.Context) { c.Next() }
	}
	aiprovider.RegisterAdminRoutes(group, aiprovider.NewHandler(nil), authenticate, requirePermission)
	RegisterAdminRoutes(group, NewHandler(nil), authenticate, requirePermission)

	routes := make(map[string]bool)
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, route := range []string{
		"GET /api/admin/v1/ai/provider/:id",
		"POST /api/admin/v1/ai/provider/models/preview",
		"POST /api/admin/v1/ai/provider/test",
		"POST /api/admin/v1/ai/provider/:id/models/sync",
	} {
		if !routes[route] {
			t.Fatalf("route %s is not registered", route)
		}
	}
}
