package notificationtask

import "github.com/gin-gonic/gin"

const (
	PermissionList   = "message:notificationTask:list"
	PermissionDetail = "message:notificationTask:detail"
	PermissionCreate = "message:notificationTask:create"
	PermissionUpdate = "message:notificationTask:update"
	PermissionDelete = "message:notificationTask:delete"
	PermissionSubmit = "message:notificationTask:submit"
	PermissionCancel = "message:notificationTask:cancel"
	PermissionCopy   = "message:notificationTask:copy"
)

func RegisterRoutes(r *gin.RouterGroup, h *Handler, auth gin.HandlerFunc, require func(string) gin.HandlerFunc) {
	r.GET("/message/notificationtask", auth, require(PermissionList), h.List)
	r.GET("/message/notificationtask/platform-option", auth, require(PermissionDetail), h.PlatformOption)
	r.GET("/message/notificationtask/user-option", auth, require(PermissionDetail), h.UserOption)
	r.GET("/message/notificationtask/role-option", auth, require(PermissionDetail), h.RoleOption)
	r.GET("/message/notificationtask/:id", auth, require(PermissionDetail), h.Detail)
	r.POST("/message/notificationtask", auth, require(PermissionCreate), h.Create)
	r.PUT("/message/notificationtask/:id", auth, require(PermissionUpdate), h.Update)
	r.DELETE("/message/notificationtask/:id", auth, require(PermissionDelete), h.Delete)
	r.POST("/message/notificationtask/:id/submit", auth, require(PermissionSubmit), h.Submit)
	r.POST("/message/notificationtask/:id/cancel", auth, require(PermissionCancel), h.Cancel)
	r.POST("/message/notificationtask/:id/copy", auth, require(PermissionCopy), h.Copy)
}
