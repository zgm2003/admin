package uploadrule

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.RouterGroup, h *Handler, auth gin.HandlerFunc, req func(string) gin.HandlerFunc) {
	g := r.Group("/storage/upload-rules")
	g.GET("", auth, req(PermissionList), h.List)
	g.GET("/page-init", auth, req(PermissionList), h.PageInit)
	g.POST("", auth, req(PermissionCreate), h.Create)
	g.GET("/:id", auth, req(PermissionList), h.Get)
	g.PUT("/:id", auth, req(PermissionUpdate), h.Update)
	g.PATCH("/:id/status", auth, req(PermissionStatus), h.Status)
	g.DELETE("/:id", auth, req(PermissionDelete), h.Delete)
}
