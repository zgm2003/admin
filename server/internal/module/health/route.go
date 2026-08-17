package health

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes gin.IRoutes, handler *Handler) {
	routes.GET("/health", handler.Health)
	routes.GET("/ready", handler.Ready)
}
