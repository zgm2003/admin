package taskdemo

import "github.com/gin-gonic/gin"

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler) {
	routes.POST("/example-tasks", handler.Create)
}
