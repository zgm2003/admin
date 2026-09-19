package realtime

import "github.com/gin-gonic/gin"

func RegisterRoutes(shared *gin.RouterGroup, root *gin.Engine, handler *Handler, origin gin.HandlerFunc, authenticate gin.HandlerFunc) {
	shared.POST("/realtime/ticket", origin, authenticate, handler.Ticket)
	root.GET("/api/v1/realtime/ws", handler.WebSocket)
}
