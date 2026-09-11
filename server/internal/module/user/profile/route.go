package profile

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	accountRoutes := routes.Group("/user")
	accountRoutes.GET("/profile", authenticate, requirePermission(PermissionDetail), handler.CurrentProfile)
	accountRoutes.PUT("/profile", authenticate, requirePermission(PermissionUpdate), handler.UpdateProfile)
	accountRoutes.POST("/password", authenticate, requirePermission(PermissionPasswordUpdate), handler.ChangePassword)
	accountRoutes.POST("/password/set", authenticate, requirePermission(PermissionPasswordUpdate), handler.SetPassword)
	accountRoutes.POST("/password/send-code", authenticate, requirePermission(PermissionPasswordUpdate), handler.SendPasswordCode)
	accountRoutes.PUT("/password/by-code", authenticate, requirePermission(PermissionPasswordUpdate), handler.ChangePasswordByCode)
}
