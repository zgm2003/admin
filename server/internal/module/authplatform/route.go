package authplatform

import "github.com/gin-gonic/gin"

func RegisterPublicRoutes(routes *gin.RouterGroup, handler *Handler) {
	routes.GET("/auth/policy", handler.Policy)
}

func RegisterManagementRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/auth-platforms", authenticate, requirePermission("system:auth-platform:list"), handler.List)
	routes.GET("/auth-platforms/deployment", authenticate, requirePermission("system:auth-platform:list"), handler.Deployment)
	routes.POST("/auth-platforms", authenticate, requirePermission("system:auth-platform:create"), handler.Create)
	routes.PUT("/auth-platforms/:id", authenticate, requirePermission("system:auth-platform:update"), handler.Update)
	routes.PATCH("/auth-platforms/:id/status", authenticate, requirePermission("system:auth-platform:status"), handler.UpdateStatus)
	routes.DELETE("/auth-platforms/:id", authenticate, requirePermission("system:auth-platform:delete"), handler.Delete)
}
