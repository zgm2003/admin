package role

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/roles", authenticate, requirePermission(PermissionList), handler.List)
	routes.POST("/roles", authenticate, requirePermission(PermissionCreate), handler.Create)
	routes.PUT("/roles/:id", authenticate, requirePermission(PermissionUpdate), handler.Update)
	routes.PATCH("/roles/:id/status", authenticate, requirePermission(PermissionStatus), handler.UpdateStatus)
	routes.PATCH("/roles/:id/default", authenticate, requirePermission(PermissionDefault), handler.SetDefault)
	routes.DELETE("/roles/:id", authenticate, requirePermission(PermissionDelete), handler.Delete)
	routes.GET("/roles/:id/permissions", authenticate, requirePermission(PermissionAuthorize), handler.Permissions)
	routes.PUT("/roles/:id/permissions", authenticate, requirePermission(PermissionAuthorize), handler.UpdatePermissions)
}
