package queuemonitor

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RegisterGrantRoute(routes *gin.RouterGroup, handler *Handler, origin, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.POST("/system/queuemonitor/grant", origin, authenticate, requirePermission(PermissionList), handler.Grant)
}

func RegisterUIRoutes(router *gin.Engine, handler http.Handler) {
	wrapped := gin.WrapH(handler)
	router.Any(UIPath, wrapped)
	router.Any(UIPath+"/*path", wrapped)
}
