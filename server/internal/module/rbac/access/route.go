package access

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc) {
	routes.GET("/access", authenticate, handler.Current)
}
