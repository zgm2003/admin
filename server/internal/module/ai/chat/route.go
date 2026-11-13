package chat

import "github.com/gin-gonic/gin"

const (
	PermissionList   = "ai:workbench:list"
	PermissionDetail = "ai:workbench:detail"
	PermissionCreate = "ai:workbench:create"
	PermissionUpdate = "ai:workbench:update"
	PermissionDelete = "ai:workbench:delete"
	PermissionSend   = "ai:workbench:send"
)

func RegisterAdminRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	group := routes.Group("/ai/chat")
	group.GET("/agents", authenticate, requirePermission(PermissionList), handler.Agents)
	group.GET("/balance", authenticate, requirePermission(PermissionList), handler.Balance)
	group.GET("/conversations", authenticate, requirePermission(PermissionList), handler.Conversations)
	group.POST("/conversations", authenticate, requirePermission(PermissionCreate), handler.CreateConversation)
	group.PATCH("/conversations/:id", authenticate, requirePermission(PermissionUpdate), handler.RenameConversation)
	group.DELETE("/conversations/:id", authenticate, requirePermission(PermissionDelete), handler.DeleteConversation)
	group.GET("/conversations/:id/messages", authenticate, requirePermission(PermissionDetail), handler.Messages)
	group.PUT("/messages/:id/feedback", authenticate, requirePermission(PermissionSend), handler.Feedback)
}
