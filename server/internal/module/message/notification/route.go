package notification

import "github.com/gin-gonic/gin"

const (
	PermissionList   = "message:notification:list"
	PermissionRead   = "message:notification:read"
	PermissionDelete = "message:notification:delete"
)

func BasePermissionCodes() []string {
	return []string{"message:notification:view", PermissionList, PermissionRead, PermissionDelete}
}

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	routes.GET("/message/notification", authenticate, requirePermission(PermissionList), handler.List)
	routes.GET("/message/notification/summary", authenticate, requirePermission(PermissionList), handler.Summary)
	routes.PATCH("/message/notification/read-all", authenticate, requirePermission(PermissionRead), handler.ReadAll)
	routes.PATCH("/message/notification/:id/read", authenticate, requirePermission(PermissionRead), handler.Read)
	routes.DELETE("/message/notification/:id", authenticate, requirePermission(PermissionDelete), handler.Delete)
}
