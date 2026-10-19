package phone

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	userRoutes := routes.Group("/user")
	userRoutes.GET("/phone/options", authenticate, handler.FormOptions)
	userRoutes.POST("/phone/send-code", authenticate, requirePermission(PermissionUpdate), handler.SendCode)
	userRoutes.PUT("/phone", authenticate, requirePermission(PermissionUpdate), handler.BindOrChange)
	userRoutes.GET("/account/:id/phone-change-log", authenticate, requirePermission(PermissionDetail), handler.ListChangeLogs)
}

// RegisterCanvasRoutes excludes management queries for other accounts.
func RegisterCanvasRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	userRoutes := routes.Group("/user")
	userRoutes.POST("/phone/send-code", authenticate, requirePermission(PermissionUpdate), handler.SendCode)
	userRoutes.PUT("/phone", authenticate, requirePermission(PermissionUpdate), handler.BindOrChange)
}
