package permission

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc) {
	routes.GET("/access", authenticate, handler.Current)
}

// RegisterCanvasRoutes declares the Canvas API surface independently.
func RegisterCanvasRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc) {
	routes.GET("/access", authenticate, handler.Current)
}
