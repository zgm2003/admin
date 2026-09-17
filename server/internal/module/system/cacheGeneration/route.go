package cachegeneration

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/system/cachegeneration", authenticate, requirePermission(PermissionList), handler.List)
}
