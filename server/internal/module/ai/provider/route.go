package provider

import "github.com/gin-gonic/gin"

const (
	PermissionView   = "ai:provider:view"
	PermissionList   = "ai:provider:list"
	PermissionDetail = "ai:provider:detail"
	PermissionCreate = "ai:provider:create"
	PermissionUpdate = "ai:provider:update"
	PermissionStatus = "ai:provider:status"
	PermissionDelete = "ai:provider:delete"
)

func RegisterAdminRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	group := routes.Group("/ai/provider")
	group.GET("/options", authenticate, handler.Options)
	group.GET("", authenticate, requirePermission(PermissionList), handler.List)
	group.GET("/:id", authenticate, requirePermission(PermissionDetail), handler.Get)
	group.POST("", authenticate, requirePermission(PermissionCreate), handler.Create)
	group.PUT("/:id", authenticate, requirePermission(PermissionUpdate), handler.Update)
	group.PATCH("/:id/status", authenticate, requirePermission(PermissionStatus), handler.UpdateStatus)
	group.DELETE("/:id", authenticate, requirePermission(PermissionDelete), handler.Delete)
}
