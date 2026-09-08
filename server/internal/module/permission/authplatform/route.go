package authplatform

import "github.com/gin-gonic/gin"

func RegisterPublicRoutes(routes *gin.RouterGroup, handler *Handler) {
	routes.GET("/auth/policy", handler.Policy)
}

func RegisterManagementRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/permission/authplatform", authenticate, requirePermission(PermissionList), handler.List)
	routes.GET("/permission/authplatform/deployment", authenticate, requirePermission(PermissionList), handler.Deployment)
	routes.POST("/permission/authplatform", authenticate, requirePermission(PermissionCreate), handler.Create)
	routes.PUT("/permission/authplatform/:id", authenticate, requirePermission(PermissionUpdate), handler.Update)
	routes.PATCH("/permission/authplatform/:id/status", authenticate, requirePermission(PermissionStatus), handler.UpdateStatus)
	routes.DELETE("/permission/authplatform/:id", authenticate, requirePermission(PermissionDelete), handler.Delete)
}
