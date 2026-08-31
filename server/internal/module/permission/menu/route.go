package menu

import "github.com/gin-gonic/gin"

func RegisterRoutes(
	routes *gin.RouterGroup,
	handler *Handler,
	authenticate gin.HandlerFunc,
	requirePermission func(string) gin.HandlerFunc,
) {
	routes.GET("/menus", authenticate, requirePermission(PermissionList), handler.List)
	routes.POST("/menus", authenticate, requirePermission(PermissionCreate), handler.Create)
	routes.PUT("/menus/:id", authenticate, requirePermission(PermissionUpdate), handler.Update)
	routes.PATCH("/menus/:id/status", authenticate, requirePermission(PermissionUpdate), handler.UpdateStatus)
	routes.DELETE("/menus/:id", authenticate, requirePermission(PermissionDelete), handler.Delete)
	routes.POST("/menus/access-cache/rebuild", authenticate, requirePermission(PermissionRebuildAccessCache), handler.RebuildAccessCache)
}
