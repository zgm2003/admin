package session

import "github.com/gin-gonic/gin"

func RegisterSessionAdminRoutes(routes *gin.RouterGroup, handler *SessionAdminHandler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/user/session", authenticate, requirePermission(PermissionList), handler.List)
	routes.GET("/user/session/stats", authenticate, requirePermission(PermissionList), handler.Stats)
	routes.DELETE("/user/session/:id", authenticate, requirePermission(PermissionRevoke), handler.RevokeOne)
	routes.DELETE("/user/session", authenticate, requirePermission(PermissionRevoke), handler.RevokeMany)
}
