package file

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.RouterGroup, h *Handler, auth gin.HandlerFunc, req func(string) gin.HandlerFunc) {
	g := r.Group("/storage/file")
	g.GET("", auth, req(PermissionList), h.List)
	g.GET("/options", auth, req(PermissionList), h.Options)
	g.GET("/:id", auth, req(PermissionDetail), h.Get)
	g.POST("/:id/url", auth, req(PermissionDetail), h.URL)
	g.POST("/:id/complete", auth, req("storage:object:upload"), h.Complete)
	g.DELETE("/:id", auth, req(PermissionDelete), h.Delete)
}
