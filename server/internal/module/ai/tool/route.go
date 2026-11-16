package tool

import "github.com/gin-gonic/gin"

const (
	PermissionList   = "ai:tool:list"
	PermissionDetail = "ai:tool:detail"
	PermissionCreate = "ai:tool:create"
	PermissionUpdate = "ai:tool:update"
	PermissionStatus = "ai:tool:status"
	PermissionDelete = "ai:tool:delete"
)

func RegisterAdminRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	group := routes.Group("/ai/tools")
	group.GET("", authenticate, requirePermission(PermissionList), handler.List)
	group.POST("", authenticate, requirePermission(PermissionCreate), handler.Create)
	group.GET("/:code", authenticate, requirePermission(PermissionDetail), handler.Get)
	group.PUT("/:code", authenticate, requirePermission(PermissionUpdate), handler.Update)
	group.PATCH("/:code/status", authenticate, requirePermission(PermissionStatus), handler.UpdateStatus)
	group.DELETE("/:code", authenticate, requirePermission(PermissionDelete), handler.Delete)
}
