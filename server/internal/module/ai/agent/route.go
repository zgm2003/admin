package agent

import "github.com/gin-gonic/gin"

const (
	PermissionList   = "ai:agent:list"
	PermissionDetail = "ai:agent:detail"
	PermissionCreate = "ai:agent:create"
	PermissionUpdate = "ai:agent:update"
	PermissionStatus = "ai:agent:status"
	PermissionDelete = "ai:agent:delete"
)

func RegisterAdminRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	group := routes.Group("/ai/agent")
	group.GET("/options", authenticate, requirePermission(PermissionList), handler.Options)
	group.GET("/model-options", authenticate, requirePermission(PermissionList), handler.ModelOptions)
	group.GET("", authenticate, requirePermission(PermissionList), handler.List)
	group.POST("", authenticate, requirePermission(PermissionCreate), handler.Create)
	group.GET("/:id", authenticate, requirePermission(PermissionDetail), handler.Get)
	group.PUT("/:id", authenticate, requirePermission(PermissionUpdate), handler.Update)
	group.PATCH("/:id/status", authenticate, requirePermission(PermissionStatus), handler.UpdateStatus)
	group.DELETE("/:id", authenticate, requirePermission(PermissionDelete), handler.Delete)
}
