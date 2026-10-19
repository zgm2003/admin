package log

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/log", authenticate, requirePermission(PermissionList), handler.List)
	routes.GET("/log/:id", authenticate, requirePermission(PermissionDetail), handler.Detail)
}
