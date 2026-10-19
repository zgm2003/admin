package uploadrule

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(r *gin.RouterGroup, h *Handler, auth gin.HandlerFunc, req func(string) gin.HandlerFunc) {
	g := r.Group("/storage/uploadrule")
	g.GET("", auth, req(PermissionList), h.List)
	g.GET("/page-init", auth, req(PermissionList), h.PageInit)
	g.POST("", auth, req(PermissionCreate), h.Create)
	g.GET("/:id", auth, req(PermissionDetail), h.Get)
	g.PUT("/:id", auth, req(PermissionUpdate), requireStatusOnUpdate(req(PermissionStatus)), h.Update)
	g.PATCH("/:id/status", auth, req(PermissionStatus), h.Status)
	g.DELETE("/:id", auth, req(PermissionDelete), h.Delete)

	r.POST("/storage/upload-credential", auth, req("storage:object:upload"), h.Credentials)
	r.POST("/storage/object-url", auth, h.ObjectURL)
}

func RegisterCanvasRoutes(r *gin.RouterGroup, h *Handler, auth gin.HandlerFunc, req func(string) gin.HandlerFunc) {
	r.POST("/storage/upload-credential", auth, req("storage:object:upload"), h.Credentials)
	r.POST("/storage/object-url", auth, h.ObjectURL)
}
