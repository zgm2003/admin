package account

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/user/account", authenticate, requirePermission(PermissionList), handler.List)
	routes.GET("/user/account/role-options", authenticate, requirePermission(PermissionList), handler.RoleOptions)
	routes.PUT("/user/account/:id", authenticate, requirePermission(PermissionUpdate), handler.Update)
	routes.PATCH("/user/account/:id/status", authenticate, requirePermission(PermissionStatus), handler.UpdateStatus)
	routes.DELETE("/user/account/:id", authenticate, requirePermission(PermissionDelete), handler.Delete)
	routes.GET("/user/account/:id/role", authenticate, requirePermission(PermissionRoles), handler.Roles)
	routes.PUT("/user/account/:id/role", authenticate, requirePermission(PermissionRoles), handler.UpdateRoles)
}
