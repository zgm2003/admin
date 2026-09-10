package dictionary

import "github.com/gin-gonic/gin"

func RegisterOptionRoute(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc) {
	routes.GET("/system/dictionary/options", authenticate, handler.Options)
}

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/system/dictionary", authenticate, requirePermission(PermissionList), handler.List)
	routes.GET("/system/dictionary/:id", authenticate, requirePermission(PermissionDetail), handler.Get)
	routes.POST("/system/dictionary", authenticate, requirePermission(PermissionCreate), handler.Create)
	routes.PUT("/system/dictionary/:id", authenticate, requirePermission(PermissionUpdate), handler.Update)
	routes.PATCH("/system/dictionary/:id/status", authenticate, requirePermission(PermissionStatus), handler.Status)
	routes.DELETE("/system/dictionary/:id", authenticate, requirePermission(PermissionDelete), handler.Delete)
	routes.POST("/system/dictionary/:id/item", authenticate, requirePermission(PermissionCreate), handler.CreateItem)
	routes.PUT("/system/dictionary/:id/item/:itemId", authenticate, requirePermission(PermissionUpdate), handler.UpdateItem)
	routes.PATCH("/system/dictionary/:id/item/:itemId/status", authenticate, requirePermission(PermissionStatus), handler.StatusItem)
	routes.DELETE("/system/dictionary/:id/item/:itemId", authenticate, requirePermission(PermissionDelete), handler.DeleteItem)
}
