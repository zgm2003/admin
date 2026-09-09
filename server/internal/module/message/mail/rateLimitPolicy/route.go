package ratelimitpolicy

import "github.com/gin-gonic/gin"

func RegisterRoutes(group *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	group.GET("/rate-limit-policy", authenticate, requirePermission("message:mail:list"), handler.List)
	group.PUT("/rate-limit-policy/:key", authenticate, requirePermission(PermissionUpdate), handler.Update)
}
