package operationlog

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/operation-logs", authenticate, requirePermission("system:operation-log:list"), handler.List)
}
