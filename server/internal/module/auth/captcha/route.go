package captcha

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, origin gin.HandlerFunc) {
	routes.GET("/auth/captcha", origin, handler.Generate)
}
