package profile

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	accountRoutes := routes.Group("/account")
	accountRoutes.GET("/profile", authenticate, requirePermission(PermissionDetail), handler.CurrentProfile)
	accountRoutes.PUT("/profile", authenticate, requirePermission(PermissionUpdate), handler.UpdateProfile)
	accountRoutes.POST("/password", authenticate, requirePermission(PermissionPasswordUpdate), handler.ChangePassword)
}
