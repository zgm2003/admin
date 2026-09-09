package log

import "github.com/gin-gonic/gin"

func RegisterRoutes(group *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	group.GET("/log", authenticate, requirePermission("message:mail:list"), handler.List)
	group.GET("/log/:id", authenticate, requirePermission(PermissionDetail), handler.Get)
}
