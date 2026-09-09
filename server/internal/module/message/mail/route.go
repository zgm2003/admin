package mail

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.RouterGroup, h *Handler, auth gin.HandlerFunc, req func(string) gin.HandlerFunc) {
	r.GET("/page-init", auth, req(PermissionList), h.PageInit)
	r.POST("/test", auth, req(PermissionTest), h.Test)
}
