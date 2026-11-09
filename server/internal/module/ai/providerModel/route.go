package providermodel

import "github.com/gin-gonic/gin"

const (
	PermissionList   = "ai:provider:model:list"
	PermissionSync   = "ai:provider:model:sync"
	PermissionUpdate = "ai:provider:model:update"
)

func RegisterAdminRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	group := routes.Group("/ai/provider/:id/models")
	group.GET("/options", authenticate, requirePermission(PermissionList), handler.Options)
	group.GET("", authenticate, requirePermission(PermissionList), handler.List)
	group.POST("/sync", authenticate, requirePermission(PermissionSync), handler.Sync)
	group.PUT("/:modelId", authenticate, requirePermission(PermissionUpdate), handler.Update)
}
