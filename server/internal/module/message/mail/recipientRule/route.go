package recipientrule

import "github.com/gin-gonic/gin"

func RegisterRoutes(group *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	group.GET("/recipient-rule", authenticate, requirePermission("message:mail:list"), handler.List)
	group.POST("/recipient-rule", authenticate, requirePermission(PermissionCreate), handler.Create)
	group.PUT("/recipient-rule/:id", authenticate, requirePermission(PermissionUpdate), handler.Update)
	group.PATCH("/recipient-rule/:id/status", authenticate, requirePermission(PermissionStatus), handler.SetStatus)
	group.DELETE("/recipient-rule/:id", authenticate, requirePermission(PermissionDelete), handler.Delete)
}
