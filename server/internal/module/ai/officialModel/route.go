package officialmodel

import "github.com/gin-gonic/gin"

const (
	PermissionList   = "ai:officialModel:list"
	PermissionDetail = "ai:officialModel:detail"
	PermissionCreate = "ai:officialModel:create"
	PermissionUpdate = "ai:officialModel:update"
	PermissionStatus = "ai:officialModel:status"
)

func RegisterAdminRoutes(r *gin.RouterGroup, h *Handler, auth gin.HandlerFunc, permission func(string) gin.HandlerFunc) {
	r.GET("/ai/officialmodel/options", auth, h.Options)
	r.GET("/ai/officialmodel", auth, permission(PermissionList), h.List)
	r.GET("/ai/officialmodel/:id", auth, permission(PermissionDetail), h.Detail)
	r.GET("/ai/officialmodel/:id/price", auth, permission(PermissionDetail), h.Prices)
	r.POST("/ai/officialmodel", auth, permission(PermissionCreate), h.Create)
	r.PUT("/ai/officialmodel/:id", auth, permission(PermissionUpdate), h.Update)
	r.PATCH("/ai/officialmodel/:id/status", auth, permission(PermissionStatus), h.Status)
}
