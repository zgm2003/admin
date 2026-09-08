package menu

import "github.com/gin-gonic/gin"

func RegisterRoutes(
	routes *gin.RouterGroup,
	handler *Handler,
	authenticate gin.HandlerFunc,
	requirePermission func(string) gin.HandlerFunc,
) {
	routes.GET("/permission/menu", authenticate, requirePermission(PermissionList), handler.List)
	routes.POST("/permission/menu", authenticate, requirePermission(PermissionCreate), handler.Create)
	routes.PUT("/permission/menu/:id", authenticate, requirePermission(PermissionUpdate), handler.Update)
	routes.PATCH("/permission/menu/:id/status", authenticate, requirePermission(PermissionUpdate), handler.UpdateStatus)
	routes.DELETE("/permission/menu/:id", authenticate, requirePermission(PermissionDelete), handler.Delete)
	routes.POST("/permission/menu/access-cache/rebuild", authenticate, requirePermission(PermissionRebuildAccessCache), handler.RebuildAccessCache)
}
