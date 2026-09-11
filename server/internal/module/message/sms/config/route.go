package config

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/config", authenticate, requirePermission(PermissionList), handler.Load)
	routes.PUT("/config", authenticate, requirePermission(PermissionUpdate), handler.Update)
	routes.DELETE("/config", authenticate, requirePermission(PermissionDelete), handler.Delete)
}
