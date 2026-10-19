package config

import "github.com/gin-gonic/gin"

const (
	PermissionView   = "payment:config:view"
	PermissionList   = "payment:config:list"
	PermissionDetail = "payment:config:detail"
	PermissionCreate = "payment:config:create"
	PermissionUpdate = "payment:config:update"
	PermissionStatus = "payment:config:status"
	PermissionDelete = "payment:config:delete"
	PermissionTest   = "payment:config:test"
)

func RegisterAdminRoutes(r *gin.RouterGroup, h *Handler, auth gin.HandlerFunc, require func(string) gin.HandlerFunc) {
	g := r.Group("/payment/config")
	g.GET("/options", auth, h.Options)
	g.GET("", auth, require(PermissionList), h.List)
	g.GET("/:id", auth, require(PermissionDetail), h.Get)
	g.POST("", auth, require(PermissionCreate), h.Create)
	g.PUT("/:id", auth, require(PermissionUpdate), h.Update)
	g.POST("/:id/version", auth, require(PermissionUpdate), h.Version)
	g.PATCH("/:id/status", auth, require(PermissionStatus), h.Status)
	g.DELETE("/:id", auth, require(PermissionDelete), h.Delete)
	g.POST("/:id/test", auth, require(PermissionTest), h.Test)
}
