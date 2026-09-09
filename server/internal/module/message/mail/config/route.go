package config

import "github.com/gin-gonic/gin"

func RegisterRoutes(group *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	group.GET("/config", authenticate, requirePermission("message:mail:list"), handler.Get)
	group.PUT("/config", authenticate, requirePermission(PermissionUpdate), handler.Save)
	group.DELETE("/config", authenticate, requirePermission(PermissionDelete), handler.Delete)
}
