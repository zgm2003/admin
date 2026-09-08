package role

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/permission/role", authenticate, requirePermission(PermissionList), handler.List)
	routes.POST("/permission/role", authenticate, requirePermission(PermissionCreate), handler.Create)
	routes.PUT("/permission/role/:id", authenticate, requirePermission(PermissionUpdate), handler.Update)
	routes.PATCH("/permission/role/:id/status", authenticate, requirePermission(PermissionStatus), handler.UpdateStatus)
	routes.PATCH("/permission/role/:id/default", authenticate, requirePermission(PermissionDefault), handler.SetDefault)
	routes.DELETE("/permission/role/:id", authenticate, requirePermission(PermissionDelete), handler.Delete)
	routes.GET("/permission/role/:id/permission", authenticate, requirePermission(PermissionAuthorize), handler.Permissions)
	routes.PUT("/permission/role/:id/permission", authenticate, requirePermission(PermissionAuthorize), handler.UpdatePermissions)
}
