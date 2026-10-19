package callbackevent

import "github.com/gin-gonic/gin"

const (
	PermissionList   = "payment:callbackEvent:list"
	PermissionDetail = "payment:callbackEvent:detail"
)

func RegisterAdminProviderRoutes(routes *gin.RouterGroup, handler *Handler) {
	routes.POST("/payment/callback/alipay/:configId/:version", handler.Provider)
}

func RegisterAdminRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	group := routes.Group("/payment/order/:id/callback")
	group.GET("", authenticate, requirePermission(PermissionList), handler.List)
	group.GET("/:eventId", authenticate, requirePermission(PermissionDetail), handler.Get)
}
