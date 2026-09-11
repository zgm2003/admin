package rateLimitPolicy

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/rate-limit-policy", authenticate, requirePermission(PermissionList), handler.List)
	routes.PUT("/rate-limit-policy/:platformId/:key", authenticate, requirePermission(PermissionUpdate), handler.Update)
}
