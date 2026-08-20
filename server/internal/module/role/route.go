package role

import (
	"admin/server/internal/module/menu"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/roles", authenticate, requirePermission(menu.PermissionRoleList), handler.List)
	routes.POST("/roles", authenticate, requirePermission(menu.PermissionRoleCreate), handler.Create)
	routes.PUT("/roles/:id", authenticate, requirePermission(menu.PermissionRoleUpdate), handler.Update)
	routes.PATCH("/roles/:id/status", authenticate, requirePermission(menu.PermissionRoleStatus), handler.UpdateStatus)
	routes.PATCH("/roles/:id/default", authenticate, requirePermission(menu.PermissionRoleDefault), handler.SetDefault)
	routes.DELETE("/roles/:id", authenticate, requirePermission(menu.PermissionRoleDelete), handler.Delete)
	routes.GET("/roles/:id/permissions", authenticate, requirePermission(menu.PermissionRoleAuthorize), handler.Permissions)
	routes.PUT("/roles/:id/permissions", authenticate, requirePermission(menu.PermissionRoleAuthorize), handler.UpdatePermissions)
}
