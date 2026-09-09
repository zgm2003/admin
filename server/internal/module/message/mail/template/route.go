package template

import "github.com/gin-gonic/gin"

func RegisterRoutes(group *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	group.GET("/template", authenticate, requirePermission("message:mail:list"), handler.List)
	group.PUT("/template/:id", authenticate, requirePermission(PermissionUpdate), handler.Update)
	group.PATCH("/template/:id/status", authenticate, requirePermission(PermissionStatus), handler.SetStatus)
}
