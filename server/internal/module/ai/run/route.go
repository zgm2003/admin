package run

import "github.com/gin-gonic/gin"

const (
	PermissionSend      = "ai:workbench:send"
	PermissionRunList   = "ai:run:list"
	PermissionRunDetail = "ai:run:detail"
	PermissionRunAll    = "ai:run:viewAll"
)

func RegisterAdminRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	group := routes.Group("/ai/chat")
	group.POST("/conversations/:id/messages", authenticate, requirePermission(PermissionSend), handler.Send)
	group.POST("/runs/:id/cancel", authenticate, requirePermission(PermissionSend), handler.Cancel)
	group.GET("/runs", authenticate, requirePermission(PermissionRunList), handler.RunsOwn)
	group.GET("/runs/:id", authenticate, requirePermission(PermissionRunDetail), handler.RunDetailOwn)
	group.GET("/manage/runs", authenticate, requirePermission(PermissionRunAll), handler.RunsAll)
	group.GET("/manage/runs/:id", authenticate, requirePermission(PermissionRunAll), handler.RunDetailAll)
}
