package agent

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterAdminRoutesExposesAgentSurface(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api/admin/v1")
	authenticate := func(c *gin.Context) { c.Next() }
	requirePermission := func(string) gin.HandlerFunc {
		return func(c *gin.Context) { c.Next() }
	}
	RegisterAdminRoutes(group, NewHandler(nil), authenticate, requirePermission)

	routes := make(map[string]bool)
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, route := range []string{
		"GET /api/admin/v1/ai/agent",
		"GET /api/admin/v1/ai/agent/options",
		"GET /api/admin/v1/ai/agent/model-options",
		"GET /api/admin/v1/ai/agent/:id",
		"POST /api/admin/v1/ai/agent",
		"PUT /api/admin/v1/ai/agent/:id",
		"PATCH /api/admin/v1/ai/agent/:id/status",
		"DELETE /api/admin/v1/ai/agent/:id",
	} {
		if !routes[route] {
			t.Fatalf("route %s is not registered", route)
		}
	}
}
