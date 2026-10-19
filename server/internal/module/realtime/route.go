package realtime

import "github.com/gin-gonic/gin"

// transports has no client-header middleware: browser WebSockets use one-time tickets.
func RegisterAdminRoutes(routes *gin.RouterGroup, root *gin.Engine, handler *Handler, origin gin.HandlerFunc, authenticate gin.HandlerFunc) {
	routes.POST("/realtime/ticket", origin, authenticate, handler.Ticket)
	root.GET("/api/admin/v1/realtime/ws", handler.AdminWebSocket)
}
