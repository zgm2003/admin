package tool

import "github.com/gin-gonic/gin"

const (
	PermissionList   = "ai:tool:list"
	PermissionStatus = "ai:tool:status"
)

func RegisterAdminRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	group := routes.Group("/ai/tools")
	group.GET("", authenticate, requirePermission(PermissionList), handler.List)
	group.PATCH("/:code/status", authenticate, requirePermission(PermissionStatus), handler.UpdateStatus)
}
