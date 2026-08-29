package account

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/users", authenticate, requirePermission(PermissionList), handler.List)
	routes.GET("/users/role-options", authenticate, requirePermission(PermissionList), handler.RoleOptions)
	routes.PUT("/users/:id", authenticate, requirePermission(PermissionUpdate), handler.Update)
	routes.PATCH("/users/:id/status", authenticate, requirePermission(PermissionStatus), handler.UpdateStatus)
	routes.DELETE("/users/:id", authenticate, requirePermission(PermissionDelete), handler.Delete)
	routes.GET("/users/:id/roles", authenticate, requirePermission(PermissionRoles), handler.Roles)
	routes.PUT("/users/:id/roles", authenticate, requirePermission(PermissionRoles), handler.UpdateRoles)
}
