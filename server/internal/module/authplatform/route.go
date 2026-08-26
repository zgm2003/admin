package authplatform

import "github.com/gin-gonic/gin"

func RegisterPublicRoutes(routes *gin.RouterGroup, handler *Handler) {
	routes.GET("/auth/policy", handler.Policy)
}

func RegisterManagementRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/auth-platforms", authenticate, requirePermission(PermissionList), handler.List)
	routes.GET("/auth-platforms/deployment", authenticate, requirePermission(PermissionList), handler.Deployment)
	routes.POST("/auth-platforms", authenticate, requirePermission(PermissionCreate), handler.Create)
	routes.PUT("/auth-platforms/:id", authenticate, requirePermission(PermissionUpdate), handler.Update)
	routes.PATCH("/auth-platforms/:id/status", authenticate, requirePermission(PermissionStatus), handler.UpdateStatus)
	routes.DELETE("/auth-platforms/:id", authenticate, requirePermission(PermissionDelete), handler.Delete)
}
