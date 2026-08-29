package cosconfig

import "github.com/gin-gonic/gin"

const (
	PermissionList   = "storage:object:list"
	PermissionCreate = "storage:cos-config:create"
	PermissionUpdate = "storage:cos-config:update"
	PermissionStatus = "storage:cos-config:status"
	PermissionTest   = "storage:cos-config:test"
	PermissionDelete = "storage:cos-config:delete"
)

func RegisterRoutes(r *gin.RouterGroup, h *Handler, auth gin.HandlerFunc, req func(string) gin.HandlerFunc) {
	g := r.Group("/storage/cos-configs")
	g.GET("", auth, req(PermissionList), h.List)
	g.POST("", auth, req(PermissionCreate), h.Create)
	g.GET("/:id", auth, req(PermissionList), h.Get)
	g.PUT("/:id", auth, req(PermissionUpdate), h.Update)
	g.PATCH("/:id/status", auth, req(PermissionStatus), h.Status)
	g.POST("/:id/test", auth, req(PermissionTest), h.Test)
	g.DELETE("/:id", auth, req(PermissionDelete), h.Delete)
}
