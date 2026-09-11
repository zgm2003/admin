package recipientRule

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/recipient-rule", authenticate, requirePermission(PermissionList), handler.List)
	routes.POST("/recipient-rule", authenticate, requirePermission(PermissionCreate), handler.Create)
	routes.PUT("/recipient-rule/:id", authenticate, requirePermission(PermissionUpdate), handler.Update)
	routes.PATCH("/recipient-rule/:id/status", authenticate, requirePermission(PermissionStatus), handler.Status)
	routes.DELETE("/recipient-rule/:id", authenticate, requirePermission(PermissionDelete), handler.Delete)
}
