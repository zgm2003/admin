package template

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/template", authenticate, requirePermission(PermissionList), handler.List)
	routes.PUT("/template/:id", authenticate, requirePermission(PermissionUpdate), handler.Update)
	routes.PATCH("/template/:id/status", authenticate, requirePermission(PermissionStatus), handler.Status)
}
