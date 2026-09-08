package auth

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, origin gin.HandlerFunc, authenticate gin.HandlerFunc) {
	authRoutes := routes.Group("/auth")
	authRoutes.GET("/login-config", origin, handler.LoginConfig)
	authRoutes.POST("/send-code", origin, handler.SendCode)
	authRoutes.POST("/login", origin, handler.Login)
	authRoutes.POST("/password/forgot", origin, handler.ForgotPassword)
	authRoutes.POST("/password/reset", origin, handler.ResetPassword)
	authRoutes.POST("/refresh", origin, handler.Refresh)
	authRoutes.POST("/logout", origin, authenticate, handler.Logout)
	authRoutes.GET("/me", authenticate, handler.Me)
}
