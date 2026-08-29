package loginlog

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	group := routes.Group("/users/login-logs")
	group.GET("/page-init", authenticate, requirePermission(PermissionList), handler.PageInit)
	routes.GET("/users/login-logs", authenticate, requirePermission(PermissionList), handler.List)
}
