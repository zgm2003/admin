package user

import (
	"admin/server/internal/module/menu"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/users", authenticate, requirePermission(menu.PermissionUserList), handler.List)
	routes.GET("/users/role-options", authenticate, requirePermission(menu.PermissionUserList), handler.RoleOptions)
	routes.PUT("/users/:id", authenticate, requirePermission(menu.PermissionUserUpdate), handler.Update)
	routes.PATCH("/users/:id/status", authenticate, requirePermission(menu.PermissionUserStatus), handler.UpdateStatus)
	routes.DELETE("/users/:id", authenticate, requirePermission(menu.PermissionUserDelete), handler.Delete)
	routes.GET("/users/:id/roles", authenticate, requirePermission(menu.PermissionUserRoles), handler.Roles)
	routes.PUT("/users/:id/roles", authenticate, requirePermission(menu.PermissionUserRoles), handler.UpdateRoles)
}
