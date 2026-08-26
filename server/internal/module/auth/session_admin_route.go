package auth

import "github.com/gin-gonic/gin"

func RegisterSessionAdminRoutes(routes *gin.RouterGroup, handler *SessionAdminHandler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/sessions", authenticate, requirePermission(PermissionSessionList), handler.List)
	routes.GET("/sessions/stats", authenticate, requirePermission(PermissionSessionList), handler.Stats)
	routes.DELETE("/sessions/:id", authenticate, requirePermission(PermissionSessionRevoke), handler.RevokeOne)
	routes.DELETE("/sessions", authenticate, requirePermission(PermissionSessionRevoke), handler.RevokeMany)
}
