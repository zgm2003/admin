package email

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	userRoutes := routes.Group("/user")
	userRoutes.GET("/email/options", authenticate, handler.FormOptions)
	userRoutes.POST("/email/send-code", authenticate, requirePermission(PermissionUpdate), handler.SendCode)
	userRoutes.PUT("/email", authenticate, requirePermission(PermissionUpdate), handler.BindOrChange)
	userRoutes.GET("/account/:id/email-change-log", authenticate, requirePermission(PermissionDetail), handler.ListChangeLogs)
}

// RegisterCanvasRoutes exposes only authenticated account-owner operations.
func RegisterCanvasRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	userRoutes := routes.Group("/user")
	userRoutes.POST("/email/send-code", authenticate, requirePermission(PermissionUpdate), handler.SendCode)
	userRoutes.PUT("/email", authenticate, requirePermission(PermissionUpdate), handler.BindOrChange)
}
