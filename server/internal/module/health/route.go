package health

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes gin.IRoutes, handler *Handler) {
	routes.GET("/api/health", handler.Health)
	routes.GET("/api/ready", handler.Ready)
}
