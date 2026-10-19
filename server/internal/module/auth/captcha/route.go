package captcha

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(routes *gin.RouterGroup, handler *Handler, origin gin.HandlerFunc) {
	routes.GET("/auth/captcha", origin, handler.Generate)
}

// RegisterCanvasRoutes declares the Canvas API surface independently.
func RegisterCanvasRoutes(routes *gin.RouterGroup, handler *Handler, origin gin.HandlerFunc) {
	routes.GET("/auth/captcha", origin, handler.Generate)
}
