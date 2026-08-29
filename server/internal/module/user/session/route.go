package session

import "github.com/gin-gonic/gin"

func RegisterSessionAdminRoutes(routes *gin.RouterGroup, handler *SessionAdminHandler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/sessions", authenticate, requirePermission(PermissionList), handler.List)
	routes.GET("/sessions/stats", authenticate, requirePermission(PermissionList), handler.Stats)
	routes.DELETE("/sessions/:id", authenticate, requirePermission(PermissionRevoke), handler.RevokeOne)
	routes.DELETE("/sessions", authenticate, requirePermission(PermissionRevoke), handler.RevokeMany)
}
