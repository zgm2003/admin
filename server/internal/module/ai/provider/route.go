package provider

import "github.com/gin-gonic/gin"

const (
	PermissionList        = "ai:provider:list"
	PermissionDetail      = "ai:provider:detail"
	PermissionCreate      = "ai:provider:create"
	PermissionUpdate      = "ai:provider:update"
	PermissionStatus      = "ai:provider:status"
	PermissionDelete      = "ai:provider:delete"
	PermissionTest        = "ai:provider:test"
	PermissionSync        = "ai:provider:sync"
	PermissionSyncPrice   = "ai:provider:syncPrice"
	PermissionUpdatePrice = "ai:provider:updatePrice"
)

func RegisterAdminRoutes(r *gin.RouterGroup, h *Handler, auth gin.HandlerFunc, permission func(string) gin.HandlerFunc) {
	r.GET("/ai/provider/options", auth, h.Options)
	r.GET("/ai/provider/candidate/create", auth, permission(PermissionCreate), h.Candidates)
	r.GET("/ai/provider/:id/candidate/update", auth, permission(PermissionUpdate), h.Candidates)
	r.GET("/ai/provider/:id/candidate/sync", auth, permission(PermissionSync), h.Candidates)
	r.GET("/ai/provider", auth, permission(PermissionList), h.List)
	r.GET("/ai/provider/:id", auth, permission(PermissionDetail), h.Detail)
	r.POST("/ai/provider", auth, permission(PermissionCreate), h.Create)
	r.PUT("/ai/provider/:id", auth, permission(PermissionUpdate), h.Update)
	r.PATCH("/ai/provider/:id/status", auth, permission(PermissionStatus), h.Status)
	r.DELETE("/ai/provider/:id", auth, permission(PermissionDelete), h.Delete)
	r.POST("/ai/provider/:id/test", auth, permission(PermissionTest), h.Test)
	r.POST("/ai/provider/:id/model/sync-preview", auth, permission(PermissionSync), h.SyncPreview)
	r.POST("/ai/provider/:id/model/sync", auth, permission(PermissionSync), h.SyncConfirm)
	r.GET("/ai/provider/:id/model", auth, permission(PermissionDetail), h.ModelList)
	r.PUT("/ai/provider/:id/model", auth, permission(PermissionUpdate), h.ModelSave)
	r.PATCH("/ai/provider/:id/model/:modelId/status", auth, permission(PermissionStatus), h.ModelStatus)
	r.GET("/ai/provider/:id/model/:modelId/price", auth, permission(PermissionDetail), h.PriceList)
	r.POST("/ai/provider/:id/model/:modelId/price/sync", auth, permission(PermissionSyncPrice), h.PriceSync)
	r.PUT("/ai/provider/:id/model/:modelId/price", auth, permission(PermissionUpdatePrice), h.PriceUpdate)
	r.POST("/ai/provider/:id/model/:modelId/price/:versionId/publish", auth, permission(PermissionUpdatePrice), h.PricePublish)
}
