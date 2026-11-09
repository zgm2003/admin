package server

import "github.com/gin-gonic/gin"

// RegisterAdminRoutes exposes read-only host metrics to every authenticated admin-platform user.
func RegisterAdminRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc) {
	routes.GET("/system/server", authenticate, handler.Detail)
}
