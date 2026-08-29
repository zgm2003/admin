package profile

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc) {
	accountRoutes := routes.Group("/account")
	accountRoutes.GET("/profile", authenticate, handler.CurrentProfile)
	accountRoutes.PUT("/profile", authenticate, handler.UpdateProfile)
	accountRoutes.POST("/password", authenticate, handler.ChangePassword)
}
