package auth

import "github.com/gin-gonic/gin"

func RegisterSessionAdminRoutes(routes *gin.RouterGroup, handler *SessionAdminHandler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/sessions", authenticate, requirePermission("system:session:list"), handler.List)
	routes.GET("/sessions/stats", authenticate, requirePermission("system:session:list"), handler.Stats)
	routes.DELETE("/sessions/:id", authenticate, requirePermission("system:session:revoke"), handler.RevokeOne)
	routes.DELETE("/sessions", authenticate, requirePermission("system:session:revoke"), handler.RevokeMany)
}
