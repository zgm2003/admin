package order

import "github.com/gin-gonic/gin"

const (
	PermissionView   = "payment:order:view"
	PermissionList   = "payment:order:list"
	PermissionDetail = "payment:order:detail"
	PersonalView     = "payment:recharge:view"
	PersonalList     = "payment:recharge:list"
	PersonalDetail   = "payment:recharge:detail"
	PersonalCreate   = "payment:recharge:create"
	PersonalPay      = "payment:recharge:pay"
)

func RegisterAdminRoutes(routes *gin.RouterGroup, handler *Handler, authenticate gin.HandlerFunc, requirePermission func(string) gin.HandlerFunc) {
	group := routes.Group("/payment/order")
	group.GET("", authenticate, requirePermission(PermissionList), handler.List)
	group.GET("/:id", authenticate, requirePermission(PermissionDetail), handler.Get)

	personal := routes.Group("/payment/recharge")
	personal.GET("/options", authenticate, requirePermission(PersonalCreate), handler.Options)
	personal.GET("", authenticate, requirePermission(PersonalList), handler.PersonalList)
	personal.GET("/:id", authenticate, requirePermission(PersonalDetail), handler.PersonalGet)
	personal.POST("", authenticate, requirePermission(PersonalCreate), handler.Create)
	personal.POST("/:id/pay", authenticate, requirePermission(PersonalPay), handler.Pay)
}
