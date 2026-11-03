package agent

import "github.com/gin-gonic/gin"

const (
	PermissionList          = "ai:agent:list"
	PermissionDetail        = "ai:agent:detail"
	PermissionCreate        = "ai:agent:create"
	PermissionUpdate        = "ai:agent:update"
	PermissionStatus        = "ai:agent:status"
	PermissionDelete        = "ai:agent:delete"
	PermissionSwitchBinding = "ai:agent:switchBinding"
)

func RegisterAdminRoutes(routes *gin.RouterGroup, h *Handler, auth gin.HandlerFunc, require func(string) gin.HandlerFunc) {
	routes.GET("/ai/agent/options", auth, h.Options)
	routes.GET("/ai/agent/candidate/create", auth, require(PermissionCreate), h.Candidates)
	routes.GET("/ai/agent/:id/candidate/update", auth, require(PermissionUpdate), h.Candidates)
	routes.GET("/ai/agent", auth, require(PermissionList), h.List)
	routes.GET("/ai/agent/:id", auth, require(PermissionDetail), h.Detail)
	routes.POST("/ai/agent", auth, require(PermissionCreate), h.Create)
	routes.PUT("/ai/agent/:id", auth, require(PermissionUpdate), h.Update)
	routes.PATCH("/ai/agent/:id/status", auth, require(PermissionStatus), h.Status)
	routes.PATCH("/ai/agent/:id/binding/current", auth, require(PermissionSwitchBinding), h.SwitchBinding)
	routes.DELETE("/ai/agent/:id", auth, require(PermissionDelete), h.Delete)
}
