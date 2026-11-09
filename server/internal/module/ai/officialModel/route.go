package officialmodel

import "github.com/gin-gonic/gin"

const (
	PermissionView   = "ai:officialModel:view"
	PermissionList   = "ai:officialModel:list"
	PermissionUpdate = "ai:officialModel:update"
	PermissionSync   = "ai:officialModel:sync"
	PermissionReset  = "ai:officialModel:reset"
)

func RegisterAdminRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	group := routes.Group("/ai/official-model")
	group.GET("/options", authenticate, handler.Options)
	group.GET("", authenticate, requirePermission(PermissionList), handler.List)
	group.POST("/sync", authenticate, requirePermission(PermissionSync), handler.SyncCatalog)
	group.PUT("/:id/price", authenticate, requirePermission(PermissionUpdate), handler.UpdatePrice)
	group.POST("/:id/reset-price", authenticate, requirePermission(PermissionReset), handler.ResetPrice)
}
