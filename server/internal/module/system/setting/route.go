package setting

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/system/setting", authenticate, requirePermission(PermissionList), handler.List)
	routes.GET("/system/setting/:key", authenticate, requirePermission(PermissionDetail), handler.Detail)
	routes.POST("/system/setting", authenticate, requirePermission(PermissionCreate), handler.Create)
	routes.PUT("/system/setting/:key", authenticate, requirePermission(PermissionUpdate), handler.Update)
	routes.PATCH("/system/setting/:key/status", authenticate, requirePermission(PermissionStatus), handler.Status)
	routes.DELETE("/system/setting/:key", authenticate, requirePermission(PermissionDelete), handler.Delete)
}
