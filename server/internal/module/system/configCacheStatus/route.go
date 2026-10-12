package configcachestatus

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/system/config-cache-status/options", authenticate, handler.FormOptions)
	routes.GET("/system/config-cache-status", authenticate, requirePermission(PermissionList), handler.List)
}
